//go:build !windows

package platform

// WindowsVersion represents a Windows version requirement.
// On non-Windows platforms, this is a no-op stub.
type WindowsVersion struct {
	Major uint32
	Minor uint32
	Build uint32
	Name  string
}

// Common Windows version requirements (stubs for non-Windows)
var (
	Windows10v1607 = WindowsVersion{}
	Windows10v1703 = WindowsVersion{}
	Windows10v1809 = WindowsVersion{}
	Windows10v1903 = WindowsVersion{}
	Windows10v2004 = WindowsVersion{}
	Windows11      = WindowsVersion{}
)

// CheckWindowsVersion always returns nil on non-Windows platforms.
func CheckWindowsVersion(required WindowsVersion) error {
	return nil
}

// GetWindowsVersionString returns empty on non-Windows.
func GetWindowsVersionString() string {
	return ""
}
