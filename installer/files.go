package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// StepCopyFile creates a Step that copies a file from src to dst.
// Creates parent directories if needed.
func StepCopyFile(src, dst string) Step {
	return Step{
		Name: fmt.Sprintf("Copy %s", filepath.Base(dst)),
		Action: func() StepResult {
			if err := CopyFile(src, dst); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepCopyExecutable creates a Step that copies an executable file.
// On Windows, this handles locked files by first trying to delete the destination.
func StepCopyExecutable(src, dst string) Step {
	return Step{
		Name: fmt.Sprintf("Copy %s", filepath.Base(dst)),
		Action: func() StepResult {
			if err := CopyExecutable(src, dst); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepEnsureDir creates a Step that ensures a directory exists.
// Skips if the directory already exists.
func StepEnsureDir(path string) Step {
	return Step{
		Name: fmt.Sprintf("Create %s", filepath.Base(path)),
		Action: func() StepResult {
			// Check if already exists
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return Skipped("already exists")
			}
			if err := os.MkdirAll(path, 0755); err != nil {
				return Failed(fmt.Errorf("create directory: %w", err))
			}
			return Success("")
		},
	}
}

// StepDeleteFile creates a Step that deletes a file.
// Skips if the file doesn't exist.
func StepDeleteFile(path string) Step {
	return Step{
		Name: fmt.Sprintf("Delete %s", filepath.Base(path)),
		Action: func() StepResult {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return Skipped("not found")
			}
			if err := os.Remove(path); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepDeleteDirIfEmpty creates a Step that deletes a directory if it's empty.
// Skips if the directory doesn't exist or is not empty.
func StepDeleteDirIfEmpty(path string) Step {
	return Step{
		Name: fmt.Sprintf("Remove %s", filepath.Base(path)),
		Action: func() StepResult {
			entries, err := os.ReadDir(path)
			if os.IsNotExist(err) {
				return Skipped("not found")
			}
			if err != nil {
				return Failed(err)
			}
			if len(entries) > 0 {
				return Skipped("not empty")
			}
			if err := os.Remove(path); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepWriteFile creates a Step that writes content to a file.
// Creates parent directories if needed.
func StepWriteFile(path string, content []byte) Step {
	return Step{
		Name: fmt.Sprintf("Write %s", filepath.Base(path)),
		Action: func() StepResult {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return Failed(fmt.Errorf("create parent directory: %w", err))
			}
			if err := os.WriteFile(path, content, 0644); err != nil {
				return Failed(err)
			}
			return Success("")
		},
	}
}

// StepWriteVersionFile creates a Step that writes a version file.
// The version is written to {dir}/.version
func StepWriteVersionFile(dir, version string) Step {
	return Step{
		Name: "Write version file",
		Action: func() StepResult {
			path := filepath.Join(dir, ".version")
			if err := os.WriteFile(path, []byte(version), 0644); err != nil {
				return Failed(err)
			}
			return Success(version)
		},
	}
}

// StepDeleteVersionFile creates a Step that deletes a version file.
func StepDeleteVersionFile(dir string) Step {
	return StepDeleteFile(filepath.Join(dir, ".version"))
}

// CopyFile copies a file from src to dst, creating parent directories as needed.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy content: %w", err)
	}

	return nil
}

// CopyExecutable copies an executable file, handling locked files on Windows.
// On Windows, if the destination file is locked (in use), this function
// attempts to delete it first, which Windows allows for locked executables
// (the file is deleted when all handles are closed).
func CopyExecutable(src, dst string) error {
	// Try deleting destination first (handles locked files on Windows)
	if _, err := os.Stat(dst); err == nil {
		_ = os.Remove(dst) // Ignore error - will fail on copy if locked
	}
	return CopyFile(src, dst)
}

// ReadVersionFile reads the version from a version file.
// Returns empty string if the file doesn't exist.
func ReadVersionFile(dir string) string {
	path := filepath.Join(dir, ".version")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// FileExists returns true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists returns true if the directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
