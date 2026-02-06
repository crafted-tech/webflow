package installer

import (
	"fmt"

	"github.com/crafted-tech/webflow/platform"
)

// StepKillProcess creates a Step that terminates all processes with the given name.
// Returns an error if processes exist but cannot be terminated.
func StepKillProcess(exeName string) Step {
	return Step{
		Name: fmt.Sprintf("Stop %s", exeName),
		Action: func() StepResult {
			if err := platform.KillProcessByName(exeName); err != nil {
				return Failed(fmt.Errorf("kill process: %w", err))
			}
			return Success("")
		},
	}
}

// StepKillProcessIfRunning creates a Step that terminates processes if running.
// Skips if no processes with that name are running.
func StepKillProcessIfRunning(exeName string) Step {
	return Step{
		Name: fmt.Sprintf("Stop %s", exeName),
		Action: func() StepResult {
			if !platform.IsProcessRunning(exeName) {
				return Skipped("not running")
			}
			if err := platform.KillProcessByName(exeName); err != nil {
				return Failed(fmt.Errorf("kill process: %w", err))
			}
			return Success("")
		},
	}
}

// StepScheduleSelfDelete creates a Step that schedules the current executable for deletion.
// This is useful for uninstallers that need to delete themselves after exit.
func StepScheduleSelfDelete() Step {
	return Step{
		Name: "Schedule cleanup",
		Action: func() StepResult {
			if err := platform.ScheduleSelfDelete(); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepLaunchAsSessionUser creates a Step that launches an executable as the
// active console session user. On Windows this uses WTS APIs to start the
// process on the user's desktop; on other platforms it starts directly.
// The step succeeds even if the launch fails (best-effort).
func StepLaunchAsSessionUser(exePath string) Step {
	return Step{
		Name: "Relaunch application",
		Action: func() StepResult {
			pid, err := platform.LaunchAsSessionUser(exePath)
			if err != nil {
				return Failed(fmt.Errorf("launch as session user: %w", err))
			}
			return Success(fmt.Sprintf("PID %d", pid))
		},
	}
}

// StepScheduleFileDelete creates a Step that schedules a file for deletion.
// The file will be deleted when it's no longer in use.
func StepScheduleFileDelete(path string) Step {
	return Step{
		Name: "Schedule file cleanup",
		Action: func() StepResult {
			if err := platform.ScheduleFileDelete(path); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}
