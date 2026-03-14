package termui

import (
	"fmt"
	"io"
	"strings"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

type legacyConsole struct {
	w            io.Writer
	zones        []string
	color        bool
	hideTaken    bool
	showPartials bool
	candWidth    int
	zoneWidth    int
	statusWidth  int
	lastLen      int
	liveProgress string
	liveChecking string
}

func newLegacyConsole(w io.Writer, zones []string, candidates []string, color, hideTaken, showPartials bool) *legacyConsole {
	width := len("stem")
	for _, candidate := range candidates {
		if len(candidate) > width {
			width = len(candidate)
		}
	}
	zoneWidth := len("available_zones")
	if joined := strings.Join(upperZones(zones), " "); len(joined) > zoneWidth {
		zoneWidth = len(joined)
	}
	if len("(none)") > zoneWidth {
		zoneWidth = len("(none)")
	}
	statusWidth := len("result")
	for _, value := range []string{"all ✓", "partial", "taken"} {
		if len(value) > statusWidth {
			statusWidth = len(value)
		}
	}
	return &legacyConsole{
		w:            w,
		zones:        zones,
		color:        color,
		hideTaken:    hideTaken,
		showPartials: showPartials,
		candWidth:    width,
		zoneWidth:    zoneWidth,
		statusWidth:  statusWidth,
	}
}

func (c *legacyConsole) Start(total int, filter report.FilterMode) error {
	if c == nil || c.w == nil {
		return nil
	}
	if _, err := fmt.Fprintf(c.w, "Zone files loaded: %s\n", strings.Join(upperZones(c.zones), ", ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "Searching %d stems | filter: %s\n", total, filter); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(c.w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "%-*s  %-*s  %-*s\n", c.candWidth, "stem", c.zoneWidth, "available_zones", c.statusWidth, "result"); err != nil {
		return err
	}
	return nil
}

func (c *legacyConsole) UpdateActive(index, total int, candidate string) error {
	if c == nil || c.w == nil {
		return nil
	}
	c.liveChecking = fmt.Sprintf("checking: %s... [%d/%d]", truncate(candidate, c.candWidth+4), index, total)
	return c.redrawLive()
}

func (c *legacyConsole) UpdateStatus(line string) error {
	if c == nil || c.w == nil {
		return nil
	}
	c.liveProgress = strings.TrimSpace(line)
	return c.redrawLive()
}

func (c *legacyConsole) EmitRow(result match.CandidateResult) error {
	if !c.ShouldEmitRow(result) {
		return c.ClearActive()
	}
	if err := c.clearForDurable(false); err != nil {
		return err
	}
	if c == nil || c.w == nil {
		return nil
	}
	_, err := fmt.Fprintln(c.w, c.formatRow(result))
	return err
}

func (c *legacyConsole) ShouldEmitRow(result match.CandidateResult) bool {
	return shouldEmitRow(c.showPartials, result)
}

func (c *legacyConsole) ClearActive() error {
	if c == nil || c.w == nil {
		return nil
	}
	c.liveChecking = ""
	return c.redrawLive()
}

func (c *legacyConsole) Finish(summary report.Summary) error {
	if err := c.clearForDurable(true); err != nil {
		return err
	}
	if c == nil || c.w == nil {
		return nil
	}
	_, err := fmt.Fprintf(c.w, "Done: checked %d | emitted %d | strong %d\n", summary.TotalCandidates, summary.EmittedResults, summary.AbsentInAll)
	return err
}

func (c *legacyConsole) Note(line string) error {
	if err := c.clearForDurable(true); err != nil {
		return err
	}
	if c == nil || c.w == nil {
		return nil
	}
	_, err := fmt.Fprintln(c.w, line)
	return err
}

func (c *legacyConsole) Close() error {
	return nil
}

func (c *legacyConsole) SetInterrupt(func()) {}

func (c *legacyConsole) redrawLive() error {
	line := strings.TrimSpace(strings.Join(nonEmpty([]string{c.liveProgress, c.liveChecking}), " | "))
	if line == "" {
		return c.clearLine()
	}
	return c.writeLiveLine(line)
}

func (c *legacyConsole) writeLiveLine(line string) error {
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

func (c *legacyConsole) clearLine() error {
	if c == nil || c.w == nil || c.lastLen == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(c.w, "\r%s\r", strings.Repeat(" ", c.lastLen)); err != nil {
		return err
	}
	c.lastLen = 0
	return nil
}

func (c *legacyConsole) clearForDurable(resetProgress bool) error {
	if c == nil || c.w == nil {
		return nil
	}
	c.liveChecking = ""
	if resetProgress {
		c.liveProgress = ""
	}
	return c.clearLine()
}

func (c *legacyConsole) formatRow(result match.CandidateResult) string {
	return formatRow(c.candWidth, c.zoneWidth, c.statusWidth, c.color, result)
}
