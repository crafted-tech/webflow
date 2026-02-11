//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

var (
	procGetShellWindow          = user32.NewProc("GetShellWindow")
	advapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTokenW = advapi32.NewProc("CreateProcessWithTokenW")
)

const logonWithProfile = 0x00000001

// COM GUIDs for the IShellDispatch2 technique.
var (
	clsidShellWindows      = ole.NewGUID("9BA05972-F6A8-11CF-A442-00A0C90A8F39")
	sidSTopLevelBrowser    = ole.NewGUID("4C96BE40-915C-11CF-99D3-00AA004AE837")
	iidIShellBrowser       = ole.NewGUID("000214E2-0000-0000-C000-000000000046")
	iidIServiceProvider    = ole.NewGUID("6D5140C1-7436-11CE-8034-00AA006009FA")
	iidIShellFolderViewDual = ole.NewGUID("E7A1AF80-4D96-11CF-960C-0080C7F4EE85")
)

// LaunchDeElevated launches an executable at the desktop user's normal
// (non-elevated) privilege level. This is intended for use from a
// UAC-elevated installer process that needs to start an application
// without passing on its elevated token.
//
// The function tries three strategies in order:
//  1. IShellDispatch2::ShellExecute via COM — asks the running Explorer
//     shell to launch the process (cross-process COM). The child inherits
//     Explorer's non-elevated token and runs outside the caller's job.
//  2. Scheduled task — creates a one-shot task via schtasks.exe with
//     /rl limited. Task Scheduler runs the process in its own context,
//     outside the caller's job object and at non-elevated privilege.
//  3. Shell-token approach — borrows Explorer's token via
//     CreateProcessWithTokenW. De-elevates but does NOT escape the
//     caller's job object.
//
// If the current process is not elevated, launches directly.
//
// Returns the PID of the launched process (0 when the process is created
// asynchronously by a helper like Task Scheduler or Explorer).
func LaunchDeElevated(exePath string) (uint32, error) {
	if !IsElevated() {
		return launchDirect(exePath)
	}

	// Primary: COM to Explorer (direct, no Task Scheduler dependency).
	if err := shellExecuteViaExplorer(exePath); err == nil {
		return 0, nil
	}

	// Fallback: scheduled task (works when Explorer isn't available).
	if err := launchViaScheduledTask(exePath); err == nil {
		return 0, nil
	}

	// Last resort: shell-token (de-elevation only, stays in job).
	pid, err := launchWithShellToken(exePath)
	if err != nil {
		return 0, fmt.Errorf("de-elevated launch: all strategies failed: %w", err)
	}
	return pid, nil
}

// launchViaScheduledTask creates and immediately runs a one-shot scheduled
// task to launch the executable, then deletes the task definition.
// Task Scheduler creates the process in the service's own context — outside
// the caller's job object — with the user's limited (non-elevated) token.
func launchViaScheduledTask(exePath string) error {
	taskName := fmt.Sprintf("UnisonLaunch_%d", os.Getpid())

	schtasks := filepath.Join(os.Getenv("WINDIR"), "System32", "schtasks.exe")

	// Create a one-time task. /rl limited = standard user privileges.
	if err := runHidden(schtasks, "/create",
		"/tn", taskName,
		"/tr", `"`+exePath+`"`,
		"/sc", "once",
		"/st", "00:00",
		"/f",
		"/rl", "limited",
	); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Grant any process the right to set foreground window. Without this,
	// the app launched by Task Scheduler would appear behind other windows.
	AllowSetForegroundForAnyProcess()

	// Run immediately.
	if err := runHidden(schtasks, "/run", "/tn", taskName); err != nil {
		runHidden(schtasks, "/delete", "/tn", taskName, "/f")
		return fmt.Errorf("run task: %w", err)
	}

	// Brief wait for Task Scheduler to start the process, then delete.
	time.Sleep(1 * time.Second)
	runHidden(schtasks, "/delete", "/tn", taskName, "/f")

	return nil
}

// runHidden runs a command with its console window hidden.
func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", filepath.Base(name), args[:2], err)
	}
	return nil
}

