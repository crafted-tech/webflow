package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger provides structured logging with file output and in-memory buffering.
// It is safe for concurrent use from multiple goroutines.
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	messages []string
	prefix   string
}

// NewLogger creates a new Logger that writes to a timestamped file in the temp directory.
// The prefix is used in the filename: {prefix}-{timestamp}.log
//
// Example:
//
//	log, err := installer.NewLogger("myapp-install")
//	if err != nil {
//	    return err
//	}
//	defer log.Close()
//	log.Info("Starting installation")
func NewLogger(prefix string) (*Logger, error) {
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.log", prefix, timestamp)
	logPath := filepath.Join(tempDir, filename)

	f, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	l := &Logger{
		file:     f,
		path:     logPath,
		messages: make([]string, 0, 100),
		prefix:   prefix,
	}

	// Write header
	l.Info("=== %s Log ===", prefix)
	l.Info("Started: %s", time.Now().Format(time.RFC3339))
	l.Info("Log file: %s", logPath)

	return l, nil
}

// NewLoggerToFile creates a new Logger that appends to the specified file path.
// Useful for subprocess logging where the parent specifies the log file.
//
// Example:
//
//	// In subprocess, use same log file as parent
//	log, err := installer.NewLoggerToFile(parentLogPath)
func NewLoggerToFile(logPath string) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	l := &Logger{
		file:     f,
		path:     logPath,
		messages: make([]string, 0, 100),
	}

	return l, nil
}

// Close closes the log file.
func (l *Logger) Close() {
	if l == nil || l.file == nil {
		return
	}
	l.Info("")
	l.Info("=== Log ended: %s ===", time.Now().Format(time.RFC3339))
	l.file.Close()
}

// Path returns the path to the log file.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Content returns the full log content as a string.
// Useful for displaying in a UI or copying to clipboard.
func (l *Logger) Content() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.messages, "\n")
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...any) {
	l.log("INFO", format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) {
	l.log("ERROR", format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...any) {
	l.log("WARN", format, args...)
}

// Step logs a major milestone/step in the process.
func (l *Logger) Step(format string, args ...any) {
	l.log("STEP", format, args...)
}

func (l *Logger) log(level, format string, args ...any) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] %s: %s", timestamp, level, msg)

	l.messages = append(l.messages, line)

	if l.file != nil {
		fmt.Fprintln(l.file, line)
		l.file.Sync()
	}
}
