package termui

import (
	"io"
	"os"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

type consoleImpl interface {
	Start(total int, filter report.FilterMode) error
	UpdateActive(index, total int, candidate string) error
	UpdateStatus(line string) error
	EmitRow(result match.CandidateResult) error
	ShouldEmitRow(result match.CandidateResult) bool
	ClearActive() error
	Finish(summary report.Summary) error
	Note(line string) error
}

// Console renders the interactive terminal UI.
type Console struct {
	impl consoleImpl
}

// NewConsole creates a new interactive console. Real TTYs use Bubble Tea;
// tests and non-TTY buffers keep a lightweight headless renderer.
func NewConsole(w io.Writer, zones []string, candidates []string, color, hideTaken, showPartials bool) *Console {
	if IsTTY(w) {
		return &Console{
			impl: newBubbleConsole(w, zones, candidates, color, hideTaken, showPartials),
		}
	}
	return &Console{
		impl: newLegacyConsole(w, zones, candidates, color, hideTaken, showPartials),
	}
}

// IsTTY reports whether w appears to be a terminal-like file.
func IsTTY(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ShouldUseInteractive decides whether to enable the interactive console.
func ShouldUseInteractive(format string, forceOn, forceOff bool, stderr io.Writer, isTTY func(io.Writer) bool) bool {
	if format != "text" || forceOff {
		return false
	}
	if forceOn {
		return true
	}
	return isTTY(stderr)
}

// ShouldUseColor decides whether interactive ANSI styling should be enabled.
func ShouldUseColor(forceOn, forceOff bool, stderr io.Writer, isTTY func(io.Writer) bool) bool {
	if forceOff {
		return false
	}
	if forceOn {
		return true
	}
	return isTTY(stderr)
}

// Start initializes the interactive console.
func (c *Console) Start(total int, filter report.FilterMode) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.Start(total, filter)
}

// UpdateActive updates the current in-flight candidate display.
func (c *Console) UpdateActive(index, total int, candidate string) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.UpdateActive(index, total, candidate)
}

// UpdateStatus updates the current ephemeral progress line.
func (c *Console) UpdateStatus(line string) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.UpdateStatus(line)
}

// EmitRow appends a durable result row when the current UI policy allows it.
func (c *Console) EmitRow(result match.CandidateResult) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.EmitRow(result)
}

// ShouldEmitRow reports whether the result should remain durably visible.
func (c *Console) ShouldEmitRow(result match.CandidateResult) bool {
	if c == nil || c.impl == nil {
		return true
	}
	return c.impl.ShouldEmitRow(result)
}

// ClearActive clears the transient active candidate line or state.
func (c *Console) ClearActive() error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.ClearActive()
}

// Finish completes the interactive run.
func (c *Console) Finish(summary report.Summary) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.Finish(summary)
}

// Note appends a durable summary line.
func (c *Console) Note(line string) error {
	if c == nil || c.impl == nil {
		return nil
	}
	return c.impl.Note(line)
}
