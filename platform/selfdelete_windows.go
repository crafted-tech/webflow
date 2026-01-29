//go:build windows

package platform

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ntdll                = windows.NewLazySystemDLL("ntdll.dll")
	procNtSetInformationFile = ntdll.NewProc("NtSetInformationFile")
)

// ScheduleSelfDelete arranges for the current executable to be deleted
// after the process exits using Windows alternate data stream renaming.
//
// This technique works by:
// 1. Opening the file with DELETE access
// 2. Renaming it to an alternate data stream (which is allowed for running exes)
// 3. Marking the file for deletion
// 4. The file is deleted when the process exits and all handles are closed
//
// This is useful for uninstallers that need to delete themselves.
func ScheduleSelfDelete() error {
	var exePath [windows.MAX_PATH + 1]uint16

	// Get the path to our own executable
	n, err := windows.GetModuleFileName(0, &exePath[0], windows.MAX_PATH)
	if err != nil || n == 0 {
		return fmt.Errorf("get module filename: %w", err)
	}

	return scheduleFileDeleteByPath(&exePath[0])
}

// ScheduleFileDelete arranges for the specified file to be deleted
// after it is no longer in use using Windows alternate data stream renaming.
func ScheduleFileDelete(filePath string) error {
	pathPtr, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return fmt.Errorf("convert path: %w", err)
	}
	return scheduleFileDeleteByPath(pathPtr)
}

// scheduleFileDeleteByPath implements the self-delete using Windows APIs.
// Based on the technique from https://github.com/secur30nly/go-self-delete
func scheduleFileDeleteByPath(pathPtr *uint16) error {
	// Open file with DELETE access
	handle, err := windows.CreateFile(
		pathPtr,
		windows.DELETE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("open file for delete: %w", err)
	}

	// Rename to alternate data stream - this is the key trick.
	// You can rename a running executable on Windows, just can't delete it directly.
	if err := renameToStream(handle); err != nil {
		windows.CloseHandle(handle)
		return fmt.Errorf("rename to stream: %w", err)
	}
	windows.CloseHandle(handle)

	// Reopen and mark for deletion
	handle, err = windows.CreateFile(
		pathPtr,
		windows.DELETE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("reopen file: %w", err)
	}

	if err := markForDeletion(handle); err != nil {
		windows.CloseHandle(handle)
		return fmt.Errorf("mark for deletion: %w", err)
	}
	windows.CloseHandle(handle)

	return nil
}

// fileRenameInfo matches Windows FILE_RENAME_INFO structure
type fileRenameInfo struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// renameToStream renames the file to an alternate data stream
func renameToStream(handle windows.Handle) error {
	streamName, _ := windows.UTF16FromString(":deleteme")

	// streamName includes null terminator, but FileNameLength should exclude it
	nameLen := len(streamName) - 1

	// Calculate size: struct + extra space for filename (without null)
	infoSize := unsafe.Sizeof(fileRenameInfo{}) + uintptr(nameLen*2)
	buf := make([]byte, infoSize)

	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	info.ReplaceIfExists = 0
	info.RootDirectory = 0
	info.FileNameLength = uint32(nameLen * 2) // Byte length, excluding null terminator

	// Copy stream name into the buffer after the struct (excluding null)
	fileNamePtr := unsafe.Pointer(&info.FileName[0])
	for i := 0; i < nameLen; i++ {
		*(*uint16)(unsafe.Pointer(uintptr(fileNamePtr) + uintptr(i*2))) = streamName[i]
	}

	return windows.SetFileInformationByHandle(
		handle,
		windows.FileRenameInfo,
		&buf[0],
		uint32(len(buf)),
	)
}

// FILE_DISPOSITION_INFORMATION_EX flags
// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information_ex
const (
	fileDispositionFlagDelete         = 0x00000001
	fileDispositionFlagPosixSemantics = 0x00000002
)

// FileDispositionInformationEx for NtSetInformationFile
const fileDispositionInformationEx = 64

// ioStatusBlock matches Windows IO_STATUS_BLOCK
type ioStatusBlock struct {
	Status      uintptr
	Information uintptr
}

// markForDeletion marks the file for deletion when all handles are closed.
// Uses NtSetInformationFile with FileDispositionInformationEx for reliable
// deletion on all Windows versions including 24H2.
// Reference: https://github.com/LloydLabs/delete-self-poc
// Fix for 24H2: https://github.com/MaangoTaachyon/SelfDeletion-Updated
func markForDeletion(handle windows.Handle) error {
	var iosb ioStatusBlock
	// FILE_DISPOSITION_INFORMATION_EX with POSIX semantics
	// This works on Windows 10 1607+ and is required for 24H2
	flags := uint32(fileDispositionFlagDelete | fileDispositionFlagPosixSemantics)

	r0, _, _ := procNtSetInformationFile.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(unsafe.Pointer(&flags)),
		unsafe.Sizeof(flags),
		uintptr(fileDispositionInformationEx),
	)

	if r0 == 0 { // STATUS_SUCCESS
		return nil
	}

	// Fall back to classic FILE_DISPOSITION_INFO for older Windows
	var deleteFlag byte = 1
	return windows.SetFileInformationByHandle(
		handle,
		windows.FileDispositionInfo,
		&deleteFlag,
		1,
	)
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
