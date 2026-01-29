//go:build !windows

package installer

// WebView2Status contains information about the WebView2 runtime installation.
// On non-Windows platforms, this is a stub.
type WebView2Status struct {
	Installed    bool
	Version      string
	MeetsMinimum bool
}

// MinimumWebView2Version is the minimum WebView2 version recommended.
// On non-Windows platforms, this is provided for API compatibility only.
const MinimumWebView2Version = "94.0.992.31"

// WebView2InstallURL is the URL to download the WebView2 Evergreen Runtime installer.
// On non-Windows platforms, this is provided for API compatibility only.
const WebView2InstallURL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

// CheckWebView2 checks if the WebView2 runtime is installed and returns its status.
// On non-Windows platforms, this always returns installed since WebView2 is Windows-only.
func CheckWebView2() WebView2Status {
	return WebView2Status{
		Installed:    true,
		Version:      "",
		MeetsMinimum: true,
	}
}

// NativeConfirm shows a native OK/Cancel confirmation dialog.
// On non-Windows platforms, this is a no-op that returns true.
func NativeConfirm(title, message string) bool {
	return true
}

// NativeError shows a native error dialog.
// On non-Windows platforms, this is a no-op.
func NativeError(title, message string) {}

// NativeInfo shows a native information dialog.
// On non-Windows platforms, this is a no-op.
func NativeInfo(title, message string) {}

// NativeWarning shows a native warning dialog.
// On non-Windows platforms, this is a no-op.
func NativeWarning(title, message string) {}

// HasEmbeddedBootstrapper returns true if the WebView2 bootstrapper is embedded.
// On non-Windows platforms, this always returns false.
func HasEmbeddedBootstrapper() bool {
	return false
}

// ExtractBootstrapper extracts the embedded bootstrapper to a temp file.
// On non-Windows platforms, this is a no-op that returns empty string.
func ExtractBootstrapper() (string, error) {
	return "", nil
}

// RunBootstrapper executes the WebView2 bootstrapper.
// On non-Windows platforms, this is a no-op.
func RunBootstrapper(installerPath string) error {
	return nil
}

// EnsureWebView2 checks for WebView2 and installs if missing.
// On non-Windows platforms, this is a no-op that returns nil.
func EnsureWebView2() error {
	return nil
}
