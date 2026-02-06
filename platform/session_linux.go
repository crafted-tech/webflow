//go:build linux

package platform

import (
	"os/exec"
)

// LaunchAsSessionUser on Linux simply starts the process directly,
// since the SYSTEM service context issue is Windows-specific.
func LaunchAsSessionUser(exePath string) (uint32, error) {
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := uint32(cmd.Process.Pid)
	cmd.Process.Release()
	return pid, nil
}
