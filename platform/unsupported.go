//go:build !windows && !linux && !darwin

package platform

// The platform package currently only supports Windows, Linux, and macOS.
// If you see this compilation error, you are attempting to build for an unsupported platform.
//
// To add support for your platform, implement the required functions in platform_{os}.go files.

const platformUnsupported = "platform package requires Windows, Linux, or macOS - see platform/unsupported.go"

// This line intentionally causes a compile error on unsupported platforms.
// The error message above explains why.
var _ int = platformUnsupported
