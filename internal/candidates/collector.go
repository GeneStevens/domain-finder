package candidates

import "strings"

// Collector incrementally normalizes and deduplicates candidate stems while
// preserving first-seen order.
type Collector struct {
	seen map[string]struct{}
}

// BatchReport describes one tolerant candidate-ingest pass.
type BatchReport struct {
	Accepted        []string
	Invalid         int
	Duplicates      int
	LexicalRejected int
}

// GeneratedPolicy defines generation-specific acceptance rules applied after
// normal stem validation.
type GeneratedPolicy struct {
	AvoidSubstrings []string
}

// NewCollector creates an empty collector.
func NewCollector() *Collector {
	return &Collector{seen: make(map[string]struct{})}
}

// Add normalizes a raw candidate and returns the normalized stem if it was new.
func (c *Collector) Add(raw string) (string, bool, error) {
	normalized, err := NormalizeCandidate(raw)
	if err != nil {
		return "", false, err
	}
	if _, ok := c.seen[normalized]; ok {
		return normalized, false, nil
	}
	c.seen[normalized] = struct{}{}
	return normalized, true, nil
}

// AddAll adds many raw candidates and returns only the newly accepted stems in input order.
func (c *Collector) AddAll(raws []string) ([]string, error) {
	out := make([]string, 0, len(raws))
	for _, raw := range raws {
		normalized, added, err := c.Add(raw)
		if err != nil {
			return nil, err
		}
		if added {
			out = append(out, normalized)
		}
	}
	return out, nil
}

// AddAllReport adds many raw candidates and reports accepted, invalid, and
// duplicate values without failing the whole batch on invalid generated output.
func (c *Collector) AddAllReport(raws []string) BatchReport {
	return c.AddAllReportLimited(raws, len(raws))
}

// AddAllReportLimited behaves like AddAllReport but caps new accepted stems at
// the provided limit so bounded generation can avoid overshooting requested work.
func (c *Collector) AddAllReportLimited(raws []string, limit int) BatchReport {
	return c.AddGeneratedReportLimited(raws, limit, GeneratedPolicy{})
}

// AddGeneratedReportLimited applies generation-specific quality rules without
// changing the normal validation behavior for manual candidate sources.
func (c *Collector) AddGeneratedReportLimited(raws []string, limit int, policy GeneratedPolicy) BatchReport {
	report := BatchReport{
		Accepted: make([]string, 0, len(raws)),
	}
	if limit < 0 {
		limit = 0
	}
	for _, raw := range raws {
		normalized, err := NormalizeCandidate(raw)
		if err != nil {
			report.Invalid++
			continue
		}
		if containsAvoidSubstring(normalized, policy.AvoidSubstrings) {
			report.LexicalRejected++
			continue
		}
		if _, ok := c.seen[normalized]; ok {
			report.Duplicates++
			continue
		}
		if len(report.Accepted) >= limit {
			report.Duplicates++
			continue
		}
		c.seen[normalized] = struct{}{}
		report.Accepted = append(report.Accepted, normalized)
	}
	return report
}

func containsAvoidSubstring(value string, banned []string) bool {
	for _, fragment := range banned {
		if fragment == "" {
			continue
		}
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
