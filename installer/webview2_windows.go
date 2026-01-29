//go:build windows

package installer

import (
	"github.com/crafted-tech/webframe"
)

// WebView2Status contains information about the WebView2 runtime installation.
type WebView2Status = webframe.WebView2Status

// MinimumWebView2Version is the minimum WebView2 version recommended.
const MinimumWebView2Version = webframe.MinimumWebView2Version

// WebView2InstallURL is the URL to download the WebView2 Evergreen Runtime installer.
const WebView2InstallURL = webframe.WebView2InstallURL

// CheckWebView2 checks if the WebView2 runtime is installed and returns its status.
// Safe to call before any UI initialization.
func CheckWebView2() WebView2Status {
	return webframe.CheckWebView2Runtime("")
}

// NativeConfirm shows a native OK/Cancel confirmation dialog.
// Returns true if the user clicked OK.
// Safe to call before any UI initialization.
func NativeConfirm(title, message string) bool {
	return webframe.ShowConfirmDialog(title, message)
}

// NativeError shows a native error dialog.
// Safe to call before any UI initialization.
func NativeError(title, message string) {
	webframe.ShowErrorDialog(title, message)
}

// NativeInfo shows a native information dialog.
// Safe to call before any UI initialization.
func NativeInfo(title, message string) {
	webframe.ShowInfoDialog(title, message)
}

// NativeWarning shows a native warning dialog.
// Safe to call before any UI initialization.
func NativeWarning(title, message string) {
	webframe.ShowWarningDialog(title, message)
}
