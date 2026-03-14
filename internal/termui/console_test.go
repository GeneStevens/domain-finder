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
	console := NewConsole(&buf, []string{"com", "net"}, []string{"example", "missing"}, false, false, true)

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

func TestConsoleFormatsPartialRowsClearlyAndSuppressesTakenRows(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"partialstem", "takenstem"}, false, false, true)

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
	if strings.Contains(got, "takenstem") || strings.Contains(got, "(none)") || strings.Contains(got, "taken") {
		t.Fatalf("taken row = %q, want taken rows suppressed from durable interactive output", got)
	}
}

func TestConsoleAdaptsColumnWidths(t *testing.T) {
	candWidth, zoneWidth, statusWidth := consoleColumnWidths([]string{"com", "network", "org"}, []string{"verylongstemname"})

	if candWidth < len("verylongstemname") {
		t.Fatalf("candWidth = %d, want width for longest stem", candWidth)
	}
	if zoneWidth < len("COM NETWORK ORG") {
		t.Fatalf("zoneWidth = %d, want width for joined zones", zoneWidth)
	}
	if statusWidth < len("partial") {
		t.Fatalf("statusWidth = %d, want width for status labels", statusWidth)
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
	console := NewConsole(&buf, []string{"com", "net"}, []string{"missing"}, true, false, false)

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
	console := NewConsole(&buf, []string{"com", "net"}, []string{"takenstem", "partialstem", "strongstem"}, false, true, true)

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

func TestConsoleDefaultsToStrongHitsOnly(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"partialstem", "strongstem"}, false, false, false)

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

	if console.ShouldEmitRow(partial) {
		t.Fatal("ShouldEmitRow(partial) = true, want false by default")
	}
	if !console.ShouldEmitRow(strong) {
		t.Fatal("ShouldEmitRow(strong) = false, want true")
	}
}

func TestConsoleUpdateStatusUsesEphemeralLine(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"missing"}, false, false, false)

	if err := console.UpdateStatus("generation: batch 1 attempt 1 requesting 2 stems"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err := console.UpdateActive(1, 1, "missing"); err != nil {
		t.Fatalf("UpdateActive() error = %v", err)
	}
	if err := console.Note("generation diagnostics"); err != nil {
		t.Fatalf("Note() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\r") {
		t.Fatalf("console output = %q, want carriage-return based ephemeral updates", got)
	}
	if !strings.Contains(got, "generation diagnostics\n") {
		t.Fatalf("console output = %q, want durable summary note", got)
	}
	if strings.Contains(got, "\ngeneration: batch 1 attempt 1 requesting 2 stems") {
		t.Fatalf("console output = %q, want status line to stay ephemeral instead of becoming a durable note", got)
	}
}

func TestConsoleLiveUpdatesReplaceWholeLine(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"vortex"}, false, false, false)

	if err := console.UpdateStatus("generation: batch 1 attempt 1 accepted 0"); err != nil {
		t.Fatalf("UpdateStatus(first) error = %v", err)
	}
	if err := console.UpdateActive(8, 20, "vortex"); err != nil {
		t.Fatalf("UpdateActive() error = %v", err)
	}
	if err := console.UpdateStatus("generation: batch 2 attempt 1 accepted 1"); err != nil {
		t.Fatalf("UpdateStatus(second) error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "[8generation:") {
		t.Fatalf("console output = %q, want canonical redraws instead of concatenated live fragments", got)
	}
	want := "\rgeneration: batch 1 attempt 1 accepted 0" +
		"\rgeneration: batch 1 attempt 1 accepted 0 | checking: vortex... [8/20]" +
		"\rgeneration: batch 2 attempt 1 accepted 1 | checking: vortex... [8/20]"
	if got != want {
		t.Fatalf("console output = %q, want %q", got, want)
	}
}

func TestConsoleClearsPaddingWhenLiveLineGetsShorter(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com"}, []string{"verylongcandidate"}, false, false, false)

	if err := console.UpdateStatus("generation: batch 12 attempt 3 accepted 10, duplicates 4"); err != nil {
		t.Fatalf("UpdateStatus(long) error = %v", err)
	}
	if err := console.UpdateStatus("generation: batch 13"); err != nil {
		t.Fatalf("UpdateStatus(short) error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\rgeneration: batch 13") {
		t.Fatalf("console output = %q, want shorter live line rendered", got)
	}
	if !strings.Contains(got, "batch 13                                    ") {
		t.Fatalf("console output = %q, want shorter line padded to clear leftovers", got)
	}
}

func TestConsoleDurableOutputClearsLiveLineFirst(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com", "net"}, []string{"missing"}, false, false, false)

	if err := console.UpdateStatus("generation: batch 1 attempt 1 accepted 1"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err := console.UpdateActive(1, 1, "missing"); err != nil {
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

	got := buf.String()
	if !strings.Contains(got, "... [1/1]\r") {
		t.Fatalf("console output = %q, want live line cleared after the active render", got)
	}
	if !strings.Contains(got, "\rmissing") {
		t.Fatalf("console output = %q, want live line cleared before durable row", got)
	}
	if strings.Contains(got, "checking: missing... [1/1]\nmissing") {
		t.Fatalf("console output = %q, want durable row separated from live line", got)
	}
}

func TestConsoleFinishClearsLiveLineBeforeSummary(t *testing.T) {
	var buf bytes.Buffer
	console := NewConsole(&buf, []string{"com"}, []string{"missing"}, false, false, false)

	if err := console.UpdateStatus("generation: batch 1 attempt 1 accepted 1"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err := console.Finish(report.Summary{TotalCandidates: 1, EmittedResults: 1, AbsentInAll: 1}); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "accepted 1\r") {
		t.Fatalf("console output = %q, want active live line before finish clear", got)
	}
	if !strings.Contains(got, "\rDone: checked 1 | emitted 1 | strong 1\n") {
		t.Fatalf("console output = %q, want final summary to follow a cleared live line", got)
	}
}
