package termui

import (
	"fmt"
	"strings"

	"github.com/genestevens/domain-finder/internal/match"
)

func consoleColumnWidths(zones []string, candidates []string) (int, int, int) {
	candWidth := len("stem")
	for _, candidate := range candidates {
		if len(candidate) > candWidth {
			candWidth = len(candidate)
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
	return candWidth, zoneWidth, statusWidth
}

func formatRow(candWidth, zoneWidth, statusWidth int, color bool, result match.CandidateResult) string {
	availableText := availableZonesText(result)
	statusText := statusText(result, color)
	return fmt.Sprintf("%-*s  %-*s  %-*s", candWidth, result.Candidate, zoneWidth, availableText, statusWidth, statusText)
}

func shouldEmitRow(showPartials bool, result match.CandidateResult) bool {
	if result.AbsentInAll {
		return true
	}
	return showPartials && availableZonesText(result) != "(none)"
}

func availableZonesText(result match.CandidateResult) string {
	available := make([]string, 0, len(result.Zones))
	for _, zone := range result.Zones {
		if !zone.Present {
			available = append(available, strings.ToUpper(zone.Zone))
		}
	}
	if len(available) == 0 {
		return "(none)"
	}
	return strings.Join(available, " ")
}

func statusText(result match.CandidateResult, color bool) string {
	switch {
	case result.AbsentInAll:
		return styleStrong("all ✓", color)
	case result.PresentInAny:
		if availableZonesText(result) == "(none)" {
			return "taken"
		}
		return "partial"
	default:
		return "partial"
	}
}

func upperZones(zones []string) []string {
	parts := make([]string, 0, len(zones))
	for _, zone := range zones {
		parts = append(parts, strings.ToUpper(zone))
	}
	return parts
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1]
}

func styleStrong(value string, color bool) string {
	if !color {
		return value
	}
	return "\x1b[1;97;42m" + value + "\x1b[0m"
}

func nonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}
