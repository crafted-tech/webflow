//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// LaunchAsSessionUser launches an executable in the active console user's
// desktop session at the user's normal (non-elevated) privilege level.
//
// The function picks the right strategy based on the caller's identity:
//   - SYSTEM (e.g., a Windows service or its child process): uses WTS
//     approach (WTSQueryUserToken + CreateProcessAsUser) which correctly
//     targets the desktop user's session. LaunchDeElevated is skipped
//     because its schtasks approach would create the task as SYSTEM
//     (no /ru flag → inherits caller identity), launching the app in
//     session 0 instead of the user's desktop.
//   - Elevated admin (e.g., UAC-elevated installer): uses LaunchDeElevated
//     which borrows the Explorer shell token to de-elevate.
//   - Non-elevated: falls through to WTS as a last resort.
//
// Returns the PID of the launched process.
func LaunchAsSessionUser(exePath string) (uint32, error) {
	// When running as SYSTEM, skip LaunchDeElevated entirely — its strategies
	// (scheduled task, COM to Explorer, shell token) all either run as SYSTEM
	// or fail. Go straight to WTS which is designed for SYSTEM → user session.
	if !isRunningAsSystem() {
		if pid, err := LaunchDeElevated(exePath); err == nil {
			return pid, nil
		}
	}

	// WTS approach (works from SYSTEM — requires SE_TCB_NAME privilege).

	// Get the active console session (the one with the physical keyboard/monitor).
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return 0, fmt.Errorf("no active console session")
	}

	// Obtain the session user's token.
	var userToken windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return 0, fmt.Errorf("query user token for session %d: %w", sessionID, err)
	}
	defer userToken.Close()

	// Duplicate as a primary token (required by CreateProcessAsUser).
	var primaryToken windows.Token
	err := windows.DuplicateTokenEx(
		userToken,
		windows.MAXIMUM_ALLOWED,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&primaryToken,
	)
	if err != nil {
		return 0, fmt.Errorf("duplicate token: %w", err)
	}
	defer primaryToken.Close()

	// Build the user's environment block.
	var envBlock *uint16
	if err := windows.CreateEnvironmentBlock(&envBlock, primaryToken, false); err != nil {
		return 0, fmt.Errorf("create environment block: %w", err)
	}
	defer windows.DestroyEnvironmentBlock(envBlock)

	// Prepare startup info targeting the interactive desktop.
	desktop, _ := windows.UTF16PtrFromString(`Winsta0\Default`)
	si := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktop,
	}

	// Launch the process.
	exePathPtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return 0, fmt.Errorf("invalid exe path: %w", err)
	}

	workDirPtr, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return 0, fmt.Errorf("invalid work dir: %w", err)
	}

	var pi windows.ProcessInformation

	// CREATE_BREAKAWAY_FROM_JOB detaches the child from the caller's job
	// object. Without this, the app inherits the job from the SFX/installer
	// chain and gets killed when the SFX exits (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE).
	// The SCM's per-service job sets JOB_OBJECT_LIMIT_BREAKAWAY_OK, so this
	// works from the service process chain. If the job doesn't allow breakaway,
	// fall back to creating without the flag (better than not launching at all).
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_BREAKAWAY_FROM_JOB)
	err = windows.CreateProcessAsUser(
		primaryToken,
		exePathPtr,
		nil,   // command line
		nil,   // process security attributes
		nil,   // thread security attributes
		false, // inherit handles
		flags,
		envBlock,
		workDirPtr, // current directory
		&si,
		&pi,
	)
	if err != nil {
		// Retry without CREATE_BREAKAWAY_FROM_JOB — the job may not allow
		// breakaway, but launching inside the job is better than not launching.
		flags = windows.CREATE_UNICODE_ENVIRONMENT
		err = windows.CreateProcessAsUser(
			primaryToken,
			exePathPtr,
			nil,
			nil,
			nil,
			false,
			flags,
			envBlock,
			workDirPtr,
			&si,
			&pi,
		)
	}
	if err != nil {
		// Retry via CreateProcessWithTokenW. This path often works in service-child
		// contexts where CreateProcessAsUser lacks required privileges.
		if pid, tokenErr := createProcessWithToken(primaryToken, exePath, envBlock, workDirPtr, desktop); tokenErr == nil {
			return pid, nil
		}

		// CreateProcessAsUser failed. Fall back to schtasks with the session
		// user's identity — the same proven approach used by LaunchDeElevated
		// (launchViaScheduledTask) but with /ru <user> /it so the task runs
		// in the user's interactive session rather than as the caller (SYSTEM).
		if schtasksErr := launchViaScheduledTaskForUser(exePath, primaryToken); schtasksErr != nil {
			return 0, fmt.Errorf("CreateProcessAsUser: %w; schtasks fallback: %w", err, schtasksErr)
		}
		// Task Scheduler can report success while the process never materializes.
		// Confirm the process appears before reporting success to the caller.
		if !waitForProcessByName(filepath.Base(exePath), 8*time.Second) {
			return 0, fmt.Errorf("schtasks fallback did not start process %q", filepath.Base(exePath))
		}
		return 0, nil
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return pi.ProcessId, nil
}

