//go:build darwin

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

// ServiceConfig holds parameters for installing a macOS service (launchd).
// This follows patterns from github.com/adnsv/go-svc.
type ServiceConfig struct {
	Name        string // Service identifier (required) - should be reverse-DNS like com.company.service
	DisplayName string // Human-readable name (used in Label comment)
	Description string // Service description (comment in plist)
	Executable  string // Full path to the executable (required)
	Args        string // Command-line arguments passed at startup
	StartType   uint32 // Ignored on macOS
}

// launchdPlistTemplate is the template for generating launchd plist files.
// Following go-svc patterns with KeepAlive and throttle configuration.
const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
{{range .Args}}        <string>{{.}}</string>
{{end}}    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>5</integer>
    <key>StandardOutPath</key>
    <string>/var/log/{{.Label}}.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/{{.Label}}.err</string>
</dict>
</plist>
`

type launchdPlistData struct {
	Label string
	Args  []string
}

// plistFilePath returns the path to the launchd plist file for a service.
func plistFilePath(name string) string {
	return filepath.Join("/Library/LaunchDaemons", name+".plist")
}

// isRoot checks if running with root privileges.
func isRoot() bool {
	return os.Geteuid() == 0
}

// runWithPrivileges runs a command, prepending sudo if not root.
// Following go-svc pattern for privilege handling.
func runWithPrivileges(name string, args ...string) error {
	if isRoot() {
		cmd := exec.Command(name, args...)
		return cmd.Run()
	}
	// Prepend sudo
	allArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", allArgs...)
	return cmd.Run()
}

// ServiceExists returns true if a launchd service with the given name exists.
func ServiceExists(name string) (bool, error) {
	plistPath := plistFilePath(name)
	_, err := os.Stat(plistPath)
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
	cmd := exec.Command("launchctl", "list", name)
	err := cmd.Run()
	return err == nil, nil
}

// ServiceStatus returns a string describing the current state of a service.
// Following go-svc pattern for consistent status strings.
func ServiceStatus(name string) (string, error) {
	exists, _ := ServiceExists(name)
	if !exists {
		return "not installed", nil
	}

	running, _ := IsServiceRunning(name)
	if running {
		return "running", nil
	}
	return "stopped", nil
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

	// Load the service (launchctl load starts it)
	plistPath := plistFilePath(name)
	if err := runWithPrivileges("launchctl", "load", plistPath); err != nil {
		return fmt.Errorf("load service: %w", err)
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

	// Unload the service (launchctl unload stops it)
	plistPath := plistFilePath(name)
	if err := runWithPrivileges("launchctl", "unload", plistPath); err != nil {
		return fmt.Errorf("unload service: %w", err)
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

// InstallService installs a new launchd service.
func InstallService(name, displayName, exePath, args string) error {
	return InstallServiceWithConfig(ServiceConfig{
		Name:        name,
		DisplayName: displayName,
		Executable:  exePath,
		Args:        args,
	})
}

// InstallServiceWithConfig installs a launchd service with full configuration.
// Following go-svc patterns with KeepAlive and ThrottleInterval.
func InstallServiceWithConfig(cfg ServiceConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if cfg.Executable == "" {
		return fmt.Errorf("executable path is required")
	}

	// Check if already installed
	exists, _ := ServiceExists(cfg.Name)
	if exists {
		return ErrAlreadyInstalled
	}

	// Build program arguments
	args := []string{cfg.Executable}
	if cfg.Args != "" {
		// Simple split by spaces (doesn't handle quoted strings)
		args = append(args, strings.Fields(cfg.Args)...)
	}

	// Generate plist content
	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}

	data := launchdPlistData{
		Label: cfg.Name,
		Args:  args,
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("generate plist file: %w", err)
	}

	// Write plist file (requires root)
	plistPath := plistFilePath(cfg.Name)
	if isRoot() {
		if err := os.WriteFile(plistPath, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("write plist file: %w", err)
		}
	} else {
		// Use sudo tee to write file
		cmd := exec.Command("sudo", "tee", plistPath)
		cmd.Stdin = strings.NewReader(content.String())
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("write plist file: %w", err)
		}
	}

	return nil
}

// UninstallService removes a launchd service.
// Returns nil if the service doesn't exist.
func UninstallService(name string) error {
	exists, _ := ServiceExists(name)
	if !exists {
		return nil
	}

	// Stop the service first if running
	StopService(name)

	// Remove plist file
	plistPath := plistFilePath(name)
	if isRoot() {
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove plist file: %w", err)
		}
	} else {
		if err := runWithPrivileges("rm", "-f", plistPath); err != nil {
			return fmt.Errorf("remove plist file: %w", err)
		}
	}

	return nil
}
