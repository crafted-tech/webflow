//go:build !windows

package platform

import (
	"os"
)

// SelfDeleteConfig holds configuration for two-phase self-delete.
// On non-Windows platforms, this is not used.
type SelfDeleteConfig struct {
	OriginalExePath string // Path to original uninstaller
	FirstPhaseWnd   uintptr
}

// IsSecondPhase checks if we're running as Phase 2 of self-delete.
// On non-Windows platforms, always returns false.
func IsSecondPhase() bool {
	return false
}

// GetSecondPhaseConfig parses command-line for Phase 2 configuration.
// On non-Windows platforms, always returns nil.
func GetSecondPhaseConfig() (*SelfDeleteConfig, error) {
	return nil, nil
}

// FilterSecondPhaseArgs returns command-line args with Phase 2 flags removed.
// On non-Windows platforms, returns all args unchanged.
func FilterSecondPhaseArgs() []string {
	return os.Args[1:]
}

// RunFirstPhase creates a temp copy and spawns Phase 2.
// On non-Windows platforms, this is a no-op since self-delete works directly.
func RunFirstPhase() error {
	return nil
}

// DeleteOriginalUninstaller deletes the original uninstaller EXE.
// On non-Windows platforms, this is a no-op.
func DeleteOriginalUninstaller(originalPath string) error {
	return os.Remove(originalPath)
}

// SignalFirstPhase sends completion message to Phase 1.
// On non-Windows platforms, this is a no-op.
func SignalFirstPhase(wnd uintptr) {
}

// TryDeleteSelf attempts to delete the current executable.
// On non-Windows/Linux, this works directly.
func TryDeleteSelf() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	os.Remove(exe)
}

// CleanupResidualTempDirs scans and cleans up old uninstall temp directories.
// On non-Windows platforms, this is a no-op.
func CleanupResidualTempDirs() error {
	return nil
}

// DelayDeleteFile attempts to delete a file with retries and delays.
// On non-Windows platforms, just tries direct deletion.
func DelayDeleteFile(path string, maxTries int, firstDelayMS, subsequentDelayMS int) error {
	return os.Remove(path)
}

// ScheduleSelfDelete arranges for the current executable to be deleted.
// On Linux/Unix, we can delete a running executable directly.
func ScheduleSelfDelete() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return os.Remove(exe)
}

// ScheduleFileDelete schedules a file for deletion.
// On non-Windows platforms, just deletes directly.
func ScheduleFileDelete(filePath string) error {
	return os.Remove(filePath)
}

// DeleteFileWhenFree attempts to delete a file.
// On non-Windows platforms, just deletes directly.
func DeleteFileWhenFree(path string) error {
	return os.Remove(path)
}
