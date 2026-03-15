package candidates

import (
	"strings"

	"github.com/genestevens/domain-finder/internal/genquality"
)

// Collector incrementally normalizes and deduplicates candidate stems while
// preserving first-seen order.
type Collector struct {
	seen         map[string]struct{}
	familyCounts map[string]int
}

// BatchReport describes one tolerant candidate-ingest pass.
type BatchReport struct {
	Accepted         []string
	Invalid          int
	Duplicates       int
	TooShort         int
	BannedPrefixes   int
	BannedSuffixes   int
	BannedSubstrings int
	LexicalRejected  int
	FamilyRejected   int
	QualityRejected  int
	QualityReasons   map[string]int
}

// GeneratedPolicy defines generation-specific acceptance rules applied after
// normal stem validation.
type GeneratedPolicy struct {
	AvoidSubstrings []string
	AvoidPrefixes   []string
	AvoidSuffixes   []string
	MinLength       int
	QualityProfile  string
	FamilyLimit     int
}

const defaultGeneratedFamilyLimit = 2

// NewCollector creates an empty collector.
func NewCollector() *Collector {
	return &Collector{
		seen:         make(map[string]struct{}),
		familyCounts: make(map[string]int),
	}
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
		if policy.MinLength > 0 && len(normalized) < policy.MinLength {
			report.TooShort++
			report.LexicalRejected++
			continue
		}
		if containsAvoidSubstring(normalized, policy.AvoidSubstrings) {
			report.BannedSubstrings++
			report.LexicalRejected++
			continue
		}
		if hasAvoidPrefix(normalized, policy.AvoidPrefixes) {
			report.BannedPrefixes++
			report.LexicalRejected++
			continue
		}
		if hasAvoidSuffix(normalized, policy.AvoidSuffixes) {
			report.BannedSuffixes++
			report.LexicalRejected++
			continue
		}
		evaluation := genquality.Evaluate(normalized, policy.QualityProfile)
		if !evaluation.Accepted {
			report.QualityRejected++
			if report.QualityReasons == nil {
				report.QualityReasons = make(map[string]int, len(evaluation.Reasons))
			}
			for _, reason := range evaluation.Reasons {
				report.QualityReasons[string(reason)]++
			}
			continue
		}
		if _, ok := c.seen[normalized]; ok {
			report.Duplicates++
			continue
		}
		if c.familyCounts[familySignature(normalized)] >= generatedFamilyLimit(policy.FamilyLimit) {
			report.FamilyRejected++
			continue
		}
		if len(report.Accepted) >= limit {
			report.Duplicates++
			continue
		}
		c.seen[normalized] = struct{}{}
		c.familyCounts[familySignature(normalized)]++
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

func hasAvoidPrefix(value string, banned []string) bool {
	for _, fragment := range banned {
		if fragment == "" {
			continue
		}
		if strings.HasPrefix(value, fragment) {
			return true
		}
	}
	return false
}

func hasAvoidSuffix(value string, banned []string) bool {
	for _, fragment := range banned {
		if fragment == "" {
			continue
		}
		if strings.HasSuffix(value, fragment) {
			return true
		}
	}
	return false
}

func generatedFamilyLimit(limit int) int {
	if limit > 0 {
		return limit
	}
	return defaultGeneratedFamilyLimit
}

func familySignature(stem string) string {
	if stem == "" {
		return ""
	}
	prefix := stem
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	consonants := firstConsonants(stem, 2)
	if consonants == "" {
		consonants = prefix
	}
	return prefix + "|" + consonants + "|" + lengthBucket(stem) + "|" + endingClass(stem)
}

func firstConsonants(stem string, n int) string {
	var out strings.Builder
	for _, r := range stem {
		if strings.ContainsRune("aeiou", r) {
			continue
		}
		if 'a' <= r && r <= 'z' {
			out.WriteRune(r)
			if out.Len() >= n {
				break
			}
		}
	}
	return out.String()
}

func lengthBucket(stem string) string {
	switch len(stem) {
	case 0, 1, 2, 3, 4:
		return "short"
	case 5, 6, 7:
		return "compact"
	case 8, 9:
		return "medium"
	default:
		return "long"
	}
}

func endingClass(stem string) string {
	if stem == "" {
		return "none"
	}
	last := stem[len(stem)-1]
	switch last {
	case 'k', 't', 'x', 'q', 'r', 'd', 'p', 'm', 'n', 'g':
		return "hard"
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return "open"
	default:
		return "closed"
	}
}
