// Package platform provides platform-specific utilities for installer and system applications.
//
// Currently, this package only supports Windows. Attempting to use it on other
// platforms will result in a compile-time error.
//
// # Features
//
// The package provides the following functionality:
//
//   - Clipboard: Copy text to the system clipboard
//   - Elevation: UAC elevation handling (check/request admin privileges)
//   - Single Instance: Prevent multiple instances of an application
//   - App Registration: Register/unregister apps in Add/Remove Programs
//   - Paths: Get common system paths (Start Menu, Desktop, etc.)
//   - Process: Find and kill processes by name
//   - Self-Delete: Schedule executable deletion after exit
//   - Shortcuts: Create and delete Windows shortcuts (.lnk files)
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
