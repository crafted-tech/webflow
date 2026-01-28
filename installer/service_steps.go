package installer

import (
	"fmt"

	"github.com/crafted-tech/webflow/platform"
)

// StepStopService creates a Step that stops a Windows service.
// Skips if the service is not running or doesn't exist.
func StepStopService(name string) Step {
	return Step{
		Name: fmt.Sprintf("Stop %s service", name),
		Action: func() StepResult {
			running, err := platform.IsServiceRunning(name)
			if err != nil {
				return Failed(err)
			}
			if !running {
				return Skipped("not running")
			}
			if err := platform.StopService(name); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepStartService creates a Step that starts a Windows service.
// Skips if the service is already running.
func StepStartService(name string) Step {
	return Step{
		Name: fmt.Sprintf("Start %s service", name),
		Action: func() StepResult {
			running, err := platform.IsServiceRunning(name)
			if err != nil {
				return Failed(err)
			}
			if running {
				return Skipped("already running")
			}
			if err := platform.StartService(name); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepInstallService creates a Step that installs a Windows service.
// Skips if the service already exists.
func StepInstallService(name, displayName, exePath, args string) Step {
	return StepInstallServiceWithConfig(platform.ServiceConfig{
		Name:        name,
		DisplayName: displayName,
		Executable:  exePath,
		Args:        args,
	})
}

// StepInstallServiceWithConfig creates a Step that installs a Windows service with full configuration.
// Skips if the service already exists.
func StepInstallServiceWithConfig(cfg platform.ServiceConfig) Step {
	return Step{
		Name: fmt.Sprintf("Install %s service", cfg.Name),
		Action: func() StepResult {
			exists, err := platform.ServiceExists(cfg.Name)
			if err != nil {
				return Failed(err)
			}
			if exists {
				return Skipped("already installed")
			}
			if err := platform.InstallServiceWithConfig(cfg); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepUninstallService creates a Step that uninstalls a Windows service.
// Skips if the service doesn't exist.
func StepUninstallService(name string) Step {
	return Step{
		Name: fmt.Sprintf("Uninstall %s service", name),
		Action: func() StepResult {
			exists, err := platform.ServiceExists(name)
			if err != nil {
				return Failed(err)
			}
			if !exists {
				return Skipped("not installed")
			}
			if err := platform.UninstallService(name); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}
