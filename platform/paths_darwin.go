//go:build darwin

package platform

import (
	"os"
	"path/filepath"
)

// UserDesktopPath returns the path to the current user's Desktop folder.
func UserDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Desktop"), nil
}

// UserDataPath returns the path to the current user's Application Support directory.
// This is ~/Library/Application Support on macOS.
func UserDataPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support"), nil
}

// UserConfigPath returns the path to the current user's Preferences directory.
// This is ~/Library/Preferences on macOS.
func UserConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Preferences"), nil
}

// UserCachePath returns the path to the current user's cache directory.
// This is ~/Library/Caches on macOS.
func UserCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Caches"), nil
}

// UserLogsPath returns the path to the current user's logs directory.
// This is ~/Library/Logs on macOS.
func UserLogsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs"), nil
}

// ApplicationsPath returns the path to the user's Applications directory.
// This is ~/Applications on macOS (user-installed apps).
func ApplicationsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications"), nil
}

// SystemApplicationsPath returns the path to the system Applications directory.
// This is /Applications on macOS.
func SystemApplicationsPath() string {
	return "/Applications"
}

// LaunchAgentsPath returns the path to the user's LaunchAgents directory.
// This is where user-level launch agents (auto-start services) are stored.
func LaunchAgentsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}
