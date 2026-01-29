//go:build windows

package installer

import (
	"fmt"
	"os"
	"time"

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

	// Signal Phase 1 and wait for it to actually exit
	// This matches Inno Setup's approach: get process ID from window handle,
	// signal, then WaitForSingleObject(INFINITE) on the process
	logInfo(log, "Signaling Phase 1 and waiting for process to exit")
	platform.SignalFirstPhaseAndWait(config.FirstPhaseWnd)

	// Small additional delay to ensure file handles are released
	// Inno Setup uses 500ms here - "helps the DelayDeleteFile call succeed on the first try"
	time.Sleep(500 * time.Millisecond)

	// Delete the original uninstaller executable
	logInfo(log, "Deleting original uninstaller")
	if err := platform.DeleteOriginalUninstaller(config.OriginalExePath); err != nil {
		logWarn(log, "Could not delete original uninstaller: %v", err)
	} else {
		logInfo(log, "Original uninstaller deleted successfully")
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
