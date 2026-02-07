//go:build !windows

package platform

import (
	"os/exec"
)

// LaunchDeElevated on non-Windows platforms simply starts the process
// directly, since UAC elevation is a Windows-specific concept.
func LaunchDeElevated(exePath string) (uint32, error) {
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := uint32(cmd.Process.Pid)
	cmd.Process.Release()
	return pid, nil
}
