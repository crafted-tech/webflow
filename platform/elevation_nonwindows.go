//go:build !windows

package platform

import (
	"fmt"
	"strings"
)

// LaunchElevated is not supported on non-Windows platforms.
// On Windows it launches the current executable via ShellExecute "runas".
func LaunchElevated(args string) error {
	return fmt.Errorf("LaunchElevated not supported on this platform")
}

// OpenProcessByPID is not supported on non-Windows platforms.
func OpenProcessByPID(pid uint32) (uintptr, error) {
	return 0, fmt.Errorf("OpenProcessByPID not supported on this platform")
}

// GetProcessExitCode is not supported on non-Windows platforms.
func GetProcessExitCode(handle uintptr) (uint32, error) {
	return 0, fmt.Errorf("GetProcessExitCode not supported on this platform")
}

// QuoteArg wraps a path in double quotes if it contains spaces.
func QuoteArg(s string) string {
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}