// shellExecuteViaExplorer uses the IShellDispatch2::ShellExecute COM
// technique to ask the running Explorer shell to launch an executable.
// The process is created by Explorer, so it runs at normal privilege
// level and outside the caller's job object.
//
// Reference: Raymond Chen, "How can I launch an unelevated process from
// my elevated process, and vice versa?"
// https://devblogs.microsoft.com/oldnewthing/20131118-00/?p=2643
func shellExecuteViaExplorer(exePath string) (err error) {
	// Recover from access violations in raw COM vtable calls.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("COM panic: %v", r)
		}
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE (1) means already initialized — that's fine.
		if oleErr, ok := err.(*ole.OleError); !ok || (oleErr.Code() != 0 && oleErr.Code() != 1) {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	// Step 1: Get the ShellWindows collection (cross-process COM to Explorer).
	unk, err := ole.CreateInstance(clsidShellWindows, ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("create ShellWindows: %w", err)
	}
	defer unk.Release()

	swDisp, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch on ShellWindows: %w", err)
	}
	defer swDisp.Release()

	// Step 2: FindWindowSW to locate the desktop shell window.
	// IShellWindows vtable (after IDispatch's 7 methods):
	//   7=get_Count, 8=Item, 9=_NewEnum, 10=Register, 11=RegisterPending,
	//   12=Revoke, 13=OnNavigate, 14=OnActivated, 15=FindWindowSW
	const swcDesktop = 8
	const swfoNeedDispatch = 1
	var hwnd int32
	var desktopDisp *ole.IDispatch
	vEmpty1 := ole.VARIANT{}
	vEmpty2 := ole.VARIANT{}

	hr, _, _ := syscall.SyscallN(
		comMethod(uintptr(unsafe.Pointer(swDisp)), 15),
		uintptr(unsafe.Pointer(swDisp)),
		uintptr(unsafe.Pointer(&vEmpty1)),     // pvarLoc
		uintptr(unsafe.Pointer(&vEmpty2)),     // pvarLocRoot
		uintptr(swcDesktop),                   // swClass
		uintptr(unsafe.Pointer(&hwnd)),        // phwnd
		uintptr(swfoNeedDispatch),             // swfwOptions
		uintptr(unsafe.Pointer(&desktopDisp)), // ppdispOut
	)
	if hr != 0 || desktopDisp == nil {
		return fmt.Errorf("FindWindowSW: HRESULT 0x%08X", uint32(hr))
	}
	defer desktopDisp.Release()

	// Step 3: Get IServiceProvider from the desktop dispatch.
	spUnk, err := desktopDisp.QueryInterface(iidIServiceProvider)
	if err != nil {
		return fmt.Errorf("query IServiceProvider: %w", err)
	}
	defer spUnk.Release()

	// IServiceProvider::QueryService is vtable index 3
	// (0=QueryInterface, 1=AddRef, 2=Release, 3=QueryService)
	var shellBrowser uintptr
	hr, _, _ = syscall.SyscallN(
		comMethod(uintptr(unsafe.Pointer(spUnk)), 3),
		uintptr(unsafe.Pointer(spUnk)),
		uintptr(unsafe.Pointer(sidSTopLevelBrowser)),
		uintptr(unsafe.Pointer(iidIShellBrowser)),
		uintptr(unsafe.Pointer(&shellBrowser)),
	)
	if hr != 0 || shellBrowser == 0 {
		return fmt.Errorf("QueryService(STopLevelBrowser): HRESULT 0x%08X", uint32(hr))
	}
	defer comRelease(shellBrowser)

	// Step 4: IShellBrowser::QueryActiveShellView (vtable index 15).
	var shellView uintptr
	hr, _, _ = syscall.SyscallN(
		comMethod(shellBrowser, 15),
		shellBrowser,
		uintptr(unsafe.Pointer(&shellView)),
	)
	if hr != 0 || shellView == 0 {
		return fmt.Errorf("QueryActiveShellView: HRESULT 0x%08X", uint32(hr))
	}
	defer comRelease(shellView)

	// Step 5: IShellView::GetItemObject(SVGIO_BACKGROUND, IID_IDispatch).
	const svgioBackground = 0
	var bgPtr uintptr
	hr, _, _ = syscall.SyscallN(
		comMethod(shellView, 15),
		shellView,
		uintptr(svgioBackground),
		uintptr(unsafe.Pointer(ole.IID_IDispatch)),
		uintptr(unsafe.Pointer(&bgPtr)),
	)
	if hr != 0 || bgPtr == 0 {
		return fmt.Errorf("GetItemObject: HRESULT 0x%08X", uint32(hr))
	}
	bgDisp := (*ole.IDispatch)(unsafe.Pointer(bgPtr))
	defer bgDisp.Release()

	// Step 6: Get IShellFolderViewDual, then its Application property.
	sfvdUnk, err := bgDisp.QueryInterface(iidIShellFolderViewDual)
	if err != nil {
		return fmt.Errorf("query IShellFolderViewDual: %w", err)
	}
	sfvd := (*ole.IDispatch)(unsafe.Pointer(sfvdUnk))
	defer sfvd.Release()

	appVar, err := oleutil.GetProperty(sfvd, "Application")
	if err != nil {
		return fmt.Errorf("get Application property: %w", err)
	}
	shellApp := appVar.ToIDispatch()
	defer shellApp.Release()

	// Step 7: IShellDispatch2::ShellExecute — runs in Explorer's process.
	dir := filepath.Dir(exePath)
	_, err = oleutil.CallMethod(shellApp, "ShellExecute",
		exePath, // sFile
		"",      // vArguments
		dir,     // vDirectory
		"open",  // vOperation
		1,       // vShow (SW_SHOWNORMAL)
	)
	if err != nil {
		return fmt.Errorf("ShellExecute: %w", err)
	}
	return nil
}

