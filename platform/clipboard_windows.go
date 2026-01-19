//go:build windows

package platform

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CopyToClipboard copies the given text to the Windows clipboard.
func CopyToClipboard(text string) error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")

	openClipboard := user32.NewProc("OpenClipboard")
	closeClipboard := user32.NewProc("CloseClipboard")
	emptyClipboard := user32.NewProc("EmptyClipboard")
	setClipboardData := user32.NewProc("SetClipboardData")
	globalAlloc := kernel32.NewProc("GlobalAlloc")
	globalLock := kernel32.NewProc("GlobalLock")
	globalUnlock := kernel32.NewProc("GlobalUnlock")
	globalFree := kernel32.NewProc("GlobalFree")

	// Open clipboard
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("OpenClipboard failed: %w", err)
	}
	defer closeClipboard.Call()

	// Empty clipboard
	emptyClipboard.Call()

	// Allocate global memory
	data := windows.StringToUTF16(text)
	size := len(data) * 2 // UTF-16 is 2 bytes per character

	hMem, _, err := globalAlloc.Call(0x0042, uintptr(size)) // GMEM_MOVEABLE | GMEM_ZEROINIT
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed: %w", err)
	}

	// Lock and copy
	ptr, _, err := globalLock.Call(hMem)
	if ptr == 0 {
		globalFree.Call(hMem)
		return fmt.Errorf("GlobalLock failed: %w", err)
	}

	// Copy data using unsafe.Slice for safer memory access.
	dest := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(data)) //nolint:govet // ptr is valid from GlobalLock
	copy(dest, data)

	globalUnlock.Call(hMem)

	// Set clipboard data (CF_UNICODETEXT = 13)
	r, _, err = setClipboardData.Call(13, hMem)
	if r == 0 {
		globalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData failed: %w", err)
	}

	return nil
}
