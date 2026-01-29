//go:build windows

package installer

import (
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

//go:embed assets/MicrosoftEdgeWebview2Setup.exe
var embeddedBootstrapper []byte

var (
	// ErrWebView2Cancelled is returned when the user cancels WebView2 installation.
	ErrWebView2Cancelled = errors.New("WebView2 installation cancelled by user")

	// ErrWebView2InstallFailed is returned when WebView2 installation fails.
	ErrWebView2InstallFailed = errors.New("WebView2 installation failed")

	// ErrNoBootstrapper is returned when no embedded bootstrapper is available.
	ErrNoBootstrapper = errors.New("no embedded WebView2 bootstrapper available")
)

// HasEmbeddedBootstrapper returns true if the WebView2 bootstrapper is embedded.
func HasEmbeddedBootstrapper() bool {
	return len(embeddedBootstrapper) > 0
}

// ExtractBootstrapper extracts the embedded bootstrapper to a temp file.
// Returns the path to the extracted file. Caller is responsible for cleanup.
func ExtractBootstrapper() (string, error) {
	if !HasEmbeddedBootstrapper() {
		return "", ErrNoBootstrapper
	}

	installerPath := filepath.Join(os.TempDir(), "MicrosoftEdgeWebview2Setup.exe")
	if err := os.WriteFile(installerPath, embeddedBootstrapper, 0o755); err != nil {
		return "", err
	}
	return installerPath, nil
}

// RunBootstrapper executes the WebView2 bootstrapper.
// The bootstrapper has its own UI and will download/install the runtime.
// Returns nil if the installer ran successfully (exit code 0).
func RunBootstrapper(installerPath string) error {
	cmd := exec.Command(installerPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		// Check if it's an exit error with non-zero status
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() != 0 {
					return ErrWebView2InstallFailed
				}
			}
		}
		return err
	}
	return nil
}

// EnsureWebView2 checks for WebView2 and installs if missing.
// Uses native dialogs (safe to call before webflow UI initialization).
// Returns nil if WebView2 is available after this call.
//
// Flow:
//  1. Check if WebView2 is already installed and meets minimum version
//  2. If not, show native confirmation dialog
//  3. If user confirms, extract and run the embedded bootstrapper
//  4. Verify installation succeeded
//
// Returns ErrWebView2Cancelled if user cancels, or ErrWebView2InstallFailed
// if installation doesn't complete successfully.
func EnsureWebView2() error {
	status := CheckWebView2()
	if status.MeetsMinimum {
		return nil // Already installed and meets minimum version
	}

	// Check if we have an embedded bootstrapper
	if !HasEmbeddedBootstrapper() {
		// No bootstrapper embedded - show error with download URL
		NativeError(
			"WebView2 Required",
			"This application requires Microsoft Edge WebView2 Runtime.\n\n"+
				"Please download and install it from:\n"+
				WebView2InstallURL+"\n\n"+
				"Then restart the installer.",
		)
		return ErrNoBootstrapper
	}

	// Determine message based on status
	var message string
	if status.Installed {
		message = "WebView2 runtime needs to be updated.\n\nClick OK to install the update."
	} else {
		message = "This application requires Microsoft Edge WebView2 Runtime.\n\nClick OK to install it now."
	}

	// Prompt user for confirmation
	if !NativeConfirm("WebView2 Required", message) {
		return ErrWebView2Cancelled
	}

	// Extract and run bootstrapper
	installerPath, err := ExtractBootstrapper()
	if err != nil {
		NativeError("Installation Error", "Failed to extract installer: "+err.Error())
		return err
	}
	defer os.Remove(installerPath)

	if err := RunBootstrapper(installerPath); err != nil {
		NativeError("Installation Error", "Failed to run installer: "+err.Error())
		return err
	}

	// Verify installation succeeded
	status = CheckWebView2()
	if !status.MeetsMinimum {
		NativeError("Installation Failed", "WebView2 installation did not complete successfully.\n\nPlease try running the installer again.")
		return ErrWebView2InstallFailed
	}

	return nil
}
