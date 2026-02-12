//go:build windows

package platform

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Two-phase self-delete approach based on Inno Setup.
// Phase 1: Original process creates temp copy and spawns Phase 2
// Phase 2: Temp copy performs uninstall and deletes original

const (
	// Command-line flags for phase detection
	secondPhaseFlag    = "/SECONDPHASE="
	firstPhaseWndFlag  = "/FIRSTPHASEWND="
	tempDirPrefix      = "uninstall-"
	tempDirSuffix      = ".tmp"
	tempExeName        = "_unins.tmp"
	doneSignalFileName = "_unins-done.tmp"

	// Window message for signaling completion
	wmUser = 0x0400
)

// SelfDeleteConfig holds configuration for two-phase self-delete.
type SelfDeleteConfig struct {
	OriginalExePath string       // Path to original uninstaller (Phase 2 only)
	FirstPhaseWnd   windows.HWND // Window handle for signaling (Phase 2 only)
}

// IsSecondPhase checks if we're running as Phase 2 of self-delete.
func IsSecondPhase() bool {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(strings.ToUpper(arg), secondPhaseFlag) {
			return true
		}
	}
	return false
}

// GetSecondPhaseConfig parses command-line for Phase 2 configuration.
// Returns nil if not in Phase 2.
func GetSecondPhaseConfig() (*SelfDeleteConfig, error) {
	config := &SelfDeleteConfig{}

	for _, arg := range os.Args[1:] {
		upper := strings.ToUpper(arg)
		if strings.HasPrefix(upper, secondPhaseFlag) {
			// Extract original path (preserve original case)
			idx := strings.Index(arg, "=")
			if idx >= 0 {
				config.OriginalExePath = arg[idx+1:]
			}
		} else if strings.HasPrefix(upper, firstPhaseWndFlag) {
			idx := strings.Index(arg, "=")
			if idx >= 0 {
				hwnd, err := strconv.ParseUint(arg[idx+1:], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse window handle: %w", err)
				}
				config.FirstPhaseWnd = windows.HWND(hwnd)
			}
		}
	}

	if config.OriginalExePath == "" {
		return nil, fmt.Errorf("missing %s argument", secondPhaseFlag)
	}

	return config, nil
}

// FilterSecondPhaseArgs returns command-line args with Phase 2 flags removed.
// Use this to get the original args to forward to Phase 2.
func FilterSecondPhaseArgs() []string {
	var filtered []string
	for _, arg := range os.Args[1:] {
		upper := strings.ToUpper(arg)
		if !strings.HasPrefix(upper, secondPhaseFlag) &&
			!strings.HasPrefix(upper, firstPhaseWndFlag) {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

// RunFirstPhase creates a temp copy and spawns Phase 2.
// This blocks until Phase 2 signals completion or crashes.
// If Phase 2 crashes without signaling, Phase 1 cleans up the temp directory.
func RunFirstPhase() error {
	// Generate unique temp directory
	tempDir, err := createUniqueTempDir()
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	// Copy self to temp directory
	selfPath, err := os.Executable()
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("get executable path: %w", err)
	}

	tempExe := filepath.Join(tempDir, tempExeName)
	if err := copyFile(selfPath, tempExe); err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("copy to temp: %w", err)
	}

	// Remove read-only attribute if present (like Inno Setup does)
	windows.SetFileAttributes(windows.StringToUTF16Ptr(tempExe), windows.FILE_ATTRIBUTE_NORMAL)

	// Create signal window
	wnd, cleanup, err := createSignalWindow()
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("create signal window: %w", err)
	}
	defer cleanup()

	// Build Phase 2 command line
	args := []string{
		fmt.Sprintf("%s%s", secondPhaseFlag, selfPath),
		fmt.Sprintf("%s%d", firstPhaseWndFlag, uintptr(wnd)),
	}
	// Forward original args (excluding any phase flags)
	args = append(args, FilterSecondPhaseArgs()...)

	// Spawn Phase 2. Use CREATE_BREAKAWAY_FROM_JOB to detach from the
	// caller's job object. Without this, the child process gets killed when
	// Phase 1 exits if the job has JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE set
	// (common when launched from Windows Terminal or other process managers).
	// If the job doesn't allow breakaway, fall back to normal CreateProcess.
	// Phase 2 uses NTFS ADS POSIX delete to remove the original exe BEFORE
	// signaling Phase 1, so being killed by the job is harmless.
	cmd := exec.Command(tempExe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB,
	}
	if err := cmd.Start(); err != nil {
		// Retry without CREATE_BREAKAWAY_FROM_JOB
		cmd = exec.Command(tempExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("start phase 2: %w", err)
		}
	}

	// Open Phase 2 process handle with SYNCHRONIZE access for waiting
	processHandle, _ := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	shouldDeleteTempDir := false

	// Wait for completion signal from Phase 2 OR Phase 2 crash
	signalRcvd := waitForSignalOrProcessDeath(wnd, processHandle)

	if processHandle != 0 {
		windows.CloseHandle(processHandle)
	}

	// If Phase 2 died without signaling, we need to clean up
	if !signalRcvd {
		shouldDeleteTempDir = true
	}

	if shouldDeleteTempDir {
		DelayDeleteFile(tempExe, 13, 50, 250)
		os.Remove(tempDir)
	}

	// Note: In Inno Setup, the done file is created by Phase 2, not Phase 1.
	// Phase 2 holds the done file open, so cleanup won't happen until Phase 2 exits.
	// Phase 1 just exits after receiving the signal.

	return nil
}

