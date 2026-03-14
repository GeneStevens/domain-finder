package candidates

import (
	"fmt"
	"sort"
	"strings"
)

// GenerationDiagnostics aggregates generated-stem rejection signals across a run.
type GenerationDiagnostics struct {
	Invalid         int
	Banned          int
	QualityRejected int
	Duplicates      int
	QualityReasons  map[string]int
}

// MergeBatch folds one generated-ingest batch report into the run-level diagnostics.
func (d *GenerationDiagnostics) MergeBatch(report BatchReport) {
	if d == nil {
		return
	}
	d.Invalid += report.Invalid
	d.Banned += report.LexicalRejected
	d.QualityRejected += report.QualityRejected
	d.Duplicates += report.Duplicates
	if len(report.QualityReasons) == 0 {
		return
	}
	if d.QualityReasons == nil {
		d.QualityReasons = make(map[string]int, len(report.QualityReasons))
	}
	for reason, count := range report.QualityReasons {
		d.QualityReasons[reason] += count
	}
}

// HasData reports whether any generated rejection diagnostics were recorded.
func (d GenerationDiagnostics) HasData() bool {
	return d.Invalid > 0 || d.Banned > 0 || d.QualityRejected > 0 || d.Duplicates > 0 || len(d.QualityReasons) > 0
}

// Lines renders a stable compact diagnostics block.
func (d GenerationDiagnostics) Lines() []string {
	if !d.HasData() {
		return nil
	}
	lines := []string{"generation diagnostics"}
	if d.Banned > 0 {
		lines = append(lines, fmt.Sprintf("  banned_substring: %d", d.Banned))
	}
	reasons := make([]reasonCount, 0, len(d.QualityReasons))
	for reason, count := range d.QualityReasons {
		reasons = append(reasons, reasonCount{reason: reason, count: count})
	}
	sort.Slice(reasons, func(i, j int) bool {
		if reasons[i].count != reasons[j].count {
			return reasons[i].count > reasons[j].count
		}
		return reasons[i].reason < reasons[j].reason
	})
	for _, entry := range reasons {
		lines = append(lines, fmt.Sprintf("  quality.%s: %d", entry.reason, entry.count))
	}
	if d.Invalid > 0 {
		lines = append(lines, fmt.Sprintf("  invalid: %d", d.Invalid))
	}
	if d.Duplicates > 0 {
		lines = append(lines, fmt.Sprintf("  duplicates: %d", d.Duplicates))
	}
	return lines
}

func (d GenerationDiagnostics) String() string {
	return strings.Join(d.Lines(), "\n")
}

type reasonCount struct {
	reason string
	count  int
}