func createProcessWithToken(primaryToken windows.Token, exePath string, envBlock *uint16, workDirPtr *uint16, desktop *uint16) (uint32, error) {
	exePathPtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return 0, fmt.Errorf("invalid exe path: %w", err)
	}

	si := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktop,
	}
	var pi windows.ProcessInformation

	flags := uintptr(windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB)
	r1, _, e1 := procCreateProcessWithTokenW.Call(
		uintptr(primaryToken),
		logonWithProfile,
		uintptr(unsafe.Pointer(exePathPtr)),
		0,
		flags,
		uintptr(unsafe.Pointer(envBlock)),
		uintptr(unsafe.Pointer(workDirPtr)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("CreateProcessWithTokenW: %w", e1)
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)
	return pi.ProcessId, nil
}

func waitForProcessByName(exeName string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsProcessRunning(exeName) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return IsProcessRunning(exeName)
}

// launchViaScheduledTaskForUser creates and immediately runs a one-shot
// scheduled task targeting a specific user's interactive session. This mirrors
// launchViaScheduledTask in deelevate_windows.go but specifies the user via
// /ru and /it so the task runs in their desktop session (not as the caller).
// The /it flag uses the user's interactive token — no password is needed.
func launchViaScheduledTaskForUser(exePath string, userToken windows.Token) error {
	// Look up the account name from the token so we can pass it to schtasks /ru.
	tokenUser, err := userToken.GetTokenUser()
	if err != nil {
		return fmt.Errorf("get token user: %w", err)
	}
	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return fmt.Errorf("lookup account: %w", err)
	}
	ruArg := account
	if domain != "" {
		ruArg = domain + `\` + account
	}

	taskName := fmt.Sprintf("UnisonLaunch_%d", os.Getpid())
	schtasks := filepath.Join(os.Getenv("WINDIR"), "System32", "schtasks.exe")

	if err := runHidden(schtasks, "/create",
		"/tn", taskName,
		"/tr", `"`+exePath+`"`,
		"/sc", "once",
		"/st", "00:00",
		"/ru", ruArg,
		"/it",
		"/f",
	); err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	// Battery settings are an optimization — don't fail the launch if
	// PowerShell is unavailable (execution policy, locked-down enterprise).
	_ = setTaskBatteryFriendly(taskName)

	AllowSetForegroundForAnyProcess()

	if err := runHidden(schtasks, "/run", "/tn", taskName); err != nil {
		runHidden(schtasks, "/delete", "/tn", taskName, "/f")
		return fmt.Errorf("run task: %w", err)
	}

	// Brief wait for Task Scheduler to start the process, then delete.
	time.Sleep(1 * time.Second)
	runHidden(schtasks, "/delete", "/tn", taskName, "/f")
	return nil
}

// isRunningAsSystem reports whether the current process is running as
// the NT AUTHORITY\SYSTEM account (SID S-1-5-18).
func isRunningAsSystem() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return false
	}

	// Well-known SID: S-1-5-18 (Local System)
	systemSID, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return false
	}

	return windows.EqualSid(user.User.Sid, systemSID)
}
