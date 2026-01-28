// Package platform provides platform-specific utilities for installer and system applications.
//
// # Platform Support
//
// Most features are Windows-only, but service management supports multiple platforms:
//   - Windows: Service Control Manager (SCM)
//   - Linux: systemd
//   - macOS: launchd
//
// # Features
//
// The package provides the following functionality:
//
//   - Clipboard: Copy text to the system clipboard (Windows)
//   - Elevation: UAC elevation handling (Windows)
//   - Single Instance: Prevent multiple instances (Windows)
//   - App Registration: Register/unregister apps in Add/Remove Programs (Windows)
//   - Paths: Get common system paths (Windows)
//   - Process: Find and kill processes by name (Windows)
//   - Self-Delete: Schedule executable deletion after exit (Windows)
//   - Shortcuts: Create and delete shortcuts (Windows)
//   - Service Management: Install/uninstall/start/stop system services (Windows/Linux/macOS)
//
// # Example Usage
//
//	// Ensure single instance
//	release, ok := platform.AcquireSingleInstance("MyApp")
//	if !ok {
//	    // Another instance is running
//	    return
//	}
//	defer release()
//
//	// Ensure admin privileges
//	if err := platform.EnsureElevated(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register app in Add/Remove Programs
//	platform.RegisterApp("Company.MyApp", platform.AppInfo{
//	    DisplayName:     "My Application",
//	    DisplayVersion:  "1.0.0",
//	    Publisher:       "My Company",
//	    InstallLocation: installDir,
//	    UninstallString: filepath.Join(installDir, "uninstall.exe"),
//	})
package platform
