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
// The function tries two strategies in order:
//  1. Shell token approach — borrows the token from the running Explorer
//     shell process. Works when the caller is a UAC-elevated admin process
//     and the desktop shell is running.
//  2. WTS approach — uses WTSQueryUserToken + CreateProcessAsUser. Works
//     when the caller is running as SYSTEM (e.g., a Windows service) and
//     has SE_TCB_NAME privilege.
//
// Returns the PID of the launched process.
func LaunchAsSessionUser(exePath string) (uint32, error) {
	// Try the shell-token approach first (works from elevated admin).
	if pid, err := LaunchDeElevated(exePath); err == nil {
		return pid, nil
	}

	// Fall back to WTS approach (works from SYSTEM).

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
