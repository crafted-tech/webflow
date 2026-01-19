//go:build linux

package platform

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// UserDesktopPath returns the path to the current user's Desktop folder.
// It follows the XDG Base Directory specification.
func UserDesktopPath() (string, error) {
	// Check environment variable first
	if dir := os.Getenv("XDG_DESKTOP_DIR"); dir != "" {
		return dir, nil
	}

	// Try to read from user-dirs.dirs
	if dir := readUserDir("XDG_DESKTOP_DIR"); dir != "" {
		return dir, nil
	}

	// Fall back to ~/Desktop
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Desktop"), nil
}

// UserDataPath returns the path to the current user's data directory.
// This is $XDG_DATA_HOME or ~/.local/share by default.
func UserDataPath() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// UserConfigPath returns the path to the current user's config directory.
// This is $XDG_CONFIG_HOME or ~/.config by default.
func UserConfigPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// UserCachePath returns the path to the current user's cache directory.
// This is $XDG_CACHE_HOME or ~/.cache by default.
func UserCachePath() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// ApplicationsPath returns the path to the user's applications directory.
// This is where .desktop files are stored for the current user.
func ApplicationsPath() (string, error) {
	dataPath, err := UserDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, "applications"), nil
}

// readUserDir reads a directory path from ~/.config/user-dirs.dirs
func readUserDir(key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(home, ".config", "user-dirs.dirs")
	file, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY="value" format
		if strings.HasPrefix(line, key+"=") {
			value := strings.TrimPrefix(line, key+"=")
			value = strings.Trim(value, "\"")
			// Expand $HOME
			value = strings.ReplaceAll(value, "$HOME", home)
			return value
		}
	}

	return ""
}
