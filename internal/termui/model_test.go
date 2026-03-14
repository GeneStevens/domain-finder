package termui

import (
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

func TestInteractiveModelUpdatesStatusAndChecking(t *testing.T) {
	model := newInteractiveModel([]string{"com", "net"}, []string{"vortex"}, false, false, nil)

	updated, _ := model.Update(startMsg{total: 25, filter: report.FilterAll})
	model = updated.(interactiveModel)
	updated, _ = model.Update(statusMsg{line: "generation: batch 2 attempt 1 accepted 1"})
	model = updated.(interactiveModel)
	updated, _ = model.Update(activeMsg{index: 8, total: 25, candidate: "vortex"})
	model = updated.(interactiveModel)

	view := model.View()
	for _, fragment := range []string{
		"Zone files loaded: COM, NET",
		"Searching 25 stems | filter: all",
		"generation: batch 2 attempt 1 accepted 1 | checking: vortex... [8/25]",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("view missing %q:\n%s", fragment, view)
		}
	}
}

func TestInteractiveModelKeepsStrongHitsDurableByDefault(t *testing.T) {
	model := newInteractiveModel([]string{"com", "net"}, []string{"strongstem", "partialstem"}, false, false, nil)

	updated, _ := model.Update(rowMsg{result: match.CandidateResult{
		Candidate:   "strongstem",
		AbsentInAll: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}})
	model = updated.(interactiveModel)

	updated, _ = model.Update(rowMsg{result: match.CandidateResult{
		Candidate:    "partialstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: true},
		},
	}})
	model = updated.(interactiveModel)

	view := model.View()
	if !strings.Contains(view, "strongstem") || !strings.Contains(view, "all ✓") {
		t.Fatalf("view = %q, want durable strong hit", view)
	}
	if strings.Contains(view, "partialstem") {
		t.Fatalf("view = %q, want partial hit suppressed by default", view)
	}
}

func TestInteractiveModelRendersFinalSummaries(t *testing.T) {
	model := newInteractiveModel([]string{"com"}, []string{"missing"}, false, false, nil)

	updated, _ := model.Update(noteMsg{line: "generation diagnostics"})
	model = updated.(interactiveModel)
	updated, _ = model.Update(noteMsg{line: "  quality.vowel_heavy: 3"})
	model = updated.(interactiveModel)
	updated, cmd := model.Update(finishMsg{summary: report.Summary{TotalCandidates: 9, EmittedResults: 2, AbsentInAll: 2}})
	model = updated.(interactiveModel)

	if cmd == nil {
		t.Fatal("finish update returned nil cmd, want tea.Quit")
	}

	view := model.View()
	for _, fragment := range []string{
		"generation diagnostics",
		"  quality.vowel_heavy: 3",
		"Done: checked 9 | emitted 2 | strong 2",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("view missing %q:\n%s", fragment, view)
		}
	}
}

func TestInteractiveModelTranscriptPreservesDurableContent(t *testing.T) {
	model := newInteractiveModel([]string{"com", "net"}, []string{"strongstem"}, false, false, nil)

	updated, _ := model.Update(rowMsg{result: match.CandidateResult{
		Candidate:   "strongstem",
		AbsentInAll: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}})
	model = updated.(interactiveModel)
	updated, _ = model.Update(noteMsg{line: "generation diagnostics"})
	model = updated.(interactiveModel)
	updated, _ = model.Update(finishMsg{summary: report.Summary{TotalCandidates: 5, EmittedResults: 1, AbsentInAll: 1}})
	model = updated.(interactiveModel)

	transcript := model.Transcript()
	for _, fragment := range []string{
		"stem",
		"strongstem",
		"COM NET",
		"all ✓",
		"generation diagnostics",
		"Done: checked 5 | emitted 1 | strong 1",
	} {
		if !strings.Contains(transcript, fragment) {
			t.Fatalf("transcript missing %q:\n%s", fragment, transcript)
		}
	}
	if strings.Contains(transcript, "checking:") || strings.Contains(transcript, "generation: batch") {
		t.Fatalf("transcript = %q, want only durable content", transcript)
	}
}
