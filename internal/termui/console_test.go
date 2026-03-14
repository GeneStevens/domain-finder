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
	console := NewConsole(&buf, []string{"com", "net"}, []string{"example", "missing"}, false, false)

	if err := console.Start(2, report.FilterAbsentInAll); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := console.UpdateActive(1, 2, "example"); err != nil {
		t.Fatalf("UpdateActive() error = %v", err)
	}
	if err := console.EmitRow(match.CandidateResult{
		Candidate:   "missing",
		AbsentInAll: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}); err != nil {
		t.Fatalf("EmitRow() error = %v", err)
	}
	if err := console.Finish(report.Summary{TotalCandidates: 2, EmittedResults: 1, AbsentInAll: 1}); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got := buf.String()
	wantFragments := []string{
		"Zone files loaded: COM, NET\n",
		"Searching 2 stems | filter: absent-in-all\n",
		"stem",
		"available_zones",
		"result",
		"checking: example... [1/2]",
		"missing",
		"COM NET",
		"all ✓",
		"Done: checked 2 | emitted 1 | strong 1\n",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("console output missing %q:\n%s", fragment, got)
		}
	}
}

func TestConsoleFormatsPartialAndTakenRowsClearly(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"partialstem", "takenstem"}, false, false)

	if err := console.EmitRow(match.CandidateResult{
		Candidate:    "partialstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: true},
		},
	}); err != nil {
		t.Fatalf("EmitRow(partial) error = %v", err)
	}
	if err := console.EmitRow(match.CandidateResult{
		Candidate:    "takenstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: true},
		},
	}); err != nil {
		t.Fatalf("EmitRow(taken) error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "partialstem") || !strings.Contains(got, "COM") || !strings.Contains(got, "partial") {
		t.Fatalf("partial row = %q, want clear available-zone semantics", got)
	}
	if !strings.Contains(got, "takenstem") || !strings.Contains(got, "(none)") || !strings.Contains(got, "taken") {
		t.Fatalf("taken row = %q, want explicit no-availability semantics", got)
	}
}

func TestConsoleAdaptsColumnWidths(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "network", "org"}, []string{"verylongstemname"}, false, false)

	if console.candWidth < len("verylongstemname") {
		t.Fatalf("candWidth = %d, want width for longest stem", console.candWidth)
	}
	if console.zoneWidth < len("COM NETWORK ORG") {
		t.Fatalf("zoneWidth = %d, want width for joined zones", console.zoneWidth)
	}
	if console.statusWidth < len("partial") {
		t.Fatalf("statusWidth = %d, want width for status labels", console.statusWidth)
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

func TestShouldUseColor(t *testing.T) {
	neverTTY := func(io.Writer) bool { return false }
	if ShouldUseColor(false, false, nil, neverTTY) {
		t.Fatal("ShouldUseColor(auto non-tty) = true, want false")
	}
	if !ShouldUseColor(true, false, nil, neverTTY) {
		t.Fatal("ShouldUseColor(forceOn) = false, want true")
	}
	if ShouldUseColor(true, true, nil, neverTTY) {
		t.Fatal("ShouldUseColor(forceOff) = true, want false")
	}
}

func TestConsoleStylesStrongHitsWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"missing"}, true, false)

	if err := console.EmitRow(match.CandidateResult{
		Candidate:   "missing",
		AbsentInAll: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}); err != nil {
		t.Fatalf("EmitRow() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "missing") || !strings.Contains(got, "\x1b[1;97;42mall ✓\x1b[0m") {
		t.Fatalf("styled row = %q, want ANSI strong-hit emphasis", got)
	}
}

func TestConsoleCanSuppressTakenRows(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"takenstem", "partialstem", "strongstem"}, false, true)

	taken := match.CandidateResult{
		Candidate:    "takenstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: true},
		},
	}
	partial := match.CandidateResult{
		Candidate:    "partialstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: true},
		},
	}
	strong := match.CandidateResult{
		Candidate:   "strongstem",
		AbsentInAll: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: false},
			{Zone: "net", Present: false},
		},
	}

	if console.ShouldEmitRow(taken) {
		t.Fatal("ShouldEmitRow(taken) = true, want false when hideTaken is enabled")
	}
	if !console.ShouldEmitRow(partial) {
		t.Fatal("ShouldEmitRow(partial) = false, want true")
	}
	if !console.ShouldEmitRow(strong) {
		t.Fatal("ShouldEmitRow(strong) = false, want true")
	}

	if err := console.EmitRow(taken); err != nil {
		t.Fatalf("EmitRow(taken) error = %v", err)
	}
	if err := console.EmitRow(partial); err != nil {
		t.Fatalf("EmitRow(partial) error = %v", err)
	}
	if err := console.EmitRow(strong); err != nil {
		t.Fatalf("EmitRow(strong) error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "takenstem") {
		t.Fatalf("console output = %q, want taken row suppressed", got)
	}
	if !strings.Contains(got, "partialstem") || !strings.Contains(got, "partial") {
		t.Fatalf("console output = %q, want partial row preserved", got)
	}
	if !strings.Contains(got, "strongstem") || !strings.Contains(got, "all ✓") {
		t.Fatalf("console output = %q, want strong row preserved", got)
	}
}
