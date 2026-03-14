package termui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

// Console renders a lightweight streaming interactive console on stderr.
type Console struct {
	w         io.Writer
	zones     []string
	candWidth int
	lastLen   int
}

// NewConsole creates a new streaming console.
func NewConsole(w io.Writer, zones []string, candidates []string) *Console {
	width := len("candidate")
	for _, candidate := range candidates {
		if len(candidate) > width {
			width = len(candidate)
		}
	}
	return &Console{
		w:         w,
		zones:     zones,
		candWidth: width,
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

// Start prints the compact interactive header and table header.
func (c *Console) Start(total int, filter report.FilterMode) error {
	if c == nil || c.w == nil {
		return nil
	}
	if _, err := fmt.Fprintf(c.w, "Zone files loaded: %s\n", strings.Join(upperZones(c.zones), ", ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "Searching %d domains | filter: %s\n", total, filter); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "%-*s  %s\n", c.candWidth, "candidate", c.zoneHeader()); err != nil {
		return err
	}
	return nil
}

// UpdateActive rewrites the transient active candidate line in place.
func (c *Console) UpdateActive(index, total int, candidate string) error {
	if c == nil || c.w == nil {
		return nil
	}
	line := fmt.Sprintf("> [%d/%d] %-*s  %s  checking", index, total, c.candWidth, candidate, c.placeholderCells())
	return c.rewrite(line)
}

// EmitRow writes a durable emitted row to the console.
func (c *Console) EmitRow(result match.CandidateResult) error {
	if err := c.ClearActive(); err != nil {
		return err
	}
	if c == nil || c.w == nil {
		return nil
	}
	_, err := fmt.Fprintln(c.w, c.formatRow(result))
	return err
}

// ClearActive clears the transient active line.
func (c *Console) ClearActive() error {
	if c == nil || c.w == nil || c.lastLen == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(c.w, "\r%s\r", strings.Repeat(" ", c.lastLen)); err != nil {
		return err
	}
	c.lastLen = 0
	return nil
}

// Finish clears any active line and prints a compact completion line.
func (c *Console) Finish(summary report.Summary) error {
	if err := c.ClearActive(); err != nil {
		return err
	}
	if c == nil || c.w == nil {
		return nil
	}
	_, err := fmt.Fprintf(c.w, "Done: checked %d, emitted %d\n", summary.TotalCandidates, summary.EmittedResults)
	return err
}

func (c *Console) rewrite(line string) error {
	padding := ""
	if c.lastLen > len(line) {
		padding = strings.Repeat(" ", c.lastLen-len(line))
	}
	if _, err := fmt.Fprintf(c.w, "\r%s%s", line, padding); err != nil {
		return err
	}
	c.lastLen = len(line)
	return nil
}

func (c *Console) zoneHeader() string {
	parts := make([]string, 0, len(c.zones))
	for _, zone := range c.zones {
		parts = append(parts, fmt.Sprintf("%-*s", cellWidth(zone), strings.ToUpper(zone)))
	}
	return strings.Join(parts, " ")
}

func (c *Console) placeholderCells() string {
	parts := make([]string, 0, len(c.zones))
	for _, zone := range c.zones {
		parts = append(parts, fmt.Sprintf("%-*s", cellWidth(zone), "..."))
	}
	return strings.Join(parts, " ")
}

func (c *Console) formatRow(result match.CandidateResult) string {
	parts := make([]string, 0, len(result.Zones))
	for _, zone := range result.Zones {
		value := "-"
		if zone.Present {
			value = "hit"
		}
		parts = append(parts, fmt.Sprintf("%-*s", cellWidth(zone.Zone), value))
	}
	return fmt.Sprintf("  %-*s  %s", c.candWidth, result.Candidate, strings.Join(parts, " "))
}

func upperZones(zones []string) []string {
	parts := make([]string, 0, len(zones))
	for _, zone := range zones {
		parts = append(parts, strings.ToUpper(zone))
	}
	return parts
}

func cellWidth(zone string) int {
	width := len(zone)
	if width < 3 {
		width = 3
	}
	return width
}
