//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrElevationDeclined indicates the user rejected the UAC prompt.
var ErrElevationDeclined = errors.New("administrator elevation declined")

// IsElevated checks if the current process is running with administrator privileges.
func IsElevated() bool {
	elevated, err := isElevated()
	if err != nil {
		return false
	}
	return elevated
}

// EnsureElevated checks if the current process has administrator privileges.
// If not, it relaunches the executable with elevation (UAC prompt) and exits.
// Returns ErrElevationDeclined if the user rejects the UAC prompt.
func EnsureElevated() error {
	elevated, err := isElevated()
	if err != nil {
		// If we cannot determine elevation status, assume not elevated
		// and attempt ShellExecute with runas anyway.
		elevated = false
	}
	if elevated {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// Relaunch with runas verb to trigger UAC
	args := strings.Join(os.Args[1:], " ")
	err = windows.ShellExecute(0,
		windows.StringToUTF16Ptr("runas"),
		windows.StringToUTF16Ptr(exePath),
		windows.StringToUTF16Ptr(args),
		nil,
		windows.SW_SHOWNORMAL,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return ErrElevationDeclined
		}
		return err
	}

	// Relaunch requested, exit current process.
	os.Exit(0)
	return nil
}

// isElevated checks if the current process is running with admin privileges.
func isElevated() (bool, error) {
	token := windows.Token(0)
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()

	// Use TOKEN_ELEVATION to detect whether the process is elevated.
	type tokenElevation struct {
		TokenIsElevated uint32
	}

	var elevation tokenElevation
	var outLen uint32
	if err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&outLen,
	); err != nil {
		return false, err
	}

	return elevation.TokenIsElevated != 0, nil
}

// LaunchElevated launches the current executable with the given args via
// ShellExecute "runas". Fire-and-forget — caller reads PID from IPC file.
// Returns ErrElevationDeclined if the user rejects the UAC prompt.
func LaunchElevated(args string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	err = windows.ShellExecute(0,
		windows.StringToUTF16Ptr("runas"),
		windows.StringToUTF16Ptr(exePath),
		windows.StringToUTF16Ptr(args),
		nil,
		windows.SW_HIDE, // Elevated child has no UI window
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return ErrElevationDeclined
		}
		return fmt.Errorf("ShellExecute runas: %w", err)
	}

	return nil
}

// OpenProcessByPID opens a process handle for the given PID with enough
// access rights to wait on it and query its exit code.
func OpenProcessByPID(pid uint32) (windows.Handle, error) {
	const desiredAccess = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE
	handle, err := windows.OpenProcess(desiredAccess, false, pid)
	if err != nil {
		return 0, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	return handle, nil
}

// GetProcessExitCode returns the exit code of a process. The process must have exited.
func GetProcessExitCode(handle windows.Handle) (uint32, error) {
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return 0, err
	}
	return exitCode, nil
}

// QuoteArg wraps a path in double quotes if it contains spaces.
func QuoteArg(s string) string {
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}
