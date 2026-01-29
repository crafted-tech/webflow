//go:build !windows

package installer

// RunSecondPhase is a no-op on non-Windows platforms.
// Two-phase self-delete is only needed on Windows where running executables
// cannot be deleted directly.
func RunSecondPhase(logFile string) {
	// No-op on non-Windows
}
