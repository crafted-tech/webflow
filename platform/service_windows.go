//go:build windows

package platform

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Service management errors following patterns from github.com/adnsv/go-svc.
var (
	ErrAlreadyInstalled       = errors.New("service already installed")
	ErrNotInstalled           = errors.New("service not installed")
	ErrServiceRunning         = errors.New("service is running")
	ErrServiceNotRunning      = errors.New("service is not running")
	ErrInsufficientPrivileges = errors.New("insufficient privileges")
)

// ServiceConfig holds parameters for installing a Windows service.
// This follows patterns from github.com/adnsv/go-svc.
type ServiceConfig struct {
	Name        string // Service identifier (required)
	DisplayName string // Human-readable name shown in services.msc
	Description string // Service description
	Executable  string // Full path to the executable (required)
	Args        string // Command-line arguments passed at startup
	StartType   uint32 // Start type: mgr.StartAutomatic (default), mgr.StartManual, mgr.StartDisabled
}

// ServiceExists returns true if a Windows service with the given name exists.
func ServiceExists(name string) (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return false, nil // Service doesn't exist
	}
	s.Close()
	return true, nil
}

// IsServiceRunning returns true if the service exists and is running.
func IsServiceRunning(name string) (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return false, nil // Service doesn't exist
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return false, fmt.Errorf("query service status: %w", err)
	}

	return status.State == svc.Running, nil
}

// ServiceStatus returns a string describing the current state of a service.
// Returns "not installed" if the service doesn't exist.
func ServiceStatus(name string) (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return "not installed", nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return "", fmt.Errorf("query service status: %w", err)
	}

	switch status.State {
	case svc.Stopped:
		return "stopped", nil
	case svc.StartPending:
		return "starting", nil
	case svc.StopPending:
		return "stopping", nil
	case svc.Running:
		return "running", nil
	case svc.ContinuePending:
		return "resuming", nil
	case svc.PausePending:
		return "pausing", nil
	case svc.Paused:
		return "paused", nil
	default:
		return "unknown", nil
	}
}

// StartService starts the service and waits for it to enter the running state.
// Returns nil if the service is already running.
func StartService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	// Check current status
	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service status: %w", err)
	}

	if status.State == svc.Running {
		return nil // Already running
	}

	// Start the service
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	// Wait for running state
	return waitForServiceState(s, svc.Running, getServiceTimeout())
}

// StopService stops the service and waits for it to enter the stopped state.
// Returns nil if the service is already stopped or doesn't exist.
func StopService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return nil // Service doesn't exist
	}
	defer s.Close()

	// Check current status
	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service status: %w", err)
	}

	if status.State == svc.Stopped {
		return nil // Already stopped
	}

	// Send stop control
	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	// Wait for stopped state
	return waitForServiceState(s, svc.Stopped, getServiceTimeout())
}

// InstallService installs a new Windows service.
// The service is created with automatic start type.
func InstallService(name, displayName, exePath, args string) error {
	return InstallServiceWithConfig(ServiceConfig{
		Name:        name,
		DisplayName: displayName,
		Executable:  exePath,
		Args:        args,
	})
}

// InstallServiceWithConfig installs a Windows service with full configuration.
// This provides more control over service parameters.
// Following go-svc patterns, this configures automatic recovery (restart on failure).
func InstallServiceWithConfig(cfg ServiceConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if cfg.Executable == "" {
		return fmt.Errorf("executable path is required")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	// Check if already installed
	s, err := m.OpenService(cfg.Name)
	if err == nil {
		s.Close()
		return ErrAlreadyInstalled
	}

	// Determine start type
	startType := cfg.StartType
	if startType == 0 {
		startType = mgr.StartAutomatic
	}

	// Build service config
	config := mgr.Config{
		DisplayName: cfg.DisplayName,
		Description: cfg.Description,
		StartType:   startType,
	}

	// Build binary path with arguments
	binPath := cfg.Executable
	if cfg.Args != "" {
		binPath = fmt.Sprintf(`"%s" %s`, cfg.Executable, cfg.Args)
	}

	s, err = m.CreateService(cfg.Name, binPath, config)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Configure automatic recovery (restart on failure)
	// Following go-svc pattern: restart after 5s for first 3 failures, then 60s
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	err = s.SetRecoveryActions(recoveryActions, uint32((24*time.Hour).Seconds())) // Reset failure count after 24h
	if err != nil {
		// Recovery configuration failure is non-fatal
		_ = err
	}

	return nil
}

// UninstallService removes a Windows service.
// Returns nil if the service doesn't exist.
func UninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return nil // Service doesn't exist
	}
	defer s.Close()

	// Stop the service first if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		s.Control(svc.Stop)
		// Wait for stop with short timeout
		waitForServiceState(s, svc.Stopped, 10*time.Second)
	}

	// Delete the service
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}

	return nil
}

// waitForServiceState waits for a service to reach the target state.
func waitForServiceState(s *mgr.Service, target svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := s.Query()
			if err != nil {
				return fmt.Errorf("query service status: %w", err)
			}
			if status.State == target {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for service state %d", target)
			}
		}
	}
}

// getServiceTimeout returns the system's service wait timeout.
// Following go-svc pattern: reads from registry, defaults to 20 seconds.
func getServiceTimeout() time.Duration {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control`,
		registry.QUERY_VALUE)
	if err != nil {
		return 20 * time.Second
	}
	defer key.Close()

	val, _, err := key.GetIntegerValue("WaitToKillServiceTimeout")
	if err != nil {
		return 20 * time.Second
	}

	return time.Duration(val) * time.Millisecond
}
