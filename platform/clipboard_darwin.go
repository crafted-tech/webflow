//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

// CopyToClipboard copies the given text to the system clipboard using pbcopy.
func CopyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
