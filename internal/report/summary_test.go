package report

import (
	"testing"

	"github.com/gene/domain-finder/internal/match"
)

func TestSummarize(t *testing.T) {
	all := []match.CandidateResult{
		{Candidate: "a.example", PresentInAny: true, AbsentInAll: false},
		{Candidate: "b.example", PresentInAny: false, AbsentInAll: true},
		{Candidate: "c.example", PresentInAny: false, AbsentInAll: true},
	}
	emitted := []match.CandidateResult{
		{Candidate: "b.example", PresentInAny: false, AbsentInAll: true},
		{Candidate: "c.example", PresentInAny: false, AbsentInAll: true},
	}

	got := Summarize(all, emitted)
	if got.TotalCandidates != 3 {
		t.Fatalf("TotalCandidates = %d, want 3", got.TotalCandidates)
	}
	if got.EmittedResults != 2 {
		t.Fatalf("EmittedResults = %d, want 2", got.EmittedResults)
	}
	if got.PresentInAny != 1 {
		t.Fatalf("PresentInAny = %d, want 1", got.PresentInAny)
	}
	if got.AbsentInAll != 2 {
		t.Fatalf("AbsentInAll = %d, want 2", got.AbsentInAll)
	}
}
