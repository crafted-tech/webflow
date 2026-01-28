//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Service management errors following patterns from github.com/adnsv/go-svc.
var (
	ErrAlreadyInstalled       = errors.New("service already installed")
	ErrNotInstalled           = errors.New("service not installed")
	ErrServiceRunning         = errors.New("service is running")
	ErrServiceNotRunning      = errors.New("service is not running")
	ErrInsufficientPrivileges = errors.New("insufficient privileges")
)

// ServiceConfig holds parameters for installing a Linux service (systemd).
// This follows patterns from github.com/adnsv/go-svc.
type ServiceConfig struct {
	Name        string // Service identifier (required)
	DisplayName string // Human-readable name (used in Description)
	Description string // Service description
	Executable  string // Full path to the executable (required)
	Args        string // Command-line arguments passed at startup
	StartType   uint32 // Ignored on Linux (services always start automatically unless disabled)
}

// systemdUnitTemplate is the template for generating systemd unit files.
// Following go-svc patterns with Restart=on-failure configuration.
const systemdUnitTemplate = `[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

type systemdUnitData struct {
	Description string
	ExecStart   string
}

// unitFilePath returns the path to the systemd unit file for a service.
func unitFilePath(name string) string {
	return filepath.Join("/etc/systemd/system", name+".service")
}

// checkPrivileges verifies root access is available.
// Following go-svc pattern: checks via id -g command.
func checkPrivileges() error {
	cmd := exec.Command("id", "-g")
	output, err := cmd.Output()
	if err != nil {
		return ErrInsufficientPrivileges
	}
	if strings.TrimSpace(string(output)) != "0" {
		return ErrInsufficientPrivileges
	}
	return nil
}

// ServiceExists returns true if a systemd service with the given name exists.
func ServiceExists(name string) (bool, error) {
	unitPath := unitFilePath(name)
	_, err := os.Stat(unitPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// IsServiceRunning returns true if the service is currently running.
func IsServiceRunning(name string) (bool, error) {
	cmd := exec.Command("systemctl", "is-active", name)
	output, err := cmd.Output()
	if err != nil {
		// is-active returns non-zero exit code if not active
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "active", nil
}

// ServiceStatus returns a string describing the current state of a service.
// Following go-svc pattern for consistent status strings.
func ServiceStatus(name string) (string, error) {
	exists, _ := ServiceExists(name)
	if !exists {
		return "not installed", nil
	}

	cmd := exec.Command("systemctl", "is-active", name)
	output, err := cmd.Output()
	if err != nil {
		return "stopped", nil
	}

	status := strings.TrimSpace(string(output))
	switch status {
	case "active":
		return "running", nil
	case "inactive":
		return "stopped", nil
	case "activating":
		return "starting", nil
	case "deactivating":
		return "stopping", nil
	case "failed":
		return "failed", nil
	default:
		return status, nil
	}
}

// StartService starts the service.
// Returns nil if the service is already running.
func StartService(name string) error {
	// Check if service exists
	exists, err := ServiceExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotInstalled
	}

	// Check if already running
	running, _ := IsServiceRunning(name)
	if running {
		return nil
	}

	// Start the service
	cmd := exec.Command("systemctl", "start", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	// Wait for running state (up to 30 seconds)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		running, _ := IsServiceRunning(name)
		if running {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for service to start")
}

// StopService stops the service.
// Returns nil if the service is already stopped or doesn't exist.
func StopService(name string) error {
	// Check if service exists
	exists, _ := ServiceExists(name)
	if !exists {
		return nil
	}

	// Check if already stopped
	running, _ := IsServiceRunning(name)
	if !running {
		return nil
	}

	// Stop the service
	cmd := exec.Command("systemctl", "stop", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	// Wait for stopped state (up to 30 seconds)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		running, _ := IsServiceRunning(name)
		if !running {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for service to stop")
}

// InstallService installs a new systemd service.
func InstallService(name, displayName, exePath, args string) error {
	return InstallServiceWithConfig(ServiceConfig{
		Name:        name,
		DisplayName: displayName,
		Executable:  exePath,
		Args:        args,
	})
}

// InstallServiceWithConfig installs a systemd service with full configuration.
// Following go-svc patterns: requires root, runs daemon-reload and enable.
func InstallServiceWithConfig(cfg ServiceConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if cfg.Executable == "" {
		return fmt.Errorf("executable path is required")
	}

	// Check privileges
	if err := checkPrivileges(); err != nil {
		return err
	}

	// Check if already installed
	exists, _ := ServiceExists(cfg.Name)
	if exists {
		return ErrAlreadyInstalled
	}

	// Build ExecStart line
	execStart := cfg.Executable
	if cfg.Args != "" {
		execStart = fmt.Sprintf("%s %s", cfg.Executable, cfg.Args)
	}

	// Build description
	description := cfg.Description
	if description == "" {
		description = cfg.DisplayName
	}
	if description == "" {
		description = cfg.Name
	}

	// Generate unit file content
	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("parse unit template: %w", err)
	}

	data := systemdUnitData{
		Description: description,
		ExecStart:   execStart,
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("generate unit file: %w", err)
	}

	// Write unit file
	unitPath := unitFilePath(cfg.Name)
	if err := os.WriteFile(unitPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	// Reload systemd daemon
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reload systemd: %w", err)
	}

	// Enable the service
	cmd = exec.Command("systemctl", "enable", cfg.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	return nil
}

// UninstallService removes a systemd service.
// Returns nil if the service doesn't exist.
func UninstallService(name string) error {
	exists, _ := ServiceExists(name)
	if !exists {
		return nil
	}

	// Check privileges
	if err := checkPrivileges(); err != nil {
		return err
	}

	// Stop the service first if running
	StopService(name)

	// Disable the service
	cmd := exec.Command("systemctl", "disable", name)
	cmd.Run() // Ignore error - service might not be enabled

	// Remove unit file
	unitPath := unitFilePath(name)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	// Reload systemd daemon
	cmd = exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reload systemd: %w", err)
	}

	return nil
}
