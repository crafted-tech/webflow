// Package installer provides utilities for building webflow-based installers.
//
// This package offers reusable components that installers can pick from:
//   - Logger: Unified logging with in-memory buffer and file output
//   - Step execution: Run steps with webflow progress UI
//   - Common step functions: Reusable implementations (copy files, create dirs, etc.)
//   - Service management: Windows service start/stop/install/uninstall utilities
//   - Detection helpers: Registry queries, version comparison, process detection
//
// # Design Philosophy
//
// This package provides utilities, not a framework. Each installer maintains its own:
//   - Wizard flow and navigation
//   - State/plan struct
//   - Configuration generation
//   - UI screens
//
// The utilities reduce boilerplate while allowing full customization.
//
// # Basic Usage
//
// Create a logger:
//
//	log, err := installer.NewLogger("myapp-install")
//	if err != nil {
//	    return err
//	}
//	defer log.Close()
//
// Build and run installation steps:
//
//	steps := []installer.Step{
//	    installer.StepEnsureDir(targetDir),
//	    installer.StepCopyFile(srcExe, dstExe),
//	    installer.SimpleStep("Configure", func() error {
//	        return writeConfig(targetDir)
//	    }),
//	}
//	return installer.RunSteps(ui, "Installing...", steps)
//
// # Step Pattern
//
// Steps are simple structs with a name and action function:
//
//	type Step struct {
//	    Name   string
//	    Action func() StepResult
//	}
//
// The StepResult indicates success, skip, or failure:
//
//	type StepResult struct {
//	    Skip bool   // Step was skipped (already done, not needed)
//	    Info string // Success/info message
//	    Err  error  // Error (nil = success)
//	}
//
// Use SimpleStep for actions that just return error:
//
//	installer.SimpleStep("Do something", func() error {
//	    return doSomething()
//	})
package installer
