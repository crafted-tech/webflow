//go:build windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"

	"github.com/crafted-tech/webflow/platform"
)

// RunSecondPhase handles Phase 2 of two-phase self-delete.
// Call this early in main() when platform.IsSecondPhase() returns true.
// The caller should exit/return after this function returns.
//
// Phase 2 runs from a temp copy of the uninstaller and is responsible for:
//   - Signaling Phase 1 that it can proceed to completion
//   - Deleting the original uninstaller executable (with retries)
//   - Attempting to delete itself (best effort)
//
// Example:
//
//	func main() {
//	    opts := parseArgs()
//	    if platform.IsSecondPhase() {
//	        installer.RunSecondPhase(opts.LogFile)
//	        return
//	    }
//	    // ... rest of main
//	}
func RunSecondPhase(logFile string) {
	config, err := platform.GetSecondPhaseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Phase 2 config error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	var log *Logger
	if logFile != "" {
		log, _ = NewLoggerToFile(logFile)
	} else {
		log, _ = NewLogger("uninstall-phase2")
	}
	if log != nil {
		defer log.Close()
	}

	logStep(log, "Starting Phase 2 (self-delete)")
	logInfo(log, "Original uninstaller: %s", config.OriginalExePath)

	// Create done signal file BEFORE signaling Phase 1
	// This file is held open until we exit, preventing cleanup while we're running
	logInfo(log, "Creating done signal file")
	doneHandle := platform.CreateDoneSignalFile()
	_ = doneHandle // Intentionally never closed - OS closes on exit

	// Delete the original exe BEFORE signaling Phase 1.
	// Use NTFS ADS technique to delete the running exe (POSIX unlink).
	// This makes Phase 2 fully resilient to Job Object kills: even if
	// Phase 2 dies when Phase 1 exits, the file is already gone.
	// Falls back to rename + MoveFileEx reboot cleanup on older Windows
	// or non-NTFS filesystems.
	deleted := false
	renamedPath := ""

	logInfo(log, "Deleting %s via POSIX delete", filepath.Base(config.OriginalExePath))
	if err := platform.DeleteRunningExe(config.OriginalExePath); err != nil {
		logWarn(log, "POSIX delete failed: %v — falling back to rename", err)
		renamedPath = config.OriginalExePath + ".removing"
		if err := os.Rename(config.OriginalExePath, renamedPath); err != nil {
			logWarn(log, "Rename failed: %v", err)
			renamedPath = ""
		} else {
			logInfo(log, "Renamed to %s", filepath.Base(renamedPath))
			if pathPtr, err := windows.UTF16PtrFromString(renamedPath); err == nil {
				if err := windows.MoveFileEx(pathPtr, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err == nil {
					logInfo(log, "Scheduled %s for deletion on reboot", filepath.Base(renamedPath))
				}
			}
		}
	} else {
		deleted = true
		logInfo(log, "Deleted successfully: %s", filepath.Base(config.OriginalExePath))
	}

	// Signal Phase 1 and wait for it to actually exit.
	// NOTE: If inside a Job Object with KILL_ON_JOB_CLOSE, Phase 2 dies here.
	// That's OK — the file is already deleted (or renamed + scheduled).
	logInfo(log, "Signaling Phase 1 and waiting for process to exit")
	platform.SignalFirstPhaseAndWait(config.FirstPhaseWnd)

	// If we reach here, Phase 2 survived (no Job Object killed us).
	time.Sleep(500 * time.Millisecond)

	// If rename fallback was used, try to delete the renamed file now
	// that Phase 1 has exited and released its mapped image section.
	if !deleted && renamedPath != "" {
		logInfo(log, "Deleting %s", filepath.Base(renamedPath))
		if err := platform.DeleteOriginalUninstaller(renamedPath); err != nil {
			logWarn(log, "Could not delete %s: %v", filepath.Base(renamedPath), err)
		} else {
			deleted = true
			logInfo(log, "Deleted successfully: %s", filepath.Base(renamedPath))
		}
	}

	// Try to remove the now-empty install directory.
	if deleted {
		dir := filepath.Dir(config.OriginalExePath)
		if err := os.Remove(dir); err == nil {
			logInfo(log, "Removed empty install directory: %s", dir)
		}
	}

	// Try to delete ourselves (best effort - cleanup handles failures)
	platform.TryDeleteSelf()

	logStep(log, "Phase 2 complete")
}

// Helper functions to handle nil logger gracefully
func logInfo(log *Logger, format string, args ...any) {
	if log != nil {
		log.Info(format, args...)
	}
}

func logWarn(log *Logger, format string, args ...any) {
	if log != nil {
		log.Warn(format, args...)
	}
}

func logStep(log *Logger, format string, args ...any) {
	if log != nil {
		log.Step(format, args...)
	}
}
