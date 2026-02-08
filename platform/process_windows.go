//go:build windows

package platform

import (
	"fmt"
	"strings"
	"sync"
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

var debugPrivilegeOnce sync.Once

// enableSeDebugPrivilege enables SeDebugPrivilege on the current process token,
// allowing OpenProcess to access processes owned by other users. This is a
// best-effort operation — it silently fails for non-admin callers.
func enableSeDebugPrivilege() {
	debugPrivilegeOnce.Do(func() {
		var token windows.Token
		err := windows.OpenProcessToken(windows.CurrentProcess(),
			windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
		if err != nil {
			return
		}
		defer token.Close()

		privName, _ := windows.UTF16PtrFromString("SeDebugPrivilege")
		var luid windows.LUID
		if windows.LookupPrivilegeValue(nil, privName, &luid) != nil {
			return
		}

		tp := windows.Tokenprivileges{
			PrivilegeCount: 1,
			Privileges: [1]windows.LUIDAndAttributes{
				{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED},
			},
		}
		windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil)
	})
}

// KillProcess terminates a process by PID and waits for it to exit.
// Enables SeDebugPrivilege so that processes owned by other users can be
// terminated (requires administrator).
func KillProcess(pid uint32) error {
	enableSeDebugPrivilege()

	handle, err := windows.OpenProcess(
		windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("terminate process %d: %w", pid, err)
	}

	// Wait for the process to fully exit so file handles are released.
	event, _ := windows.WaitForSingleObject(handle, 5_000) // 5s timeout
	if event == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("process %d did not exit within timeout", pid)
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
