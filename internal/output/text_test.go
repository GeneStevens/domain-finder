package output

import (
	"bytes"
	"testing"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	results := []match.CandidateResult{
		{
			Candidate:    "example",
			PresentInAny: true,
			AbsentInAll:  false,
			Zones: []match.ZonePresence{
				{Zone: "com", Present: true},
				{Zone: "net", Present: true},
			},
		},
		{
			Candidate:    "missing",
			PresentInAny: false,
			AbsentInAll:  true,
			Zones: []match.ZonePresence{
				{Zone: "com", Present: false},
				{Zone: "net", Present: false},
			},
		},
	}

	summary := report.Summary{
		TotalCandidates: 2,
		EmittedResults:  2,
		PresentInAny:    1,
		AbsentInAll:     1,
	}

	if err := WriteText(&buf, results, summary); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	want := "" +
		"example\n" +
		"  summary: present in at least one loaded zone\n" +
		"  com: present\n" +
		"  net: present\n" +
		"missing\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"summary\n" +
		"  total_candidates: 2\n" +
		"  emitted_results: 2\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 1\n"

	if buf.String() != want {
		t.Fatalf("WriteText() = %q, want %q", buf.String(), want)
	}
}

func TestWriteTextIncludesFilteredOutWhenApplicable(t *testing.T) {
	var buf bytes.Buffer
	summary := report.Summary{
		TotalCandidates: 3,
		EmittedResults:  1,
		PresentInAny:    2,
		AbsentInAll:     1,
	}

	if err := WriteTextSummary(&buf, summary); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	want := "" +
		"summary\n" +
		"  total_candidates: 3\n" +
		"  emitted_results: 1\n" +
		"  present_in_any: 2\n" +
		"  absent_in_all: 1\n" +
		"  filtered_out: 2\n"
	if buf.String() != want {
		t.Fatalf("WriteText() = %q, want %q", buf.String(), want)
	}
}

func TestWriteTextResult(t *testing.T) {
	var buf bytes.Buffer
	result := match.CandidateResult{
		Candidate:    "example",
		PresentInAny: true,
		AbsentInAll:  false,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: true},
		},
	}

	if err := WriteTextResult(&buf, result); err != nil {
		t.Fatalf("WriteTextResult() error = %v", err)
	}

	want := "" +
		"example\n" +
		"  summary: present in at least one loaded zone\n" +
		"  com: present\n" +
		"  net: present\n"
	if buf.String() != want {
		t.Fatalf("WriteTextResult() = %q, want %q", buf.String(), want)
	}
}
