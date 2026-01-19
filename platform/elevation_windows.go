//go:build windows

package platform

import (
	"errors"
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
