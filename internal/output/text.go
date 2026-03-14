package output

import (
	"fmt"
	"io"

	"github.com/gene/domain-finder/internal/match"
	"github.com/gene/domain-finder/internal/report"
)

// WriteText renders candidate results in a human-readable deterministic format.
func WriteText(w io.Writer, results []match.CandidateResult, summary report.Summary) error {
	if _, err := fmt.Fprintf(
		w,
		"summary\n  total_candidates: %d\n  emitted_results: %d\n  present_in_any: %d\n  absent_in_all: %d\n",
		summary.TotalCandidates,
		summary.EmittedResults,
		summary.PresentInAny,
		summary.AbsentInAll,
	); err != nil {
		return err
	}

	if summary.EmittedResults != summary.TotalCandidates {
		if _, err := fmt.Fprintf(w, "  filtered_out: %d\n", summary.TotalCandidates-summary.EmittedResults); err != nil {
			return err
		}
	}

	for _, result := range results {
		if _, err := fmt.Fprintln(w, result.Candidate); err != nil {
			return err
		}

		summary := "present in at least one loaded zone"
		if result.AbsentInAll {
			summary = "absent in all loaded zones"
		}
		if _, err := fmt.Fprintf(w, "  summary: %s\n", summary); err != nil {
			return err
		}

		for _, zone := range result.Zones {
			status := "absent"
			if zone.Present {
				status = "present"
			}
			if _, err := fmt.Fprintf(w, "  %s: %s\n", zone.Zone, status); err != nil {
				return err
			}
		}
	}
	return nil
}