// DeleteRunningExe deletes an executable that may be currently running by
// using the NTFS Alternate Data Stream technique. This works on Windows 10
// 1709+ (build 16299) with NTFS filesystems.
//
// The technique renames the file's default ::$DATA stream to an alternate
// stream name, then marks the file for POSIX deletion. POSIX semantics
// unlink the directory entry immediately while the actual data persists
// until all handles and mapped image sections are released.
//
// Returns an error if the technique is not supported (non-NTFS, old Windows)
// or if the operation fails. Callers should fall back to rename-based
// deletion when this fails.
func DeleteRunningExe(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	// Step 1: Open with DELETE access for stream rename.
	h, err := windows.CreateFile(pathPtr,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return fmt.Errorf("open for rename: %w", err)
	}

	// Step 2: Rename the default ::$DATA stream to :deadbeef.
	// This moves the executable's mapped image section out of the default
	// stream so the file can be marked for deletion.
	type fileRenameInfo struct {
		ReplaceIfExists uint32
		RootDirectory   uintptr
		FileNameLength  uint32
		FileName        [9]uint16 // ":deadbeef" = 9 UTF-16 code units
	}
	adsNameUTF16, _ := windows.UTF16FromString(":deadbeef")
	var renameInfo fileRenameInfo
	renameInfo.FileNameLength = 9 * 2 // byte count
	copy(renameInfo.FileName[:], adsNameUTF16)

	err = windows.SetFileInformationByHandle(h, windows.FileRenameInfo,
		(*byte)(unsafe.Pointer(&renameInfo)), uint32(unsafe.Sizeof(renameInfo)))
	windows.CloseHandle(h)
	if err != nil {
		return fmt.Errorf("rename stream: %w", err)
	}

	// Step 3: Reopen and mark for POSIX delete.
	// Must reopen because the previous handle was associated with the
	// now-renamed stream; we need a fresh handle to the file entry.
	h, err = windows.CreateFile(pathPtr,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return fmt.Errorf("reopen for delete: %w", err)
	}

	flags := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS)
	err = windows.SetFileInformationByHandle(h, windows.FileDispositionInfoEx,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
	windows.CloseHandle(h)
	if err != nil {
		return fmt.Errorf("set disposition: %w", err)
	}

	return nil
}

// DeleteOriginalUninstaller deletes the original uninstaller EXE with retries.
// Called from Phase 2 after uninstallation is complete.
func DeleteOriginalUninstaller(originalPath string) error {
	// Same parameters as Inno Setup: 13 retries, 50ms first delay, 250ms subsequent
	return DelayDeleteFile(originalPath, 13, 50, 250)
}

// CreateDoneSignalFile creates the done signal file in the temp directory.
// Called from Phase 2 after uninstall work is complete, BEFORE signaling Phase 1.
// The file is held open until Phase 2 exits, preventing cleanup while running.
// Returns the file handle (caller should NOT close it - OS closes on exit).
func CreateDoneSignalFile() windows.Handle {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}

	// Only create if we're the temp copy (_unins.tmp)
	if !strings.EqualFold(filepath.Base(exe), tempExeName) {
		return 0
	}

	doneFile := filepath.Join(filepath.Dir(exe), doneSignalFileName)
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(doneFile),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ, // Allow read sharing but not delete
		nil,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0
	}

	// Intentionally return handle without closing - OS closes on process exit
	return handle
}

