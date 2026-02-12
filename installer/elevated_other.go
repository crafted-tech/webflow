//go:build !windows

package installer

import (
	"encoding/json"
	"fmt"

	"github.com/crafted-tech/webflow"
)

// ElevatedExecConfig configures an elevated subprocess execution.
type ElevatedExecConfig struct {
	Title       string
	ArgsBuilder func(progressPath string) string
	PIDTimeout  any // time.Duration on windows
}

// ElevatedOutcome holds the result from the elevated child.
type ElevatedOutcome struct {
	RawResult json.RawMessage
	ExitCode  uint32
}

// RunElevatedWithProgress is not supported on non-Windows platforms.
func RunElevatedWithProgress(ui *webflow.Flow, cfg ElevatedExecConfig) (*ElevatedOutcome, error) {
	return nil, fmt.Errorf("elevated execution not supported on this platform")
}
