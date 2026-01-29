//go:build windows

package platform

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsVersion represents a Windows version requirement.
type WindowsVersion struct {
	Major uint32
	Minor uint32
	Build uint32
	Name  string // Human-readable name, e.g., "Windows 10 version 1607"
}

// Common Windows version requirements
var (
	// Windows10v1607 is Windows 10 Anniversary Update (build 14393)
	Windows10v1607 = WindowsVersion{Major: 10, Minor: 0, Build: 14393, Name: "Windows 10 version 1607"}

	// Windows10v1703 is Windows 10 Creators Update (build 15063)
	Windows10v1703 = WindowsVersion{Major: 10, Minor: 0, Build: 15063, Name: "Windows 10 version 1703"}

	// Windows10v1809 is Windows 10 October 2018 Update (build 17763)
	Windows10v1809 = WindowsVersion{Major: 10, Minor: 0, Build: 17763, Name: "Windows 10 version 1809"}

	// Windows10v1903 is Windows 10 May 2019 Update (build 18362)
	Windows10v1903 = WindowsVersion{Major: 10, Minor: 0, Build: 18362, Name: "Windows 10 version 1903"}

	// Windows10v2004 is Windows 10 May 2020 Update (build 19041)
	Windows10v2004 = WindowsVersion{Major: 10, Minor: 0, Build: 19041, Name: "Windows 10 version 2004"}

	// Windows11 is Windows 11 (build 22000)
	Windows11 = WindowsVersion{Major: 10, Minor: 0, Build: 22000, Name: "Windows 11"}
)

// CheckWindowsVersion verifies the OS meets minimum requirements.
// Returns nil if OK, or an error describing the version mismatch.
func CheckWindowsVersion(required WindowsVersion) error {
	major, _, build := getWindowsVersion()

	// Compare major version first
	if major > required.Major {
		return nil
	}
	if major == required.Major && build >= required.Build {
		return nil
	}

	return &WindowsVersionError{Required: required}
}

// WindowsVersionError indicates the Windows version is not supported.
type WindowsVersionError struct {
	Required WindowsVersion
}

func (e *WindowsVersionError) Error() string {
	return fmt.Sprintf("%s or later required", e.Required.Name)
}

// getWindowsVersion returns major, minor, build numbers.
func getWindowsVersion() (major, minor, build uint32) {
	// Use RtlGetVersion which returns accurate info (unlike GetVersion)
	type osVersionInfoEx struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformId        uint32
		CSDVersion        [128]uint16
		// ... other fields not needed
	}

	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")

	var info osVersionInfoEx
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))

	proc.Call(uintptr(unsafe.Pointer(&info)))

	return info.MajorVersion, info.MinorVersion, info.BuildNumber
}

// GetWindowsVersionString returns a human-readable version string.
func GetWindowsVersionString() string {
	major, minor, build := getWindowsVersion()
	return fmt.Sprintf("Windows %d.%d (Build %d)", major, minor, build)
}
