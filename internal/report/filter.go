package report

import (
	"fmt"

	"github.com/gene/domain-finder/internal/match"
)

// FilterMode controls which classified candidate results are emitted.
type FilterMode string

const (
	FilterAll         FilterMode = "all"
	FilterAbsentInAll FilterMode = "absent-in-all"
)

// ParseFilterMode validates a user-provided filter mode.
func ParseFilterMode(value string) (FilterMode, error) {
	mode := FilterMode(value)
	switch mode {
	case FilterAll, FilterAbsentInAll:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported -filter %q: want all or absent-in-all", value)
	}
}

// ApplyFilter returns a deterministic filtered slice while preserving input order.
func ApplyFilter(results []match.CandidateResult, mode FilterMode) []match.CandidateResult {
	if mode == FilterAll {
		out := make([]match.CandidateResult, len(results))
		copy(out, results)
		return out
	}

	out := make([]match.CandidateResult, 0, len(results))
	for _, result := range results {
		if result.AbsentInAll {
			out = append(out, result)
		}
	}
	return out
}

// ShouldEmit reports whether a single result should be emitted for the filter mode.
func ShouldEmit(result match.CandidateResult, mode FilterMode) bool {
	switch mode {
	case FilterAll:
		return true
	case FilterAbsentInAll:
		return result.AbsentInAll
	default:
		return false
	}
}
