package report

import (
	"reflect"
	"testing"

	"github.com/gene/domain-finder/internal/match"
)

func TestParseFilterMode(t *testing.T) {
	mode, err := ParseFilterMode("absent-in-all")
	if err != nil {
		t.Fatalf("ParseFilterMode() error = %v", err)
	}
	if mode != FilterAbsentInAll {
		t.Fatalf("ParseFilterMode() = %q, want %q", mode, FilterAbsentInAll)
	}
}

func TestApplyFilterAbsentInAll(t *testing.T) {
	input := []match.CandidateResult{
		{Candidate: "present.example", AbsentInAll: false},
		{Candidate: "missing-a.example", AbsentInAll: true},
		{Candidate: "missing-b.example", AbsentInAll: true},
	}

	got := ApplyFilter(input, FilterAbsentInAll)
	want := []match.CandidateResult{
		{Candidate: "missing-a.example", AbsentInAll: true},
		{Candidate: "missing-b.example", AbsentInAll: true},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyFilter() = %#v, want %#v", got, want)
	}
}

func TestApplyFilterAllPreservesOrder(t *testing.T) {
	input := []match.CandidateResult{
		{Candidate: "first.example"},
		{Candidate: "second.example"},
	}

	got := ApplyFilter(input, FilterAll)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("ApplyFilter() = %#v, want %#v", got, input)
	}
}
