//go:build windows

package platform

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const uninstallKeyBase = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`

// AppInfo describes an installed application for Windows Add/Remove Programs.
type AppInfo struct {
	// Required fields
	DisplayName     string // Name shown in Add/Remove Programs
	DisplayVersion  string // Version string (e.g., "1.2.3")
	Publisher       string // Publisher/company name
	InstallLocation string // Installation directory
	UninstallString string // Path to uninstaller executable

	// Optional fields
	DisplayIcon   string // Path to icon (defaults to main exe)
	URLInfoAbout  string // Product website
	URLUpdateInfo string // Update URL
	HelpLink      string // Support URL
	InstallDate   string // Install date in YYYYMMDD format
	EstimatedSize uint32 // Size in KB (for display in Add/Remove Programs)
	NoModify      bool   // Hide "Modify" button
	NoRepair      bool   // Hide "Repair" button
}

// RegisterApp creates a Windows uninstall registry entry in Add/Remove Programs.
// The registryKey should be unique to your application (e.g., "CompanyName.ProductName").
func RegisterApp(registryKey string, info AppInfo) error {
	keyPath := uninstallKeyBase + registryKey
	key, _, err := registry.CreateKey(
		registry.LOCAL_MACHINE,
		keyPath,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create registry key: %w", err)
	}
	defer key.Close()

	// Required string values
	stringValues := map[string]string{
		"DisplayName":     info.DisplayName,
		"DisplayVersion":  info.DisplayVersion,
		"Publisher":       info.Publisher,
		"InstallLocation": info.InstallLocation,
		"UninstallString": info.UninstallString,
	}

	// Optional string values
	if info.DisplayIcon != "" {
		stringValues["DisplayIcon"] = info.DisplayIcon
	} else if info.UninstallString != "" {
		// Default to uninstaller icon
		stringValues["DisplayIcon"] = info.UninstallString
	}
	if info.URLInfoAbout != "" {
		stringValues["URLInfoAbout"] = info.URLInfoAbout
	}
	if info.URLUpdateInfo != "" {
		stringValues["URLUpdateInfo"] = info.URLUpdateInfo
	}
	if info.HelpLink != "" {
		stringValues["HelpLink"] = info.HelpLink
	}
	if info.InstallDate != "" {
		stringValues["InstallDate"] = info.InstallDate
	}

	for name, value := range stringValues {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}

	// DWORD values
	if info.NoModify {
		if err := key.SetDWordValue("NoModify", 1); err != nil {
			return fmt.Errorf("set NoModify: %w", err)
		}
	}
	if info.NoRepair {
		if err := key.SetDWordValue("NoRepair", 1); err != nil {
			return fmt.Errorf("set NoRepair: %w", err)
		}
	}
	if info.EstimatedSize > 0 {
		if err := key.SetDWordValue("EstimatedSize", info.EstimatedSize); err != nil {
			return fmt.Errorf("set EstimatedSize: %w", err)
		}
	}

	return nil
}

// UnregisterApp removes the Windows uninstall registry entry.
func UnregisterApp(registryKey string) error {
	keyPath := uninstallKeyBase + registryKey
	err := registry.DeleteKey(registry.LOCAL_MACHINE, keyPath)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete registry key: %w", err)
	}
	return nil
}

// RegisterUserApp creates a per-user uninstall registry entry (HKCU).
// No admin elevation required. Use this for per-user installations.
func RegisterUserApp(registryKey string, info AppInfo) error {
	keyPath := uninstallKeyBase + registryKey
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		keyPath,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create registry key: %w", err)
	}
	defer key.Close()

	// Required string values
	stringValues := map[string]string{
		"DisplayName":     info.DisplayName,
		"DisplayVersion":  info.DisplayVersion,
		"Publisher":       info.Publisher,
		"InstallLocation": info.InstallLocation,
		"UninstallString": info.UninstallString,
	}

	// Optional string values
	if info.DisplayIcon != "" {
		stringValues["DisplayIcon"] = info.DisplayIcon
	} else if info.UninstallString != "" {
		// Default to uninstaller icon
		stringValues["DisplayIcon"] = info.UninstallString
	}
	if info.URLInfoAbout != "" {
		stringValues["URLInfoAbout"] = info.URLInfoAbout
	}
	if info.URLUpdateInfo != "" {
		stringValues["URLUpdateInfo"] = info.URLUpdateInfo
	}
	if info.HelpLink != "" {
		stringValues["HelpLink"] = info.HelpLink
	}
	if info.InstallDate != "" {
		stringValues["InstallDate"] = info.InstallDate
	}

	for name, value := range stringValues {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}

	// DWORD values
	if info.NoModify {
		if err := key.SetDWordValue("NoModify", 1); err != nil {
			return fmt.Errorf("set NoModify: %w", err)
		}
	}
	if info.NoRepair {
		if err := key.SetDWordValue("NoRepair", 1); err != nil {
			return fmt.Errorf("set NoRepair: %w", err)
		}
	}
	if info.EstimatedSize > 0 {
		if err := key.SetDWordValue("EstimatedSize", info.EstimatedSize); err != nil {
			return fmt.Errorf("set EstimatedSize: %w", err)
		}
	}

	return nil
}

// UnregisterUserApp removes the per-user uninstall registry entry.
func UnregisterUserApp(registryKey string) error {
	keyPath := uninstallKeyBase + registryKey
	err := registry.DeleteKey(registry.CURRENT_USER, keyPath)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete registry key: %w", err)
	}
	return nil
}

// FindInstalledApp looks up an existing installation by registry key.
// Returns nil if the app is not installed.
func FindInstalledApp(registryKey string) (*AppInfo, error) {
	keyPath := uninstallKeyBase + registryKey
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		keyPath,
		registry.QUERY_VALUE,
	)
	if err != nil {
		// Key doesn't exist - not installed
		return nil, nil
	}
	defer key.Close()

	info := &AppInfo{}

	if v, _, err := key.GetStringValue("DisplayName"); err == nil {
		info.DisplayName = v
	}
	if v, _, err := key.GetStringValue("DisplayVersion"); err == nil {
		info.DisplayVersion = v
	}
	if v, _, err := key.GetStringValue("Publisher"); err == nil {
		info.Publisher = v
	}
	if v, _, err := key.GetStringValue("InstallLocation"); err == nil {
		info.InstallLocation = v
	}
	if v, _, err := key.GetStringValue("UninstallString"); err == nil {
		info.UninstallString = v
	}
	if v, _, err := key.GetStringValue("DisplayIcon"); err == nil {
		info.DisplayIcon = v
	}

	return info, nil
}

// FindInstalledUserApp looks up a per-user installation by registry key.
// Returns nil if the app is not installed for the current user.
func FindInstalledUserApp(registryKey string) (*AppInfo, error) {
	keyPath := uninstallKeyBase + registryKey
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		keyPath,
		registry.QUERY_VALUE,
	)
	if err != nil {
		// Key doesn't exist - not installed
		return nil, nil
	}
	defer key.Close()

	info := &AppInfo{}

	if v, _, err := key.GetStringValue("DisplayName"); err == nil {
		info.DisplayName = v
	}
	if v, _, err := key.GetStringValue("DisplayVersion"); err == nil {
		info.DisplayVersion = v
	}
	if v, _, err := key.GetStringValue("Publisher"); err == nil {
		info.Publisher = v
	}
	if v, _, err := key.GetStringValue("InstallLocation"); err == nil {
		info.InstallLocation = v
	}
	if v, _, err := key.GetStringValue("UninstallString"); err == nil {
		info.UninstallString = v
	}
	if v, _, err := key.GetStringValue("DisplayIcon"); err == nil {
		info.DisplayIcon = v
	}

	return info, nil
}

// FindInstalledAppByName searches all uninstall entries for a matching DisplayName.
// Returns the AppInfo, the registry key where it was found, and any error.
// Returns (nil, "", nil) if no matching app is found.
// This is useful for detecting installations made by other installers (e.g., MSI/WiX).
func FindInstalledAppByName(displayName string) (*AppInfo, string, error) {
	// Check HKLM (per-machine installations)
	info, key, err := scanForAppByName(registry.LOCAL_MACHINE, displayName)
	if err != nil {
		return nil, "", err
	}
	if info != nil {
		return info, key, nil
	}

	// Check HKCU (per-user installations)
	return scanForAppByName(registry.CURRENT_USER, displayName)
}

// scanForAppByName scans uninstall keys for a matching display name.
func scanForAppByName(root registry.Key, displayName string) (*AppInfo, string, error) {
	uninstallKey, err := registry.OpenKey(
		root,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		registry.ENUMERATE_SUB_KEYS,
	)
	if err != nil {
		return nil, "", nil
	}
	defer uninstallKey.Close()

	subkeys, err := uninstallKey.ReadSubKeyNames(-1)
	if err != nil {
		return nil, "", nil
	}

	for _, subkey := range subkeys {
		productKey, err := registry.OpenKey(
			root,
			uninstallKeyBase+subkey,
			registry.QUERY_VALUE,
		)
		if err != nil {
			continue
		}

		name, _, err := productKey.GetStringValue("DisplayName")
		if err != nil {
			productKey.Close()
			continue
		}

		if strings.EqualFold(name, displayName) {
			info := &AppInfo{DisplayName: name}

			if v, _, err := productKey.GetStringValue("DisplayVersion"); err == nil {
				info.DisplayVersion = v
			}
			if v, _, err := productKey.GetStringValue("Publisher"); err == nil {
				info.Publisher = v
			}
			if v, _, err := productKey.GetStringValue("InstallLocation"); err == nil {
				info.InstallLocation = v
			}
			if v, _, err := productKey.GetStringValue("UninstallString"); err == nil {
				info.UninstallString = v
			}
			if v, _, err := productKey.GetStringValue("DisplayIcon"); err == nil {
				info.DisplayIcon = v
			}

			productKey.Close()
			return info, subkey, nil
		}

		productKey.Close()
	}

	return nil, "", nil
}
