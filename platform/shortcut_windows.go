//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Shortcut describes a Windows shortcut (.lnk file).
type Shortcut struct {
	Target      string // Path to the target executable
	Arguments   string // Command-line arguments (optional)
	WorkingDir  string // Working directory (optional, defaults to target's directory)
	Description string // Tooltip description (optional)
	IconPath    string // Path to icon file (optional, defaults to target)
	IconIndex   int    // Icon index within the icon file (optional)
}

// CreateShortcut creates a Windows shortcut (.lnk file) at the specified path.
func CreateShortcut(lnkPath string, s Shortcut) error {
	// Verify target exists
	if _, err := os.Stat(s.Target); err != nil {
		return fmt.Errorf("target not found: %s", s.Target)
	}

	// Lock OS thread for COM operations - COM is thread-bound
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok {
			code := oleErr.Code()
			if code != 0 && code != 1 { // S_OK=0, S_FALSE=1
				return fmt.Errorf("COM initialization failed: %s", oleErrorString(err))
			}
		}
	}
	defer ole.CoUninitialize()

	return createShortcutInternal(lnkPath, s)
}

// DeleteShortcut removes a shortcut file.
func DeleteShortcut(lnkPath string) error {
	err := os.Remove(lnkPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CreateDesktopShortcut creates a shortcut on the public (all users) desktop.
func CreateDesktopShortcut(name string, s Shortcut) error {
	desktop, err := DesktopPath()
	if err != nil {
		return fmt.Errorf("get desktop path: %w", err)
	}
	lnkPath := filepath.Join(desktop, name+".lnk")
	return CreateShortcut(lnkPath, s)
}

// CreateUserDesktopShortcut creates a shortcut on the current user's desktop.
func CreateUserDesktopShortcut(name string, s Shortcut) error {
	desktop, err := UserDesktopPath()
	if err != nil {
		return fmt.Errorf("get user desktop path: %w", err)
	}
	lnkPath := filepath.Join(desktop, name+".lnk")
	return CreateShortcut(lnkPath, s)
}

// CreateStartMenuShortcut creates a shortcut in the public Start Menu.
// The folder parameter specifies a subfolder (e.g., company name). Use "" for the root.
func CreateStartMenuShortcut(folder, name string, s Shortcut) error {
	startMenu, err := StartMenuPath()
	if err != nil {
		return fmt.Errorf("get start menu path: %w", err)
	}
	var lnkPath string
	if folder != "" {
		lnkPath = filepath.Join(startMenu, folder, name+".lnk")
	} else {
		lnkPath = filepath.Join(startMenu, name+".lnk")
	}
	return CreateShortcut(lnkPath, s)
}

// CreateUserStartMenuShortcut creates a shortcut in the current user's Start Menu.
// The folder parameter specifies a subfolder (e.g., company name). Use "" for the root.
func CreateUserStartMenuShortcut(folder, name string, s Shortcut) error {
	startMenu, err := UserStartMenuPath()
	if err != nil {
		return fmt.Errorf("get user start menu path: %w", err)
	}
	var lnkPath string
	if folder != "" {
		lnkPath = filepath.Join(startMenu, folder, name+".lnk")
	} else {
		lnkPath = filepath.Join(startMenu, name+".lnk")
	}
	return CreateShortcut(lnkPath, s)
}

// DeleteDesktopShortcut removes a shortcut from the public desktop.
func DeleteDesktopShortcut(name string) error {
	desktop, err := DesktopPath()
	if err != nil {
		return err
	}
	return DeleteShortcut(filepath.Join(desktop, name+".lnk"))
}

// DeleteUserDesktopShortcut removes a shortcut from the current user's desktop.
func DeleteUserDesktopShortcut(name string) error {
	desktop, err := UserDesktopPath()
	if err != nil {
		return err
	}
	return DeleteShortcut(filepath.Join(desktop, name+".lnk"))
}

// DeleteStartMenuShortcut removes a shortcut from the public Start Menu.
// Also removes the folder if it becomes empty.
func DeleteStartMenuShortcut(folder, name string) error {
	startMenu, err := StartMenuPath()
	if err != nil {
		return err
	}
	var lnkPath string
	if folder != "" {
		lnkPath = filepath.Join(startMenu, folder, name+".lnk")
	} else {
		lnkPath = filepath.Join(startMenu, name+".lnk")
	}
	if err := DeleteShortcut(lnkPath); err != nil {
		return err
	}
	// Try to remove folder if empty
	if folder != "" {
		_ = os.Remove(filepath.Join(startMenu, folder))
	}
	return nil
}

// DeleteUserStartMenuShortcut removes a shortcut from the current user's Start Menu.
// Also removes the folder if it becomes empty.
func DeleteUserStartMenuShortcut(folder, name string) error {
	startMenu, err := UserStartMenuPath()
	if err != nil {
		return err
	}
	var lnkPath string
	if folder != "" {
		lnkPath = filepath.Join(startMenu, folder, name+".lnk")
	} else {
		lnkPath = filepath.Join(startMenu, name+".lnk")
	}
	if err := DeleteShortcut(lnkPath); err != nil {
		return err
	}
	// Try to remove folder if empty
	if folder != "" {
		_ = os.Remove(filepath.Join(startMenu, folder))
	}
	return nil
}

// createShortcutInternal creates a shortcut using COM.
// Assumes COM is already initialized.
func createShortcutInternal(lnkPath string, s Shortcut) error {
	// Ensure parent directory exists
	parentDir := filepath.Dir(lnkPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", parentDir, err)
	}

	// Remove existing shortcut if present
	if _, err := os.Stat(lnkPath); err == nil {
		_ = os.Remove(lnkPath)
	}

	oleShellObject, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("cannot create WScript.Shell object: %s", oleErrorString(err))
	}
	defer oleShellObject.Release()

	wshell, err := oleShellObject.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("cannot get shell interface: %s", oleErrorString(err))
	}
	defer wshell.Release()

	shortcutVariant, err := oleutil.CallMethod(wshell, "CreateShortcut", lnkPath)
	if err != nil {
		return fmt.Errorf("cannot create shortcut object: %s", oleErrorString(err))
	}
	shortcut := shortcutVariant.ToIDispatch()
	defer shortcut.Release()

	// Set target path
	if _, err := oleutil.PutProperty(shortcut, "TargetPath", s.Target); err != nil {
		return fmt.Errorf("cannot set target path: %s", oleErrorString(err))
	}

	// Set arguments if provided
	if s.Arguments != "" {
		if _, err := oleutil.PutProperty(shortcut, "Arguments", s.Arguments); err != nil {
			return fmt.Errorf("cannot set arguments: %s", oleErrorString(err))
		}
	}

	// Set working directory
	workingDir := s.WorkingDir
	if workingDir == "" {
		workingDir = filepath.Dir(s.Target)
	}
	if _, err := oleutil.PutProperty(shortcut, "WorkingDirectory", workingDir); err != nil {
		return fmt.Errorf("cannot set working directory: %s", oleErrorString(err))
	}

	// Set description if provided
	if s.Description != "" {
		if _, err := oleutil.PutProperty(shortcut, "Description", s.Description); err != nil {
			return fmt.Errorf("cannot set description: %s", oleErrorString(err))
		}
	}

	// Set icon
	iconPath := s.IconPath
	if iconPath == "" {
		iconPath = s.Target
	}
	iconLocation := fmt.Sprintf("%s,%d", iconPath, s.IconIndex)
	if _, err := oleutil.PutProperty(shortcut, "IconLocation", iconLocation); err != nil {
		return fmt.Errorf("cannot set icon: %s", oleErrorString(err))
	}

	// Save
	if _, err := oleutil.CallMethod(shortcut, "Save"); err != nil {
		return fmt.Errorf("cannot save shortcut: %s", oleErrorString(err))
	}

	return nil
}

// oleErrorString extracts a meaningful error message from OLE errors.
func oleErrorString(err error) string {
	if err == nil {
		return "unknown error"
	}
	if oleErr, ok := err.(*ole.OleError); ok {
		return fmt.Sprintf("%s (HRESULT: 0x%08X)", oleErr.Error(), uint32(oleErr.Code()))
	}
	return err.Error()
}