// SignalFirstPhaseAndWait signals Phase 1 and waits for it to exit.
// This is the correct approach - we must wait for the Phase 1 process to
// actually terminate before trying to delete its executable.
// Matches Inno Setup's approach in DeleteUninstallDataFiles.
func SignalFirstPhaseAndWait(wnd windows.HWND) {
	if wnd == 0 {
		return
	}

	// Get process ID from window handle (like Inno Setup does)
	var processID uint32
	ret, _, _ := procGetWindowThreadProcessId.Call(uintptr(wnd), uintptr(unsafe.Pointer(&processID)))

	// Open process with SYNCHRONIZE access so we can wait on it
	var processHandle windows.Handle
	if ret != 0 && processID != 0 {
		processHandle, _ = windows.OpenProcess(windows.SYNCHRONIZE, false, processID)
	}

	// Signal Phase 1 to exit using SendNotifyMessage (like Inno Setup)
	// SendNotifyMessage is async for cross-process but guaranteed delivery
	sendNotifyMessage(wnd, wmUser, 0, 0)

	// Wait for Phase 1 process to actually terminate
	if processHandle != 0 {
		windows.WaitForSingleObject(processHandle, windows.INFINITE)
		windows.CloseHandle(processHandle)
	} else {
		// Fallback: if we couldn't get process handle, wait a bit
		// This shouldn't normally happen, but provides a safety net
		time.Sleep(1 * time.Second)
	}
}

// SignalFirstPhase sends completion message to Phase 1.
// Deprecated: Use SignalFirstPhaseAndWait instead to properly wait for process exit.
func SignalFirstPhase(wnd windows.HWND) {
	if wnd != 0 {
		postMessage(wnd, wmUser, 0, 0)
	}
}

// TryDeleteSelf attempts to delete the current executable.
// This is best-effort - may fail if still in use, cleanup handles it.
func TryDeleteSelf() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	// Try a few times with short delays
	DelayDeleteFile(exe, 5, 50, 100)
}

// CleanupResidualTempDirs scans and cleans up old uninstall temp directories.
// Call this at startup of installer/uninstaller.
// Has a 3-second time limit to avoid blocking startup too long.
func CleanupResidualTempDirs() error {
	tempDir := os.TempDir()
	pattern := filepath.Join(tempDir, tempDirPrefix+"*"+tempDirSuffix)
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return nil // Glob errors are non-fatal
	}

	// Get our own executable path to avoid cleaning up our own temp dir
	selfExe, _ := os.Executable()

	// Time limit to prevent blocking startup (matches Inno Setup's 3s limit)
	deadline := time.Now().Add(3 * time.Second)
	checked := 0

	for _, dir := range dirs {
		// Check time limit after processing at least 10 directories
		if checked >= 10 && time.Now().After(deadline) {
			break
		}
		tryDeleteUninstallDir(dir, selfExe)
		checked++
	}

	return nil
}

// tryDeleteUninstallDir attempts to clean up a single temp directory.
// Matches Inno Setup's TryDeleteUninstallDir logic.
func tryDeleteUninstallDir(dir, selfExe string) {
	uninsExe := filepath.Join(dir, tempExeName)

	// Skip if this is our own process's directory
	if strings.EqualFold(uninsExe, selfExe) {
		return
	}

	// Open directory with DELETE access and FILE_SHARE_READ sharing.
	// This serves as a mutex: concurrent processes will fail with ERROR_SHARING_VIOLATION.
	// Also use FILE_FLAG_BACKUP_SEMANTICS to open directory, and
	// FILE_FLAG_OPEN_REPARSE_POINT to avoid following symlinks.
	dirHandle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(dir),
		windows.DELETE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return
	}
	defer windows.CloseHandle(dirHandle)

	// Verify it's a real directory, not a reparse point (symlink attack protection)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(dirHandle, &info); err != nil {
		return
	}
	// Check it's a directory and NOT a reparse point
	isDir := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	isReparsePoint := info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
	if !isDir || isReparsePoint {
		return
	}

	// Try to open _unins-done.tmp with DELETE access
	// If the uninstaller is still running, it holds this file open with FILE_SHARE_READ,
	// which conflicts with DELETE access, causing ERROR_SHARING_VIOLATION
	doneFile := filepath.Join(dir, doneSignalFileName)
	doneHandle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(doneFile),
		windows.DELETE,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		// Done file doesn't exist or is locked - check time threshold
		// If directory is older than 5 minutes, try to remove it anyway
		// (it might be an empty dir from a previous failed cleanup)
		if !isOlderThan5Minutes(info.LastWriteTime) {
			return
		}
		// Try to remove old empty directory
		os.Remove(dir)
		return
	}
	defer windows.CloseHandle(doneHandle)

	// Delete the temp exe file
	if err := os.Remove(uninsExe); err != nil {
		return
	}

	// Delete the done file
	os.Remove(doneFile)

	// Try to remove the directory (should be empty now)
	os.Remove(dir)
}

