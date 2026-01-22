package webflow

import "encoding/json"

// icons contains SVG icons loaded from embedded JSON.
// Keys: copy, check, download, file, folder, info, warning, error, success
var icons map[string]string

func init() {
	if err := json.Unmarshal(iconsJSON, &icons); err != nil {
		panic("webflow: failed to parse embedded icons: " + err.Error())
	}
}

// Backward-compatible icon exports.
// These are Lucide icons from https://lucide.dev/ (ISC License)
var (
	// IconCopy is a copy/clipboard icon
	IconCopy = ""
	// IconCheck is a checkmark icon (used for copy confirmation)
	IconCheck = ""
	// IconDownload is a download/save icon
	IconDownload = ""
	// IconFile is a generic file icon
	IconFile = ""
	// IconFolder is a folder icon
	IconFolder = ""
	// IconInfo is an info circle icon
	IconInfo = ""
	// IconWarning is a warning triangle icon
	IconWarning = ""
	// IconError is an error/X circle icon
	IconError = ""
	// IconSuccess is a success/check circle icon
	IconSuccess = ""
)

func init() {
	// Initialize backward-compatible exports from loaded icons map
	IconCopy = icons["copy"]
	IconCheck = icons["check"]
	IconDownload = icons["download"]
	IconFile = icons["file"]
	IconFolder = icons["folder"]
	IconInfo = icons["info"]
	IconWarning = icons["warning"]
	IconError = icons["error"]
	IconSuccess = icons["success"]
}

// GetIcon returns an icon SVG by name.
// Available icons: copy, check, download, file, folder, info, warning, error, success
func GetIcon(name string) string {
	return icons[name]
}
