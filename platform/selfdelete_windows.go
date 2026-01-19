//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// ScheduleSelfDelete arranges for the current executable to be deleted
// after the process exits. Uses a detached cmd.exe helper that
// repeatedly attempts to delete the file until it succeeds.
//
// This is useful for uninstallers that need to delete themselves.
func ScheduleSelfDelete() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	absPath, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	return ScheduleFileDelete(absPath)
}

// ScheduleFileDelete arranges for the specified file to be deleted
// after it is no longer in use. Uses a detached cmd.exe helper that
// repeatedly attempts to delete the file until it succeeds.
func ScheduleFileDelete(filePath string) error {
	// Use a detached cmd.exe helper that repeatedly attempts to delete the
	// file until it succeeds. This is a standard Windows pattern:
	// the helper waits for the file to be released.
	script := fmt.Sprintf(
		`:loop & del /f /q "%[1]s" 2>nul & if exist "%[1]s" ( timeout /t 1 /nobreak >nul & goto loop )`,
		filePath,
	)

	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start delete helper: %w", err)
	}

	return nil
}
