//go:build windows

package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

// StartMenuPath returns the path to the common (all users) Start Menu Programs folder.
// Example: C:\ProgramData\Microsoft\Windows\Start Menu\Programs
func StartMenuPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_CommonPrograms, 0)
}

// UserStartMenuPath returns the path to the current user's Start Menu Programs folder.
// Example: C:\Users\<user>\AppData\Roaming\Microsoft\Windows\Start Menu\Programs
func UserStartMenuPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
}

// DesktopPath returns the path to the common (all users) Desktop folder.
// Example: C:\Users\Public\Desktop
func DesktopPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_PublicDesktop, 0)
}

// UserDesktopPath returns the path to the current user's Desktop folder.
// Example: C:\Users\<user>\Desktop
func UserDesktopPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
}

// ProgramFilesPath returns the path to the Program Files folder.
// Example: C:\Program Files
func ProgramFilesPath() string {
	path := os.Getenv("ProgramFiles")
	if path == "" {
		return `C:\Program Files`
	}
	return path
}

// ProgramFilesX86Path returns the path to the Program Files (x86) folder.
// Example: C:\Program Files (x86)
func ProgramFilesX86Path() string {
	path := os.Getenv("ProgramFiles(x86)")
	if path == "" {
		return `C:\Program Files (x86)`
	}
	return path
}

// LocalAppDataPath returns the path to the current user's local app data folder.
// Example: C:\Users\<user>\AppData\Local
func LocalAppDataPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
}

// RoamingAppDataPath returns the path to the current user's roaming app data folder.
// Example: C:\Users\<user>\AppData\Roaming
func RoamingAppDataPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0)
}

// ProgramDataPath returns the path to the common program data folder.
// Example: C:\ProgramData
func ProgramDataPath() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
}