// isOlderThan5Minutes checks if a FILETIME is more than 5 minutes old.
func isOlderThan5Minutes(ft windows.Filetime) bool {
	const threshold = 5 * 60 * 10000000 // 5 minutes in 100-nanosecond intervals

	var now windows.Filetime
	windows.GetSystemTimeAsFileTime(&now)

	ftVal := uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
	nowVal := uint64(now.HighDateTime)<<32 | uint64(now.LowDateTime)

	if nowVal > ftVal {
		return (nowVal - ftVal) > threshold
	}
	return false
}

// DelayDeleteFile attempts to delete a file with retries and delays.
// Based on Inno Setup's DelayDeleteFile function.
// It sleeps firstDelayMS before the second try, and subsequentDelayMS before subsequent tries.
func DelayDeleteFile(path string, maxTries int, firstDelayMS, subsequentDelayMS int) error {
	var lastErr error

	for i := 0; i < maxTries; i++ {
		// Sleep BEFORE retry (not after attempt) - matches Inno Setup behavior
		if i == 1 {
			time.Sleep(time.Duration(firstDelayMS) * time.Millisecond)
		} else if i > 1 {
			time.Sleep(time.Duration(subsequentDelayMS) * time.Millisecond)
		}

		if err := os.Remove(path); err == nil {
			return nil
		} else {
			lastErr = err
			// Check for "file not found" - already deleted
			if os.IsNotExist(err) {
				return nil
			}
		}
	}

	return lastErr
}

// createUniqueTempDir creates a unique temp directory for self-delete.
func createUniqueTempDir() (string, error) {
	// Generate random suffix
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	suffix := hex.EncodeToString(buf)

	dir := filepath.Join(os.TempDir(), tempDirPrefix+suffix+tempDirSuffix)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return dir, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Close()
}

// Signal window implementation using raw Windows API

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procCallWindowProcW          = user32.NewProc("CallWindowProcW")
	procSetWindowLongPtrW        = user32.NewProc("SetWindowLongPtrW")
	procRegisterClassW           = user32.NewProc("RegisterClassW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procSendNotifyMessageW       = user32.NewProc("SendNotifyMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procPeekMessageW             = user32.NewProc("PeekMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")

	procMsgWaitForMultipleObjects = user32.NewProc("MsgWaitForMultipleObjects")
)

const (
	wsOverlapped      = 0x00000000
	wmQuit            = 0x0012
	wmQueryEndSession = 0x0011
	pmRemove          = 0x0001
	qsAllinput        = 0x04FF
)

// gwlpWndProc is GWLP_WNDPROC (-4) as uintptr
// We compute it as ^uintptr(3) which equals -4 in two's complement
var gwlpWndProc = ^uintptr(3)

type wndClassW struct {
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
}

type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// oldWndProc stores the original window procedure for CallWindowProc
var oldWndProc uintptr

// signalReceived is set to true when WM_USER is received
var signalReceived bool

