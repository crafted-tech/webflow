package installer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// IPC protocol line prefixes for parent↔child communication via temp file.
const (
	IPCPrefixPID      = "PID:"
	IPCPrefixProgress = "PROGRESS:"
	IPCPrefixResult   = "RESULT:"
)

// ProgressWriter is used by the elevated child process to write progress
// updates to a shared temp file. Each write is followed by f.Sync() to
// ensure the parent can read it promptly.
type ProgressWriter struct {
	f *os.File
}

// NewProgressWriter opens the progress file for appending.
func NewProgressWriter(path string) (*ProgressWriter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open progress file: %w", err)
	}
	return &ProgressWriter{f: f}, nil
}

// WritePID writes the child's PID as the first line.
func (w *ProgressWriter) WritePID(pid int) error {
	return w.writeLine(fmt.Sprintf("%s%d", IPCPrefixPID, pid))
}

// WriteProgress writes a progress update line.
func (w *ProgressWriter) WriteProgress(percent float64, message string) error {
	return w.writeLine(fmt.Sprintf("%s%.1f:%s", IPCPrefixProgress, percent, message))
}

// WriteResult writes the final result as a JSON line. The value can be any
// JSON-serializable type — the caller on the reading side unmarshals it
// into their own result struct.
func (w *ProgressWriter) WriteResult(result any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return w.writeLine(fmt.Sprintf("%s%s", IPCPrefixResult, string(data)))
}

// Close closes the progress file.
func (w *ProgressWriter) Close() error {
	if w.f != nil {
		return w.f.Close()
	}
	return nil
}

func (w *ProgressWriter) writeLine(line string) error {
	_, err := fmt.Fprintln(w.f, line)
	if err != nil {
		return err
	}
	return w.f.Sync()
}

// ProgressReader is used by the non-elevated parent process to poll
// progress updates from the shared temp file. It tracks the file offset
// to only process new lines.
type ProgressReader struct {
	path   string
	offset int64
}

// NewProgressReader creates a reader for the progress file.
func NewProgressReader(path string) *ProgressReader {
	return &ProgressReader{path: path}
}

// ProgressLine represents a parsed line from the progress file.
type ProgressLine struct {
	Type      string          // "PID", "PROGRESS", "RESULT"
	PID       uint32          // Only set for PID lines
	Percent   float64         // Only set for PROGRESS lines
	Message   string          // Only set for PROGRESS lines
	RawResult json.RawMessage // Only set for RESULT lines — caller unmarshals to their type
}

// ReadNewLines reads any new complete lines since the last read.
func (r *ProgressReader) ReadNewLines() ([]ProgressLine, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(r.offset, 0); err != nil {
		return nil, err
	}

	var lines []ProgressLine
	scanner := bufio.NewScanner(f)
	// 1MB buffer to handle large RESULT lines (e.g. embedded LogContent).
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var bytesRead int64
	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += int64(len(scanner.Bytes())) + 1 // +1 for newline

		parsed, ok := parseLine(line)
		if ok {
			lines = append(lines, parsed)
		}
	}

	r.offset += bytesRead
	return lines, scanner.Err()
}

func parseLine(line string) (ProgressLine, bool) {
	switch {
	case strings.HasPrefix(line, IPCPrefixPID):
		pidStr := strings.TrimPrefix(line, IPCPrefixPID)
		pid, err := strconv.ParseUint(pidStr, 10, 32)
		if err != nil {
			return ProgressLine{}, false
		}
		return ProgressLine{Type: "PID", PID: uint32(pid)}, true

	case strings.HasPrefix(line, IPCPrefixProgress):
		rest := strings.TrimPrefix(line, IPCPrefixProgress)
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			return ProgressLine{}, false
		}
		percent, err := strconv.ParseFloat(rest[:colonIdx], 64)
		if err != nil {
			return ProgressLine{}, false
		}
		return ProgressLine{
			Type:    "PROGRESS",
			Percent: percent,
			Message: rest[colonIdx+1:],
		}, true

	case strings.HasPrefix(line, IPCPrefixResult):
		jsonData := strings.TrimPrefix(line, IPCPrefixResult)
		return ProgressLine{
			Type:      "RESULT",
			RawResult: json.RawMessage(jsonData),
		}, true

	default:
		return ProgressLine{}, false
	}
}

// CreateProgressFile creates an empty temp file for IPC and returns its path.
func CreateProgressFile() (string, error) {
	f, err := os.CreateTemp("", "webflow-progress-*.txt")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	return path, nil
}
