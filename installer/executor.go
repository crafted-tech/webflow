package installer

import (
	"github.com/crafted-tech/webflow"
)

// RunSteps executes steps sequentially with webflow progress UI.
// Returns the first error encountered, or nil if all succeeded.
//
// Example:
//
//	steps := []installer.Step{
//	    installer.StepEnsureDir(targetDir),
//	    installer.StepCopyFile(srcExe, dstExe),
//	}
//	if err := installer.RunSteps(ui, "Installing...", steps); err != nil {
//	    return err
//	}
func RunSteps(ui *webflow.Flow, title string, steps []Step) error {
	return RunStepsWithLogger(ui, title, steps, nil)
}

// RunStepsWithCancel executes steps with cancellation support.
// Returns ErrCancelled if the user cancels during execution.
//
// Example:
//
//	if err := installer.RunStepsWithCancel(ui, "Installing...", steps); err != nil {
//	    if errors.Is(err, installer.ErrCancelled) {
//	        // User cancelled - clean up if needed
//	        return nil
//	    }
//	    return err
//	}
func RunStepsWithCancel(ui *webflow.Flow, title string, steps []Step) error {
	return RunStepsWithLoggerCancel(ui, title, steps, nil)
}

// RunStepsWithLogger executes steps with logging to the provided Logger.
// If log is nil, no logging is performed.
func RunStepsWithLogger(ui *webflow.Flow, title string, steps []Step, log *Logger) error {
	return runStepsInternal(ui, title, steps, log, false)
}

// RunStepsWithLoggerCancel executes steps with logging and cancellation support.
// Returns ErrCancelled if the user cancels during execution.
func RunStepsWithLoggerCancel(ui *webflow.Flow, title string, steps []Step, log *Logger) error {
	return runStepsInternal(ui, title, steps, log, true)
}

func runStepsInternal(ui *webflow.Flow, title string, steps []Step, log *Logger, returnCancelled bool) error {
	var execErr error

	result := ui.ShowProgress(title, func(p webflow.Progress) {
		totalSteps := len(steps)

		for i, step := range steps {
			// Check for cancellation before each step
			if p.Cancelled() {
				if log != nil {
					log.Warn("Installation cancelled by user")
				}
				execErr = ErrCancelled
				return
			}

			// Calculate progress percentage
			progress := float64(i) / float64(totalSteps) * 100
			p.Update(progress, step.Name)

			if log != nil {
				log.Step("Starting: %s", step.Name)
			}

			// Execute the step
			result := step.Action()

			if result.Err != nil {
				if log != nil {
					log.Error("Step '%s' failed: %v", step.Name, result.Err)
				}
				execErr = result.Err
				return
			}

			if result.Skip {
				if log != nil {
					if result.Info != "" {
						log.Info("Step '%s' skipped: %s", step.Name, result.Info)
					} else {
						log.Info("Step '%s' skipped", step.Name)
					}
				}
			} else {
				if log != nil {
					if result.Info != "" {
						log.Info("Step '%s' completed: %s", step.Name, result.Info)
					} else {
						log.Info("Step '%s' completed", step.Name)
					}
				}
			}
		}

		// Final progress update
		p.Update(100, "Complete")
		if log != nil {
			log.Info("All steps completed successfully")
		}
	})

	// Check if cancelled via UI
	if webflow.IsClose(result) {
		if returnCancelled {
			return ErrCancelled
		}
		return nil
	}

	return execErr
}