// firstPhaseWindowProc is the custom window procedure for Phase 1.
// It denies shutdown requests (WM_QUERYENDSESSION) and handles our signal message.
func firstPhaseWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmQueryEndSession:
		// Return 0 to deny shutdown requests during uninstall
		return 0
	case wmUser:
		// Signal received from Phase 2 - post quit message
		procPostQuitMessage.Call(0)
		signalReceived = true
		return 0
	default:
		// Call original window procedure
		ret, _, _ := procCallWindowProcW.Call(oldWndProc, hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

func createSignalWindow() (windows.HWND, func(), error) {
	// Create a STATIC window (like Inno Setup does)
	staticClass, _ := windows.UTF16PtrFromString("STATIC")

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		0,
		wsOverlapped,
		0, 0, 0, 0,
		0, // HWND_DESKTOP
		0, 0, 0,
	)
	if hwnd == 0 {
		return 0, nil, fmt.Errorf("create window: %w", err)
	}

	// Subclass the window with our custom procedure
	// This allows us to handle WM_QUERYENDSESSION and our signal message
	signalReceived = false
	wndProcCallback := windows.NewCallback(firstPhaseWindowProc)
	oldWndProc, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, wndProcCallback)

	cleanup := func() {
		procDestroyWindow.Call(hwnd)
	}

	return windows.HWND(hwnd), cleanup, nil
}

// waitForSignalOrProcessDeath waits for either a WM_USER message (signal from Phase 2)
// or for the Phase 2 process to terminate. Returns true if signal was received,
// false if process died without signaling (crash case).
// This matches Inno Setup's MsgWaitForMultipleObjects approach.
func waitForSignalOrProcessDeath(wnd windows.HWND, processHandle windows.Handle) bool {
	var m msg
	for {
		// Wait for either process death or message arrival
		// WAIT_OBJECT_0 = process died
		// WAIT_OBJECT_0 + 1 = message available
		ret, _, _ := procMsgWaitForMultipleObjects.Call(
			1,
			uintptr(unsafe.Pointer(&processHandle)),
			0, // bWaitAll = FALSE
			uintptr(windows.INFINITE),
			qsAllinput,
		)

		if ret == uintptr(windows.WAIT_OBJECT_0) {
			// Process died - Phase 2 crashed without signaling
			return false
		}

		if ret != uintptr(windows.WAIT_OBJECT_0+1) {
			// Unexpected result (WAIT_FAILED or other)
			return false
		}

		// Message available - process messages (like Inno Setup's ProcessMsgs)
		for {
			ret, _, _ := procPeekMessageW.Call(
				uintptr(unsafe.Pointer(&m)),
				0, 0, 0,
				pmRemove,
			)
			if ret == 0 {
				break
			}

			if m.message == wmQuit {
				// WM_QUIT posted by our window procedure when signal received
				return signalReceived
			}

			// Dispatch message to window procedure
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func waitForSignal(wnd windows.HWND) {
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			uintptr(wnd),
			0, 0,
		)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		// Any message to our window means Phase 2 is done
		if m.message == wmUser {
			break
		}
	}
}

func postMessage(wnd windows.HWND, msg uint32, wParam, lParam uintptr) {
	procPostMessageW.Call(uintptr(wnd), uintptr(msg), wParam, lParam)
}

// sendNotifyMessage sends a message to a window. For cross-process windows,
// it's async like PostMessage but with guaranteed delivery to the message queue.
// Matches Inno Setup's use of SendNotifyMessage.
func sendNotifyMessage(wnd windows.HWND, msg uint32, wParam, lParam uintptr) {
	procSendNotifyMessageW.Call(uintptr(wnd), uintptr(msg), wParam, lParam)
}

// ScheduleSelfDelete is the legacy API - now triggers Phase 1 of two-phase delete.
// For Phase 2, use DeleteOriginalUninstaller instead.
func ScheduleSelfDelete() error {
	if IsSecondPhase() {
		// In Phase 2, the caller should use DeleteOriginalUninstaller
		return nil
	}
	return RunFirstPhase()
}

// ScheduleFileDelete schedules a file for deletion with retries.
// For running executables, use the two-phase approach instead.
func ScheduleFileDelete(filePath string) error {
	return DelayDeleteFile(filePath, 13, 50, 250)
}

// DeleteFileWhenFree attempts to delete a file, falling back to
// scheduling deletion on reboot if the file is in use.
func DeleteFileWhenFree(path string) error {
	// Try direct deletion first
	if err := os.Remove(path); err == nil {
		return nil
	}

	// Fall back to delete-on-reboot
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	return windows.MoveFileEx(pathPtr, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}
