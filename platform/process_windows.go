//go:build windows

package platform

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = modkernel32.NewProc("Process32FirstW")
	procProcess32NextW           = modkernel32.NewProc("Process32NextW")
)

const (
	th32csSnapProcess = 0x00000002
	maxPath           = 260
)

type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [maxPath]uint16
}

// FindProcessesByName returns PIDs of all processes matching the given executable name.
// The comparison is case-insensitive.
func FindProcessesByName(exeName string) []uint32 {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return nil
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil
	}

	var pids []uint32
	exeNameLower := strings.ToLower(exeName)

	for {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if strings.ToLower(name) == exeNameLower {
			pids = append(pids, entry.ProcessID)
		}

		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return pids
}

// IsProcessRunning checks if any process with the given executable name is running.
func IsProcessRunning(exeName string) bool {
	pids := FindProcessesByName(exeName)
	return len(pids) > 0
}

// KillProcess terminates a process by PID.
func KillProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("terminate process %d: %w", pid, err)
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
