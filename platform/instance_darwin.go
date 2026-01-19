//go:build darwin

package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// AcquireSingleInstance tries to acquire a file lock to prevent multiple instances.
// The name should be unique to your application (e.g., "com.mycompany.myapp").
// Returns a release function and true if the lock was acquired.
// Returns nil and false if another instance already holds the lock.
//
// Usage:
//
//	release, ok := platform.AcquireSingleInstance("com.mycompany.myapp")
//	if !ok {
//	    // Another instance is running
//	    return
//	}
//	defer release()
func AcquireSingleInstance(name string) (release func(), ok bool) {
	// Create lock file in user's cache directory
	cacheDir, err := UserCachePath()
	if err != nil {
		// Fall back to temp directory
		cacheDir = os.TempDir()
	}

	lockPath := filepath.Join(cacheDir, name+".lock")

	// Create or open the lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// Lock is held by another process
		file.Close()
		return nil, false
	}

	// Write our PID to the file
	file.Truncate(0)
	file.WriteString(string(rune(os.Getpid())))

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		os.Remove(lockPath)
	}, true
}

// IsSingleInstanceRunning checks if another instance with the given name is running.
// This does not acquire the lock, just checks if it exists and is locked.
func IsSingleInstanceRunning(name string) bool {
	cacheDir, err := UserCachePath()
	if err != nil {
		cacheDir = os.TempDir()
	}

	lockPath := filepath.Join(cacheDir, name+".lock")

	// Try to open the lock file
	file, err := os.OpenFile(lockPath, os.O_RDONLY, 0)
	if err != nil {
		// Lock file doesn't exist - no other instance
		return false
	}
	defer file.Close()

	// Try to acquire a non-blocking shared lock
	// If this fails, another process has an exclusive lock
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if err != nil {
		// Cannot acquire lock - another instance is running
		return true
	}

	// We got the lock, release it
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return false
}
