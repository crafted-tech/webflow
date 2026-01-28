package installer

import "errors"

// ErrCancelled is returned when an operation was cancelled by the user.
var ErrCancelled = errors.New("operation cancelled")

// StepResult represents the outcome of a step execution.
type StepResult struct {
	// Skip indicates the step was skipped (already done, not needed).
	// When Skip is true, the step is counted as successful.
	Skip bool

	// Info contains a success or informational message.
	// For skipped steps, this explains why it was skipped.
	Info string

	// Err contains the error if the step failed.
	// A nil Err indicates success (or skip if Skip is true).
	Err error
}

// Success creates a successful StepResult with an optional info message.
func Success(info string) StepResult {
	return StepResult{Info: info}
}

// Skipped creates a StepResult indicating the step was skipped.
func Skipped(reason string) StepResult {
	return StepResult{Skip: true, Info: reason}
}

// Failed creates a StepResult with an error.
func Failed(err error) StepResult {
	return StepResult{Err: err}
}

// Step represents a named action to be executed during installation.
type Step struct {
	// Name is the display name for the step (shown in progress UI).
	Name string

	// Action executes the step and returns the result.
	// The action should check for cancellation if it's long-running.
	Action func() StepResult
}

// SimpleStep creates a Step from a simple function that returns error.
// This is a convenience for the common case where steps don't need
// to return skip/info, just success or failure.
//
// Example:
//
//	installer.SimpleStep("Configure", func() error {
//	    return writeConfig(targetDir)
//	})
func SimpleStep(name string, action func() error) Step {
	return Step{
		Name: name,
		Action: func() StepResult {
			if err := action(); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}
