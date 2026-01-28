package installer

import (
	"strconv"
	"strings"
)

// CompareVersions compares two semantic version strings.
// Returns:
//   - negative if v1 < v2
//   - zero if v1 == v2
//   - positive if v1 > v2
//
// Handles versions like "1.0.0", "1.2", "1.2.3-beta", etc.
// Non-numeric suffixes are compared lexicographically.
func CompareVersions(v1, v2 string) int {
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare numeric parts
	maxLen := max(len(parts1), len(parts2))
	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// parseVersion extracts numeric parts from a version string.
// "1.2.3" -> [1, 2, 3]
// "1.2.3-beta" -> [1, 2, 3] (suffix ignored)
func parseVersion(v string) []int {
	// Remove any leading 'v' or 'V'
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	// Split by dots
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		// Handle suffixes like "3-beta" -> take "3"
		if idx := strings.IndexAny(part, "-+_"); idx > 0 {
			part = part[:idx]
		}

		n, err := strconv.Atoi(part)
		if err != nil {
			continue // Skip non-numeric parts
		}
		result = append(result, n)
	}

	return result
}

// IsNewerVersion returns true if newVersion is newer than oldVersion.
func IsNewerVersion(newVersion, oldVersion string) bool {
	return CompareVersions(newVersion, oldVersion) > 0
}

// IsOlderVersion returns true if newVersion is older than oldVersion.
func IsOlderVersion(newVersion, oldVersion string) bool {
	return CompareVersions(newVersion, oldVersion) < 0
}

// IsSameVersion returns true if the versions are equal.
func IsSameVersion(v1, v2 string) bool {
	return CompareVersions(v1, v2) == 0
}

// InstallAction represents the type of installation action.
type InstallAction int

const (
	ActionFreshInstall InstallAction = iota
	ActionUpgrade
	ActionDowngrade
	ActionReinstall
)

// String returns the action name.
func (a InstallAction) String() string {
	switch a {
	case ActionFreshInstall:
		return "Fresh Install"
	case ActionUpgrade:
		return "Upgrade"
	case ActionDowngrade:
		return "Downgrade"
	case ActionReinstall:
		return "Reinstall"
	default:
		return "Install"
	}
}

// DetermineAction determines the installation action based on versions.
func DetermineAction(existingVersion, newVersion string) InstallAction {
	if existingVersion == "" {
		return ActionFreshInstall
	}

	cmp := CompareVersions(newVersion, existingVersion)
	switch {
	case cmp > 0:
		return ActionUpgrade
	case cmp < 0:
		return ActionDowngrade
	default:
		return ActionReinstall
	}
}
