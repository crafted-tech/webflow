//go:build windows

package platform

import (
	"fmt"
	"path/filepath"
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
		windows.SecurityIdentification,
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
	err = windows.CreateProcessAsUser(
		primaryToken,
		exePathPtr,
		nil,   // command line
		nil,   // process security attributes
		nil,   // thread security attributes
		false, // inherit handles
		windows.CREATE_UNICODE_ENVIRONMENT,
		envBlock,
		workDirPtr, // current directory
		&si,
		&pi,
	)
	if err != nil {
		return 0, fmt.Errorf("create process as user: %w", err)
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return pi.ProcessId, nil
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
