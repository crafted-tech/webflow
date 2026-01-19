//go:build linux

package platform

import (
	"errors"
	"os/exec"
	"strings"
)

// CopyToClipboard copies the given text to the system clipboard.
// It tries Wayland (wl-copy) first, then falls back to X11 tools (xclip, xsel).
func CopyToClipboard(text string) error {
	// Try Wayland first
	if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}

	// Fall back to xclip (X11)
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}

	// Fall back to xsel (X11 alternative)
	if _, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}

	return errors.New("no clipboard tool available (install xclip, xsel, or wl-clipboard)")
}
