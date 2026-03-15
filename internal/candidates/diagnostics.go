package candidates

import (
	"fmt"
	"sort"
	"strings"
)

// GenerationDiagnostics aggregates generated-stem rejection signals across a run.
type GenerationDiagnostics struct {
	Invalid            int
	Banned             int
	TooShort           int
	ScoreRejected      int
	PhoneticRejected   int
	StructuralRejected int
	BannedSubstrings   int
	BannedPrefixes     int
	BannedSuffixes     int
	QualityRejected    int
	FamilyRejected     int
	Duplicates         int
	QualityReasons     map[string]int
	ScoreBuckets       map[string]int
}

// MergeBatch folds one generated-ingest batch report into the run-level diagnostics.
func (d *GenerationDiagnostics) MergeBatch(report BatchReport) {
	if d == nil {
		return
	}
	d.Invalid += report.Invalid
	d.Banned += report.LexicalRejected
	d.TooShort += report.TooShort
	d.ScoreRejected += report.ScoreRejected
	d.PhoneticRejected += report.PhoneticRejected
	d.StructuralRejected += report.StructuralRejected
	d.BannedSubstrings += report.BannedSubstrings
	d.BannedPrefixes += report.BannedPrefixes
	d.BannedSuffixes += report.BannedSuffixes
	d.QualityRejected += report.QualityRejected
	d.FamilyRejected += report.FamilyRejected
	d.Duplicates += report.Duplicates
	if len(report.ScoreBuckets) > 0 {
		if d.ScoreBuckets == nil {
			d.ScoreBuckets = make(map[string]int, len(report.ScoreBuckets))
		}
		for bucket, count := range report.ScoreBuckets {
			d.ScoreBuckets[bucket] += count
		}
	}
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
	return d.Invalid > 0 || d.Banned > 0 || d.TooShort > 0 || d.ScoreRejected > 0 || d.PhoneticRejected > 0 || d.StructuralRejected > 0 || d.BannedSubstrings > 0 || d.BannedPrefixes > 0 || d.BannedSuffixes > 0 || d.QualityRejected > 0 || d.FamilyRejected > 0 || d.Duplicates > 0 || len(d.QualityReasons) > 0 || len(d.ScoreBuckets) > 0
}

// Lines renders a stable compact diagnostics block.
func (d GenerationDiagnostics) Lines() []string {
	if !d.HasData() {
		return nil
	}
	lines := []string{"generation diagnostics"}
	if d.TooShort > 0 {
		lines = append(lines, fmt.Sprintf("  too_short: %d", d.TooShort))
	}
	if d.ScoreRejected > 0 {
		lines = append(lines, fmt.Sprintf("  score_rejected: %d", d.ScoreRejected))
	}
	if d.PhoneticRejected > 0 {
		lines = append(lines, fmt.Sprintf("  phonetic_rejected: %d", d.PhoneticRejected))
	}
	if d.StructuralRejected > 0 {
		lines = append(lines, fmt.Sprintf("  structural_rejected: %d", d.StructuralRejected))
	}
	if d.BannedSubstrings > 0 {
		lines = append(lines, fmt.Sprintf("  banned_substring: %d", d.BannedSubstrings))
	}
	if d.BannedPrefixes > 0 {
		lines = append(lines, fmt.Sprintf("  banned_prefix: %d", d.BannedPrefixes))
	}
	if d.BannedSuffixes > 0 {
		lines = append(lines, fmt.Sprintf("  banned_suffix: %d", d.BannedSuffixes))
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
	if len(d.ScoreBuckets) > 0 {
		buckets := make([]reasonCount, 0, len(d.ScoreBuckets))
		for bucket, count := range d.ScoreBuckets {
			buckets = append(buckets, reasonCount{reason: bucket, count: count})
		}
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].reason < buckets[j].reason
		})
		for _, bucket := range buckets {
			lines = append(lines, fmt.Sprintf("  score.%s: %d", bucket.reason, bucket.count))
		}
	}
	if d.FamilyRejected > 0 {
		lines = append(lines, fmt.Sprintf("  family_rejected: %d", d.FamilyRejected))
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
