//go:build darwin

package platform

/*
#include <libproc.h>
#include <sys/sysctl.h>
#include <stdlib.h>
#include <string.h>

// Get all PIDs on the system
int get_all_pids(pid_t **pids) {
    int count = proc_listallpids(NULL, 0);
    if (count <= 0) {
        return 0;
    }

    *pids = (pid_t *)malloc(count * sizeof(pid_t));
    if (*pids == NULL) {
        return 0;
    }

    count = proc_listallpids(*pids, count * sizeof(pid_t));
    return count;
}

// Get process name by PID
int get_proc_name(pid_t pid, char *name, int namesize) {
    return proc_name(pid, name, namesize);
}
*/
import "C"

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// FindProcessesByName returns PIDs of all processes matching the given executable name.
// The comparison is case-insensitive.
func FindProcessesByName(exeName string) []uint32 {
	var pids []uint32
	var pidArray *C.pid_t

	count := C.get_all_pids(&pidArray)
	if count <= 0 || pidArray == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(pidArray))

	// Convert C array to Go slice
	pidSlice := unsafe.Slice(pidArray, int(count))

	nameBuf := make([]byte, 256)
	for i := 0; i < int(count); i++ {
		pid := pidSlice[i]
		if pid <= 0 {
			continue
		}

		// Get process name
		n := C.get_proc_name(pid, (*C.char)(unsafe.Pointer(&nameBuf[0])), C.int(len(nameBuf)))
		if n <= 0 {
			continue
		}

		procName := string(nameBuf[:n])
		// Case-insensitive comparison
		if strings.EqualFold(procName, exeName) {
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
