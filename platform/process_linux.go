//go:build linux

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// FindProcessesByName returns PIDs of all processes matching the given executable name.
// The comparison is case-sensitive (unlike Windows).
func FindProcessesByName(exeName string) []uint32 {
	var pids []uint32

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		// Only process numeric directories (PIDs)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Read the process name from /proc/<pid>/comm
		commPath := fmt.Sprintf("/proc/%d/comm", pid)
		comm, err := os.ReadFile(commPath)
		if err != nil {
			continue // Process may have exited
		}

		// comm contains the executable name (max 15 chars) with trailing newline
		name := strings.TrimSpace(string(comm))
		if name == exeName {
			pids = append(pids, uint32(pid))
		}
	}

	return pids
}

// IsProcessRunning checks if any process with the given executable name is running.
func IsProcessRunning(exeName string) bool {
	pids := FindProcessesByName(exeName)
	return len(pids) > 0
}

// KillProcess terminates a process by PID using SIGTERM.
func KillProcess(pid uint32) error {
	err := syscall.Kill(int(pid), syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("kill process %d: %w", pid, err)
	}
	return nil
}

// KillProcessByName terminates all processes with the given executable name.
// Returns nil if no processes are found. Returns an error if any termination fails.
func KillProcessByName(exeName string) error {
	pids := FindProcessesByName(exeName)
	if len(pids) == 0 {
		return nil
	}

	var lastErr error
	for _, pid := range pids {
		if err := KillProcess(pid); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