// launchWithShellToken borrows the Explorer shell's non-elevated token
// and uses it to create a new process at normal privilege level.
// This is a fallback for when the COM approach is unavailable.
func launchWithShellToken(exePath string) (uint32, error) {
	shellWnd, _, _ := procGetShellWindow.Call()
	if shellWnd == 0 {
		return 0, fmt.Errorf("no shell window found (explorer.exe not running?)")
	}

	var shellPID uint32
	procGetWindowThreadProcessId.Call(shellWnd, uintptr(unsafe.Pointer(&shellPID)))
	if shellPID == 0 {
		return 0, fmt.Errorf("cannot determine shell process ID")
	}

	shellProcess, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION, false, shellPID)
	if err != nil {
		return 0, fmt.Errorf("open shell process (PID %d): %w", shellPID, err)
	}
	defer windows.CloseHandle(shellProcess)

	var shellToken windows.Token
	err = windows.OpenProcessToken(shellProcess, windows.TOKEN_DUPLICATE, &shellToken)
	if err != nil {
		return 0, fmt.Errorf("open shell process token: %w", err)
	}
	defer shellToken.Close()

	var primaryToken windows.Token
	err = windows.DuplicateTokenEx(
		shellToken,
		windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_ASSIGN_PRIMARY|
			windows.TOKEN_ADJUST_DEFAULT|windows.TOKEN_ADJUST_SESSIONID,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&primaryToken,
	)
	if err != nil {
		return 0, fmt.Errorf("duplicate shell token: %w", err)
	}
	defer primaryToken.Close()

	var envBlock *uint16
	if err := windows.CreateEnvironmentBlock(&envBlock, primaryToken, false); err != nil {
		return 0, fmt.Errorf("create environment block: %w", err)
	}
	defer windows.DestroyEnvironmentBlock(envBlock)

	si := windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})),
	}

	exePathUTF16, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return 0, fmt.Errorf("invalid path: %w", err)
	}

	workDir, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return 0, fmt.Errorf("invalid work dir: %w", err)
	}

	var pi windows.ProcessInformation
	r1, _, e1 := procCreateProcessWithTokenW.Call(
		uintptr(primaryToken),
		logonWithProfile,
		uintptr(unsafe.Pointer(exePathUTF16)),
		0,
		windows.CREATE_UNICODE_ENVIRONMENT,
		uintptr(unsafe.Pointer(envBlock)),
		uintptr(unsafe.Pointer(workDir)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("CreateProcessWithTokenW: %w", e1)
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return pi.ProcessId, nil
}

// launchDirect starts a process using CreateProcess (no token manipulation).
func launchDirect(exePath string) (uint32, error) {
	exePathUTF16, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return 0, fmt.Errorf("invalid path: %w", err)
	}

	workDir, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return 0, fmt.Errorf("invalid work dir: %w", err)
	}

	si := windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})),
	}
	var pi windows.ProcessInformation

	err = windows.CreateProcess(
		exePathUTF16,
		nil,     // command line
		nil,     // process security attributes
		nil,     // thread security attributes
		false,   // inherit handles
		0,       // creation flags
		nil,     // environment (inherit)
		workDir, // current directory
		&si,
		&pi,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateProcess: %w", err)
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return pi.ProcessId, nil
}

// comMethod returns the function pointer at the given vtable index.
func comMethod(iface uintptr, index int) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(iface))
	return *(*uintptr)(unsafe.Pointer(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

// comRelease calls IUnknown::Release on a raw COM interface pointer.
func comRelease(iface uintptr) {
	if iface != 0 {
		syscall.SyscallN(comMethod(iface, 2), iface)
	}
}
