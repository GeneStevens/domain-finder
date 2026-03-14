package termui

import (
	"fmt"
	"io"
	"strings"
)

// ActivityLine renders a single reusable transient activity line.
// It is intentionally simple: carriage-return updates plus explicit clearing.
type ActivityLine struct {
	w       io.Writer
	lastLen int
}

// NewActivityLine creates a new activity-line renderer.
func NewActivityLine(w io.Writer) *ActivityLine {
	return &ActivityLine{w: w}
}

// Update rewrites the current activity line in place.
func (a *ActivityLine) Update(message string) error {
	if a == nil || a.w == nil {
		return nil
	}

	padding := ""
	if a.lastLen > len(message) {
		padding = strings.Repeat(" ", a.lastLen-len(message))
	}

	if _, err := fmt.Fprintf(a.w, "\r%s%s", message, padding); err != nil {
		return err
	}
	a.lastLen = len(message)
	return nil
}

// Clear removes the current transient activity line.
func (a *ActivityLine) Clear() error {
	if a == nil || a.w == nil || a.lastLen == 0 {
		return nil
	}

	if _, err := fmt.Fprintf(a.w, "\r%s\r", strings.Repeat(" ", a.lastLen)); err != nil {
		return err
	}
	a.lastLen = 0
	return nil
}

// Finish clears any transient activity line and optionally prints a final
// durable status line to the same writer.
func (a *ActivityLine) Finish(message string) error {
	if err := a.Clear(); err != nil {
		return err
	}
	if a == nil || a.w == nil || message == "" {
		return nil
	}
	_, err := fmt.Fprintln(a.w, message)
	return err
}
