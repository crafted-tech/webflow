//go:build windows

package platform

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// AcquireSingleInstance tries to acquire a named mutex to prevent multiple instances.
// The name should be unique to your application (e.g., "MyCompany.MyApp").
// Returns a release function and true if the lock was acquired.
// Returns nil and false if another instance already holds the lock.
//
// Usage:
//
//	release, ok := platform.AcquireSingleInstance("MyApp")
//	if !ok {
//	    // Another instance is running
//	    return
//	}
//	defer release()
func AcquireSingleInstance(name string) (release func(), ok bool) {
	// Use Global\ prefix to work across all sessions
	mutexName, _ := windows.UTF16PtrFromString("Global\\" + name)

	handle, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		// ERROR_ALREADY_EXISTS means another instance has the mutex
		if err == windows.ERROR_ALREADY_EXISTS {
			// Close the handle we got (it's a reference to the existing mutex)
			if handle != 0 {
				windows.CloseHandle(handle)
			}
			return nil, false
		}
		// Other errors - proceed anyway (fail open)
		return func() { windows.CloseHandle(handle) }, true
	}

	return func() { windows.CloseHandle(handle) }, true
}

var procAllowSetForegroundWindow = syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// AllowSetForegroundForAnyProcess grants any process the one-time right to
// call SetForegroundWindow. Call this from a second instance (which holds
// foreground rights as a user-launched process) before signaling the first
// instance to come to foreground.
func AllowSetForegroundForAnyProcess() {
	const ASFW_ANY = 0xFFFFFFFF
	procAllowSetForegroundWindow.Call(ASFW_ANY)
}

// IsSingleInstanceRunning checks if another instance with the given name is running.
// This does not acquire the lock, just checks if it exists.
func IsSingleInstanceRunning(name string) bool {
	mutexName, _ := windows.UTF16PtrFromString("Global\\" + name)

	// Try to open existing mutex
	handle, err := windows.OpenMutex(windows.SYNCHRONIZE, false, mutexName)
	if err != nil {
		// Mutex doesn't exist - no other instance running
		return false
	}
	windows.CloseHandle(handle)
	return true
}
