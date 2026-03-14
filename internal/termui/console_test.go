package termui

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

func TestConsoleRendersHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"example", "missing"})

	if err := console.Start(2, report.FilterAbsentInAll); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := console.UpdateActive(1, 2, "example"); err != nil {
		t.Fatalf("UpdateActive() error = %v", err)
	}
	if err := console.EmitRow(match.CandidateResult{
		Candidate: "missing",
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}); err != nil {
		t.Fatalf("EmitRow() error = %v", err)
	}
	if err := console.Finish(report.Summary{TotalCandidates: 2, EmittedResults: 1}); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got := buf.String()
	wantFragments := []string{
		"Zone files loaded: COM, NET\n",
		"Searching 2 domains | filter: absent-in-all\n",
		"candidate",
		"> [1/2] example",
		"  missing",
		"Done: checked 2, emitted 1\n",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("console output missing %q:\n%s", fragment, got)
		}
	}
}

func TestShouldUseInteractive(t *testing.T) {
	neverTTY := func(io.Writer) bool { return false }
	if ShouldUseInteractive("jsonl", true, false, nil, neverTTY) {
		t.Fatal("ShouldUseInteractive(jsonl) = true, want false")
	}
	if !ShouldUseInteractive("text", true, false, nil, neverTTY) {
		t.Fatal("ShouldUseInteractive(forceOn) = false, want true")
	}
	if ShouldUseInteractive("text", true, true, nil, neverTTY) {
		t.Fatal("ShouldUseInteractive(forceOff) = true, want false")
	}
}
