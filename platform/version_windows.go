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

	// Windows10v1709 is Windows 10 Fall Creators Update (build 16299)
	// This is the minimum version required for WebView2 Evergreen Runtime.
	Windows10v1709 = WindowsVersion{Major: 10, Minor: 0, Build: 16299, Name: "Windows 10 version 1709"}

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

// Product type constants
const (
	productTypeWorkstation      = 1
	productTypeDomainController = 2
	productTypeServer           = 3
)

// IsWindowsServer returns true if running on Windows Server edition.
func IsWindowsServer() bool {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel32.NewProc("GetProductInfo")
	if err := proc.Find(); err != nil {
		// Fallback: check product type via RtlGetVersion
		return isServerViaProductType()
	}

	major, minor, build := getWindowsVersion()
	var productType uint32

	proc.Call(
		uintptr(major),
		uintptr(minor),
		uintptr(build&0xFFFF), // Service pack major (use build for newer Windows)
		0,                     // Service pack minor
		uintptr(unsafe.Pointer(&productType)),
	)

	// Server product types are in ranges that differ from workstation
	// Simpler approach: use the product type from version info
	return isServerViaProductType()
}

// isServerViaProductType checks product type field from version info.
func isServerViaProductType() bool {
	type osVersionInfoExW struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformId        uint32
		CSDVersion        [128]uint16
		ServicePackMajor  uint16
		ServicePackMinor  uint16
		SuiteMask         uint16
		ProductType       byte
		Reserved          byte
	}

	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")

	var info osVersionInfoExW
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))

	proc.Call(uintptr(unsafe.Pointer(&info)))

	return info.ProductType == productTypeServer || info.ProductType == productTypeDomainController
}

// WebView2VersionError indicates the Windows version doesn't support WebView2.
type WebView2VersionError struct {
	IsServer bool
	Current  string
}

func (e *WebView2VersionError) Error() string {
	if e.IsServer {
		return "Windows Server 2016 or later required"
	}
	return "Windows 10 version 1709 or later required"
}

// CheckWebView2Support verifies the OS supports WebView2 Evergreen Runtime.
// Requirements:
//   - Windows 10 version 1709+ (build 16299) for client editions
//   - Windows Server 2016+ (build 14393) for server editions
//
// Returns nil if supported, or WebView2VersionError describing the requirement.
func CheckWebView2Support() error {
	major, _, build := getWindowsVersion()
	isServer := IsWindowsServer()

	// Determine minimum build based on edition
	var minBuild uint32
	if isServer {
		minBuild = 14393 // Server 2016
	} else {
		minBuild = 16299 // Windows 10 1709
	}

	// Check version
	if major > 10 {
		return nil // Windows 11+ or future versions
	}
	if major == 10 && build >= minBuild {
		return nil
	}

	return &WebView2VersionError{
		IsServer: isServer,
		Current:  GetWindowsVersionString(),
	}
}
