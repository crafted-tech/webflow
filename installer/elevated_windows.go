//go:build windows

package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/crafted-tech/webflow"
	"github.com/crafted-tech/webflow/platform"
	"golang.org/x/sys/windows"
)

// ElevatedExecConfig configures an elevated subprocess execution.
type ElevatedExecConfig struct {
	// Title is the progress bar title (translated).
	Title string

	// ArgsBuilder builds the CLI args string including the progress file path.
	// It receives the path to the IPC progress file.
	ArgsBuilder func(progressPath string) string

	// PIDTimeout is the maximum time to wait for the child to write its PID.
	// Defaults to 30 seconds if zero.
	PIDTimeout time.Duration
}

// ElevatedOutcome holds the result from the elevated child.
type ElevatedOutcome struct {
	// RawResult is the JSON-encoded result from the child process.
	// The caller unmarshals this into their own result struct.
	RawResult json.RawMessage

	// ExitCode is the child process exit code (only meaningful when RawResult is nil).
	ExitCode uint32
}

// RunElevatedWithProgress launches an elevated child process, shows a progress
// UI, and polls for completion. Returns the raw JSON result or error.
//
// Flow: CreateProgressFile → LaunchElevated(args) → poll PID → OpenProcessByPID
// → ShowProgress with WaitForSingleObject(200ms) + ReadNewLines loop → return result.
//
// Returns platform.ErrElevationDeclined if user rejects UAC.
// Returns ErrCancelled if user cancels during progress.
func RunElevatedWithProgress(ui *webflow.Flow, cfg ElevatedExecConfig) (*ElevatedOutcome, error) {
	pidTimeout := cfg.PIDTimeout
	if pidTimeout == 0 {
		pidTimeout = 30 * time.Second
	}

	// Create progress file for IPC.
	progressPath, err := CreateProgressFile()
	if err != nil {
		return nil, fmt.Errorf("create progress file: %w", err)
	}
	defer os.Remove(progressPath)

	// Launch elevated worker via ShellExecute("runas").
	args := cfg.ArgsBuilder(progressPath)
	if err := platform.LaunchElevated(args); err != nil {
		return nil, err // includes ErrElevationDeclined
	}

	// Poll for the child's PID line and get a waitable handle.
	reader := NewProgressReader(progressPath)
	var childPID uint32

	deadline := time.Now().Add(pidTimeout)
	for time.Now().Before(deadline) {
		lines, _ := reader.ReadNewLines()
		for _, line := range lines {
			if line.Type == "PID" {
				childPID = line.PID
				break
			}
		}
		if childPID != 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if childPID == 0 {
		return nil, fmt.Errorf("elevated worker did not report PID within timeout")
	}

	processHandle, err := platform.OpenProcessByPID(childPID)
	if err != nil {
		return nil, fmt.Errorf("open worker process: %w", err)
	}
	defer windows.CloseHandle(processHandle)

	// Show progress UI while polling the child process.
	var rawResult json.RawMessage
	var progressErr error

	ui.ShowProgress(cfg.Title, func(p webflow.Progress) {
		for {
			// Check for user cancellation.
			if p.Cancelled() {
				progressErr = ErrCancelled
				break
			}

			// Wait on the process handle with a 200ms timeout (acts as poll timer).
			event, _ := windows.WaitForSingleObject(processHandle, 200)

			// Read new progress lines.
			lines, _ := reader.ReadNewLines()
			for _, line := range lines {
				switch line.Type {
				case "PROGRESS":
					p.Update(line.Percent, line.Message)
				case "RESULT":
					rawResult = line.RawResult
				}
			}

			// If the child process has exited, read remaining lines and break.
			if event == windows.WAIT_OBJECT_0 {
				remaining, _ := reader.ReadNewLines()
				for _, line := range remaining {
					if line.Type == "RESULT" {
						rawResult = line.RawResult
					}
				}
				break
			}
		}

		// If no RESULT line was received, the child crashed or was cancelled.
		if rawResult == nil && progressErr == nil {
			exitCode, _ := platform.GetProcessExitCode(processHandle)
			progressErr = fmt.Errorf("elevated worker exited without result (exit code %d)", exitCode)
		}

		if progressErr == nil {
			p.Update(100, "")
		}
	})

	if progressErr != nil {
		return nil, progressErr
	}
	if rawResult == nil {
		return nil, fmt.Errorf("no result from elevated worker")
	}

	return &ElevatedOutcome{RawResult: rawResult}, nil
}
