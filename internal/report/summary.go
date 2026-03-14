package report

import "github.com/genestevens/domain-finder/internal/match"

// Summary captures deterministic counts for a classification/report run.
// TotalCandidates, PresentInAny, and AbsentInAll are computed from the full
// classified result set before filtering. EmittedResults is computed after filtering.
type Summary struct {
	TotalCandidates int
	EmittedResults  int
	PresentInAny    int
	AbsentInAll     int
}

// Summarize computes summary counts from all classified results and the emitted subset.
func Summarize(allResults, emittedResults []match.CandidateResult) Summary {
	summary := Summary{
		TotalCandidates: len(allResults),
		EmittedResults:  len(emittedResults),
	}

	for _, result := range allResults {
		if result.PresentInAny {
			summary.PresentInAny++
		}
		if result.AbsentInAll {
			summary.AbsentInAll++
		}
	}

	return summary
}
