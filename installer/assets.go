package installer

import (
	_ "embed"
)

// Shared assets for installers.
// These are embedded at build time and available for use by installer applications.

//go:embed assets/installer-icon.ico
var InstallerIconICO []byte

//go:embed assets/installer-icon.png
var InstallerIconPNG []byte
