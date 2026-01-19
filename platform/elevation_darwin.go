//go:build darwin

package platform

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrElevationDeclined indicates the user rejected the elevation prompt.
var ErrElevationDeclined = errors.New("administrator elevation declined")

// IsElevated checks if the current process is running with root privileges.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// EnsureElevated checks if the current process has root privileges.
// If not, it relaunches the executable with elevation (using osascript) and exits.
// Returns ErrElevationDeclined if the user rejects the prompt.
func EnsureElevated() error {
	if IsElevated() {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// Build the command to run with administrator privileges
	// Using osascript to show a native macOS authentication dialog
	args := strings.Join(os.Args[1:], "\" \"")
	var shellCmd string
	if args != "" {
		shellCmd = "\"" + exePath + "\" \"" + args + "\""
	} else {
		shellCmd = "\"" + exePath + "\""
	}

	// AppleScript to run command with administrator privileges
	script := `do shell script ` + `"` + shellCmd + `"` + ` with administrator privileges`

	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		// Check if user cancelled
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return ErrElevationDeclined
			}
		}
		return err
	}

	// Relaunch succeeded, exit current process
	os.Exit(0)
	return nil
}

// RunElevated runs a command with administrator privileges.
// Shows a native macOS authentication dialog.
// Returns ErrElevationDeclined if the user rejects the prompt.
func RunElevated(command string, args ...string) error {
	// Build shell command
	shellCmd := command
	for _, arg := range args {
		shellCmd += " \"" + arg + "\""
	}

	// AppleScript to run command with administrator privileges
	script := `do shell script "` + shellCmd + `" with administrator privileges`

	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return ErrElevationDeclined
			}
		}
		return err
	}

	return nil
}
