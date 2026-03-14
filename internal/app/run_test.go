package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/audit"
	"github.com/genestevens/domain-finder/internal/backend"
	"github.com/genestevens/domain-finder/internal/config"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/openai"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

type fakeStemGenerator struct {
	responses []fakeStemResponse
	calls     []int
	model     string
}

type fakeStemResponse struct {
	result openai.BatchResult
	err    error
}

type fakeLookup struct {
	zones   []string
	present map[string]bool
}

func (f fakeLookup) ZoneNames() []string {
	return append([]string(nil), f.zones...)
}

func (f fakeLookup) Contains(_ context.Context, zone, stem string) (bool, error) {
	return f.present[zone+"/"+stem], nil
}

func (f *fakeStemGenerator) GenerateBatch(_ context.Context, _ string, count int) (openai.BatchResult, error) {
	f.calls = append(f.calls, count)
	if len(f.responses) == 0 {
		return openai.BatchResult{}, fmt.Errorf("unexpected GenerateBatch call")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.result, response.err
}

func (f *fakeStemGenerator) ModelName() string {
	if f.model != "" {
		return f.model
	}
	return "gpt-4o-mini"
}

func TestRunTextWorkflowFallsBackWhenNotInteractive(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
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
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty fallback mode", stderr.String())
	}
}

func TestRunPostgresBackendWorkflow(t *testing.T) {
	originalOpenPostgresBackend := openPostgresBackend
	defer func() {
		openPostgresBackend = originalOpenPostgresBackend
	}()

	var gotDSN string
	var gotZones []string
	openPostgresBackend = func(dsn string, zones []string) (backend.Lookup, io.Closer, error) {
		gotDSN = dsn
		gotZones = append([]string(nil), zones...)
		return fakeLookup{
			zones: zones,
			present: map[string]bool{
				"com/example": true,
				"net/example": true,
			},
		}, nil, nil
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("postgres:\n  dsn: postgres://yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalGetWorkingDir := getWorkingDir
	defer func() { getWorkingDir = originalGetWorkingDir }()
	getWorkingDir = func() (string, error) { return dir, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-backend", "postgres",
		"-no-interactive",
		"-zone", "net",
		"-zone", "com",
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotDSN != "postgres://yaml" {
		t.Fatalf("openPostgresBackend dsn = %q, want postgres://yaml", gotDSN)
	}
	if !reflect.DeepEqual(gotZones, []string{"com", "net"}) {
		t.Fatalf("openPostgresBackend zones = %#v, want [com net]", gotZones)
	}
	if !strings.Contains(stdout.String(), "example\n") || !strings.Contains(stdout.String(), "missing\n") {
		t.Fatalf("stdout = %q, want normal deterministic result output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty fallback mode", stderr.String())
	}
}

func TestRunBackendSelectionValidation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-backend", "postgres",
		"-zone", "com=testdata/small/com.zone",
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "postgres backend requires -zone zone-name") {
		t.Fatalf("Run() error = %v, want postgres zone validation", err)
	}

	err = Run([]string{
		"-backend", "file",
		"-zone", "com",
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "file backend requires -zone zone=path") {
		t.Fatalf("Run() error = %v, want file zone validation", err)
	}

	err = Run([]string{
		"-backend", "postgres",
		"-zone", "com",
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "postgres backend requires a DSN") {
		t.Fatalf("Run() error = %v, want postgres DSN validation", err)
	}
}

func TestRunTextWorkflowInteractiveOverride(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout without -out", stdout.String())
	}

	progress := stderr.String()
	wantProgress := []string{
		"Zone files loaded: COM, NET\n",
		"Searching 2 stems | filter: all\n",
		"available_zones",
		"result",
		"checking: example... [1/2]",
		"checking: missing... [2/2]",
		"example",
		"(none)",
		"taken",
		"missing",
		"COM NET",
		"all ✓",
		"Done: checked 2 | emitted 2 | strong 1\n",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunTextWorkflowInteractiveCanHideTakenRows(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-interactive",
		"-interactive-hide-taken",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout without -out", stdout.String())
	}

	progress := stderr.String()
	if strings.Contains(progress, "example  (none)") || strings.Contains(progress, "taken") {
		t.Fatalf("stderr = %q, want taken rows suppressed", progress)
	}
	wantFragments := []string{
		"checking: example... [1/2]",
		"checking: missing... [2/2]",
		"missing",
		"COM NET",
		"all ✓",
		"Done: checked 2 | emitted 2 | strong 1\n",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunInteractiveAuditLogIncludesSuppressedTakenRows(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "run.jsonl")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-interactive-hide-taken",
		"-audit-log", auditPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if strings.Contains(stderr.String(), "example  (none)") || strings.Contains(stderr.String(), "taken") {
		t.Fatalf("stderr = %q, want taken row suppressed from interactive tape", stderr.String())
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("audit lines = %d, want 2 records", len(lines))
	}

	var got []audit.Record
	for _, line := range lines {
		var record audit.Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", line, err)
		}
		got = append(got, record)
	}

	if got[0].Stem != "example" || got[0].State != audit.StateTaken || got[0].ReportEmitted != true || got[0].InteractiveEmitted != false {
		t.Fatalf("got[0] = %#v, want taken record preserved but not interactively emitted", got[0])
	}
	if got[1].Stem != "missing" || got[1].State != audit.StateAll || got[1].InteractiveEmitted != true {
		t.Fatalf("got[1] = %#v, want strong emitted record", got[1])
	}
	if !reflect.DeepEqual(got[0].RequestedZones, []string{"com", "net"}) {
		t.Fatalf("requested_zones = %#v, want [com net]", got[0].RequestedZones)
	}
}

func TestRunTextWorkflowWithCandidateFileInteractive(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-interactive",
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "" +
		""
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want empty interactive stdout", stdout.String())
	}

	progress := stderr.String()
	wantProgress := []string{
		"Searching 3 stems | filter: absent-in-all\n",
		"available_zones",
		"result",
		"checking: missing... [1/3]",
		"checking: example... [2/3]",
		"checking: mixedcase... [3/3]",
		"missing",
		"COM NET",
		"all ✓",
		"Done: checked 3 | emitted 1 | strong 1\n",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunTextWorkflowNoInteractiveOverride(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-interactive",
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no interactive output when disabled", stderr.String())
	}
	if !strings.Contains(stdout.String(), "summary\n") {
		t.Fatalf("stdout = %q, want deterministic text report", stdout.String())
	}
}

func TestRunInteractiveColorOverride(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-interactive",
		"-color",
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing") || !strings.Contains(stderr.String(), "\x1b[1;97;42mall ✓\x1b[0m") {
		t.Fatalf("stderr = %q, want ANSI strong-hit styling", stderr.String())
	}
}

func TestRunJSONLWorkflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-format", "jsonl",
		"-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got match.CandidateResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Candidate != "example" || !got.PresentInAny || got.AbsentInAll {
		t.Fatalf("got = %#v, want present example result", got)
	}
	if len(got.Zones) != 2 || got.Zones[0].Zone != "com" || got.Zones[1].Zone != "net" {
		t.Fatalf("zones = %#v, want deterministic order", got.Zones)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in jsonl mode", stderr.String())
	}
}

func TestRunNonInteractiveAuditLog(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "run.jsonl")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-audit-log", auditPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "summary\n") {
		t.Fatalf("stdout = %q, want normal deterministic output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty fallback stderr", stderr.String())
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("audit lines = %d, want 2 records", len(lines))
	}
	var record audit.Record
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record.Backend != "file" || record.InteractiveEmitted {
		t.Fatalf("record = %#v, want file backend and non-interactive emission=false", record)
	}
}

func TestRunNonInteractiveRunSummary(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "run-summary.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-run-summary", summaryPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "summary\n") {
		t.Fatalf("stdout = %q, want normal deterministic output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty fallback stderr", stderr.String())
	}

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, string(data))
	}
	if got["backend"] != "file" || got["filter_mode"] != "all" || got["interactive"] != false {
		t.Fatalf("summary = %#v, want stable manual summary fields", got)
	}
	if got["total_checked_stems"] != float64(2) || got["emitted_results"] != float64(2) || got["strong_hits"] != float64(1) {
		t.Fatalf("summary counts = %#v, want total 2 emitted 2 strong 1", got)
	}
	if _, ok := got["generation"]; ok {
		t.Fatalf("generation = %#v, want omitted for manual-only run", got["generation"])
	}
}

func TestRunAuditLogIncludesFilteredOutStem(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "run.jsonl")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-filter", "absent-in-all",
		"-audit-log", auditPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("audit lines = %d, want 2 records", len(lines))
	}

	var example audit.Record
	var missing audit.Record
	for _, line := range lines {
		var record audit.Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", line, err)
		}
		switch record.Stem {
		case "example":
			example = record
		case "missing":
			missing = record
		}
	}

	if example.Stem != "example" || example.ReportEmitted || example.InteractiveEmitted {
		t.Fatalf("example record = %#v, want filtered-out stem still logged but not emitted", example)
	}
	if missing.Stem != "missing" || !missing.ReportEmitted || !missing.InteractiveEmitted {
		t.Fatalf("missing record = %#v, want emitted strong hit", missing)
	}
	if !strings.Contains(stderr.String(), "missing") || strings.Contains(stderr.String(), "example  (none)") {
		t.Fatalf("stderr = %q, want only emitted interactive row", stderr.String())
	}
}

func TestRunSummaryCoexistsWithAuditLog(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "run.jsonl")
	summaryPath := filepath.Join(dir, "run-summary.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-audit-log", auditPath,
		"-run-summary", summaryPath,
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
		"-candidate", "missing",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout", stdout.String())
	}

	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile(audit) error = %v", err)
	}
	if len(strings.Split(strings.TrimSpace(string(auditData)), "\n")) != 2 {
		t.Fatalf("audit = %q, want 2 records", string(auditData))
	}

	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("ReadFile(summary) error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(summaryData, &got); err != nil {
		t.Fatalf("json.Unmarshal(summary) error = %v\n%s", err, string(summaryData))
	}
	if got["interactive"] != true || got["emitted_results"] != float64(1) {
		t.Fatalf("summary = %#v, want interactive emitted summary", got)
	}
}

func TestRunGenerateDryRunRejectsAuditLog(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
		"-audit-log", "run.jsonl",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "-audit-log cannot be used with -generate-dry-run") {
		t.Fatalf("Run() error = %v, want audit/dry-run validation", err)
	}
}

func TestRunGenerateDryRunRejectsRunSummary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
		"-run-summary", "run-summary.json",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "-run-summary cannot be used with -generate-dry-run") {
		t.Fatalf("Run() error = %v, want run-summary/dry-run validation", err)
	}
}

func TestRunTextWorkflowToFileInteractive(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "results.txt")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-out", outPath,
		"-filter", "absent-in-all",
		"-candidate-stdin",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader("missing\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when writing to file", stdout.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	wantFile := "" +
		"missing\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"summary\n" +
		"  total_candidates: 3\n" +
		"  emitted_results: 1\n" +
		"  present_in_any: 2\n" +
		"  absent_in_all: 1\n" +
		"  filtered_out: 2\n"
	if string(data) != wantFile {
		t.Fatalf("file output = %q, want %q", string(data), wantFile)
	}
	if !strings.Contains(stderr.String(), "Done: checked 3 | emitted 1 | strong 1\n") {
		t.Fatalf("stderr = %q, want interactive completion", stderr.String())
	}
}

func TestRunTextWorkflowWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 4\n  batch_size: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		model: "gpt-4o-mini",
		responses: []fakeStemResponse{
			{result: openai.BatchResult{
				Stems: []string{"brandfoo", "noviq"},
				Usage: &openai.Usage{InputTokens: 120, OutputTokens: 18, CachedInputTokens: 40},
			}},
			{result: openai.BatchResult{
				Stems: []string{"example", "trynex"},
				Usage: &openai.Usage{InputTokens: 80, OutputTokens: 12},
			}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "" +
		"missing\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"brandfoo\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"noviq\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"example\n" +
		"  summary: present in at least one loaded zone\n" +
		"  com: present\n" +
		"  net: present\n" +
		"trynex\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"summary\n" +
		"  total_candidates: 5\n" +
		"  emitted_results: 5\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 4\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	progress := stderr.String()
	wantProgress := []string{
		"generation: batch 1 attempt 1 requesting 2 stems",
		"generation: batch 1 attempt 1 accepted 2, invalid 0, banned 0, quality_rejected 0, duplicates 0, need 0 more | last $0.000026 | total $0.000026",
		"generation: batch 2 attempt 1 requesting 2 stems",
		"generation: batch 2 attempt 1 accepted 2, invalid 0, banned 0, quality_rejected 0, duplicates 0, need 0 more | last $0.000019 | total $0.000045",
		"generation: complete, accepted 4 stems | total $0.000045",
		"generation usage",
		"  model: gpt-4o-mini",
		"  input_tokens: 200",
		"  output_tokens: 30",
		"  cached_input_tokens: 40",
		"  estimated_cost_usd: $0.000045",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
	if len(generator.calls) != 2 || generator.calls[0] != 2 || generator.calls[1] != 2 {
		t.Fatalf("generator calls = %#v, want [2 2]", generator.calls)
	}
}

func TestRunTextWorkflowInteractiveWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 3\n  batch_size: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"brandfoo", "example"}}},
			{result: openai.BatchResult{Stems: []string{"noviq"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout without -out", stdout.String())
	}

	progress := stderr.String()
	wantProgress := []string{
		"Searching 4 stems | filter: absent-in-all\n",
		"generation: batch 1 attempt 1 requesting 2 stems",
		"generation: batch 1 attempt 1 accepted 2, invalid 0, banned 0, quality_rejected 0, duplicates 0, need 0 more",
		"generation: batch 2 attempt 1 requesting 1 stems",
		"generation: batch 2 attempt 1 accepted 1, invalid 0, banned 0, quality_rejected 0, duplicates 0, need 0 more",
		"generation: complete, accepted 3 stems",
		"checking: missing... [1/4]",
		"checking: brandfoo... [2/4]",
		"checking: example... [3/4]",
		"checking: noviq... [4/4]",
		"missing",
		"brandfoo",
		"noviq",
		"Done: checked 4 | emitted 3 | strong 3\n",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunGenerationConstraintsFlowIntoResolvedConfig(t *testing.T) {
	dir := t.TempDir()
	configBody := "" +
		"generate:\n" +
		"  count: 4\n" +
		"  batch_size: 2\n" +
		"  quality_profile: industrial\n" +
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n" +
		"  avoid_substrings: dev,cloud\n" +
		"  avoid_prefixes: dev,neo\n" +
		"  avoid_suffixes: ia,ora\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"shieldr", "trynex"}}},
			{result: openai.BatchResult{Stems: []string{"noviq", "tractix"}}},
			{result: openai.BatchResult{Stems: []string{"secbase"}}},
		},
	}

	var captured config.GenerateConfig
	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(cfg config.Config) (openai.StemGenerator, error) {
		captured = cfg.Generate
		return generator, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-generate", "invented security stems",
		"-generate-count", "5",
		"-generate-max-length", "12",
		"-generate-prefix", "sec",
		"-generate-style", "invented SaaS",
		"-generate-avoid-prefixes", "sys,neo",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if captured.Count != 5 {
		t.Fatalf("captured.Count = %d, want 5", captured.Count)
	}
	if captured.BatchSize != 2 {
		t.Fatalf("captured.BatchSize = %d, want 2 from config", captured.BatchSize)
	}
	if captured.QualityProfile != "industrial" {
		t.Fatalf("captured.QualityProfile = %q, want industrial from config", captured.QualityProfile)
	}
	if captured.MaxLength != 12 {
		t.Fatalf("captured.MaxLength = %d, want 12 from CLI", captured.MaxLength)
	}
	if captured.MaxSyllables != 2 {
		t.Fatalf("captured.MaxSyllables = %d, want 2 from config", captured.MaxSyllables)
	}
	if captured.Prefix != "sec" {
		t.Fatalf("captured.Prefix = %q, want sec from CLI", captured.Prefix)
	}
	if captured.Suffix != "ix" {
		t.Fatalf("captured.Suffix = %q, want ix from config", captured.Suffix)
	}
	if captured.Style != "invented SaaS" {
		t.Fatalf("captured.Style = %q, want invented SaaS from CLI", captured.Style)
	}
	if got := strings.Join(captured.AvoidSubstrings, ","); got != "dev,cloud" {
		t.Fatalf("captured.AvoidSubstrings = %q, want dev,cloud from config", got)
	}
	if got := strings.Join(captured.AvoidPrefixes, ","); got != "sys,neo" {
		t.Fatalf("captured.AvoidPrefixes = %q, want sys,neo from CLI", got)
	}
	if got := strings.Join(captured.AvoidSuffixes, ","); got != "ia,ora" {
		t.Fatalf("captured.AvoidSuffixes = %q, want ia,ora from config", got)
	}
	if len(generator.calls) != 3 || generator.calls[0] != 2 || generator.calls[1] != 2 || generator.calls[2] != 1 {
		t.Fatalf("generator calls = %#v, want [2 2 1]", generator.calls)
	}
	if !strings.Contains(stderr.String(), "generation: complete, accepted 5 stems") {
		t.Fatalf("stderr = %q, want generation completion", stderr.String())
	}
}

func TestRunGenerateDryRunDoesNotRequireAPIKey(t *testing.T) {
	dir := t.TempDir()
	configBody := "" +
		"openai:\n" +
		"  model: yaml-model\n" +
		"generate:\n" +
		"  count: 4\n" +
		"  batch_size: 2\n" +
		"  max_attempts: 3\n" +
		"  retry_count: 1\n" +
		"  quality_profile: industrial\n" +
		"  max_length: 10\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n" +
		"  avoid_substrings: dev,cloud\n" +
		"  avoid_prefixes: dev,neo\n" +
		"  avoid_suffixes: ia,ora\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) {
		t.Fatal("newStemGenerator should not be called during dry run")
		return nil, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty dry-run stderr", stderr.String())
	}

	out := stdout.String()
	wantFragments := []string{
		"generation dry run",
		"model: yaml-model",
		"generate_count: 4",
		"batch_size: 2",
		"max_attempts: 3",
		"retry_count: 1",
		"quality_profile: industrial",
		"theme: short product stems",
		"style: security product",
		"max_length: 10",
		"max_syllables: 2",
		"prefix: neo",
		"suffix: ix",
		"avoid_substrings: dev, cloud",
		"avoid_prefixes: dev, neo",
		"avoid_suffixes: ia, ora",
		"system prompt",
		"user prompt",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(out, fragment) {
			t.Fatalf("dry run output missing %q:\n%s", fragment, out)
		}
	}
}

func TestRunGenerateDryRunReflectsCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	configBody := "" +
		"openai:\n" +
		"  model: yaml-model\n" +
		"generate:\n" +
		"  count: 4\n" +
		"  batch_size: 2\n" +
		"  quality_profile: industrial\n" +
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n" +
		"  avoid_substrings: dev,cloud\n" +
		"  avoid_prefixes: dev,neo\n" +
		"  avoid_suffixes: ia,ora\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) {
		t.Fatal("newStemGenerator should not be called during dry run")
		return nil, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
		"-generate-model", "cli-model",
		"-generate-count", "8",
		"-generate-batch-size", "4",
		"-generate-quality-profile", "off",
		"-generate-max-length", "12",
		"-generate-max-syllables", "3",
		"-generate-prefix", "dev",
		"-generate-suffix", "io",
		"-generate-style", "developer tool",
		"-generate-avoid-substrings", "stack,forge,cloud",
		"-generate-avoid-prefixes", "dev,neo",
		"-generate-avoid-suffixes", "ia,ora",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	wantFragments := []string{
		"model: cli-model",
		"generate_count: 8",
		"batch_size: 4",
		"quality_profile: off",
		"max_length: 12",
		"max_syllables: 3",
		"prefix: dev",
		"suffix: io",
		"style: developer tool",
		"avoid_substrings: stack, forge, cloud",
		"avoid_prefixes: dev, neo",
		"avoid_suffixes: ia, ora",
		"start with `dev`",
		"end with `io`",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(out, fragment) {
			t.Fatalf("dry run output missing %q:\n%s", fragment, out)
		}
	}
}

func TestRunGenerateDryRunJSONOutput(t *testing.T) {
	dir := t.TempDir()
	configBody := "" +
		"openai:\n" +
		"  model: yaml-model\n" +
		"generate:\n" +
		"  count: 4\n" +
		"  batch_size: 2\n" +
		"  max_attempts: 3\n" +
		"  retry_count: 1\n" +
		"  quality_profile: industrial\n" +
		"  max_length: 10\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n" +
		"  avoid_prefixes: dev,neo\n" +
		"  avoid_suffixes: ia,ora\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) {
		t.Fatal("newStemGenerator should not be called during dry run")
		return nil, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
		"-generate-dry-run-format", "json",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, stdout.String())
	}
	if got["model"] != "yaml-model" {
		t.Fatalf("model = %#v, want yaml-model", got["model"])
	}
	if got["generate_count"] != float64(4) || got["batch_size"] != float64(2) {
		t.Fatalf("counts = %#v, want 4/2", got)
	}
	if got["theme"] != "short product stems" || got["style"] != "security product" {
		t.Fatalf("theme/style = %#v, want resolved prompt values", got)
	}
	if got["quality_profile"] != "industrial" {
		t.Fatalf("quality_profile = %#v, want industrial", got["quality_profile"])
	}
	constraints, ok := got["constraints"].(map[string]any)
	if !ok {
		t.Fatalf("constraints = %#v, want object", got["constraints"])
	}
	if constraints["max_length"] != float64(10) || constraints["max_syllables"] != float64(2) || constraints["prefix"] != "neo" || constraints["suffix"] != "ix" {
		t.Fatalf("constraints = %#v, want stable constraint shape", constraints)
	}
	avoidPrefixes := constraints["avoid_prefixes"].([]any)
	if len(avoidPrefixes) != 2 || avoidPrefixes[0] != "dev" || avoidPrefixes[1] != "neo" {
		t.Fatalf("avoid_prefixes = %#v, want [dev neo]", avoidPrefixes)
	}
	avoidSuffixes := constraints["avoid_suffixes"].([]any)
	if len(avoidSuffixes) != 2 || avoidSuffixes[0] != "ia" || avoidSuffixes[1] != "ora" {
		t.Fatalf("avoid_suffixes = %#v, want [ia ora]", avoidSuffixes)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty dry-run stderr", stderr.String())
	}
}

func TestRunGenerateDryRunJSONReflectsCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	configBody := "" +
		"openai:\n" +
		"  model: yaml-model\n" +
		"generate:\n" +
		"  count: 4\n" +
		"  batch_size: 2\n" +
		"  quality_profile: industrial\n" +
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n" +
		"  avoid_prefixes: dev,neo\n" +
		"  avoid_suffixes: ia,ora\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGetWorkingDir := getWorkingDir
	defer func() { getWorkingDir = originalGetWorkingDir }()
	getWorkingDir = func() (string, error) { return dir, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-generate", "short product stems",
		"-generate-dry-run",
		"-generate-dry-run-format", "json",
		"-generate-model", "cli-model",
		"-generate-count", "8",
		"-generate-batch-size", "4",
		"-generate-quality-profile", "off",
		"-generate-max-length", "12",
		"-generate-max-syllables", "3",
		"-generate-prefix", "dev",
		"-generate-suffix", "io",
		"-generate-style", "developer tool",
		"-generate-avoid-prefixes", "sys,neo",
		"-generate-avoid-suffixes", "io,iva",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, stdout.String())
	}
	if got["model"] != "cli-model" || got["generate_count"] != float64(8) || got["batch_size"] != float64(4) {
		t.Fatalf("top-level overrides = %#v, want CLI values", got)
	}
	if got["quality_profile"] != "off" {
		t.Fatalf("quality_profile = %#v, want off", got["quality_profile"])
	}
	constraints := got["constraints"].(map[string]any)
	if constraints["max_length"] != float64(12) || constraints["max_syllables"] != float64(3) || constraints["prefix"] != "dev" || constraints["suffix"] != "io" {
		t.Fatalf("constraints = %#v, want CLI override values", constraints)
	}
	avoidPrefixes := constraints["avoid_prefixes"].([]any)
	if len(avoidPrefixes) != 2 || avoidPrefixes[0] != "sys" || avoidPrefixes[1] != "neo" {
		t.Fatalf("avoid_prefixes = %#v, want [sys neo]", avoidPrefixes)
	}
	avoidSuffixes := constraints["avoid_suffixes"].([]any)
	if len(avoidSuffixes) != 2 || avoidSuffixes[0] != "io" || avoidSuffixes[1] != "iva" {
		t.Fatalf("avoid_suffixes = %#v, want [io iva]", avoidSuffixes)
	}
	if got["style"] != "developer tool" {
		t.Fatalf("style = %#v, want developer tool", got["style"])
	}
}

func TestRunGenerateDryRunRequiresPrompt(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-generate-dry-run",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "-generate-dry-run requires -generate") {
		t.Fatalf("Run() error = %v, want dry-run prompt validation", err)
	}
}

func TestRunTextWorkflowRejectsBannedGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  avoid_substrings: dev,cloud\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"devspark", "noviq"}}},
			{result: openai.BatchResult{Stems: []string{"cloudbase", "trynex"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if strings.Contains(stdout.String(), "devspark") || strings.Contains(stdout.String(), "cloudbase") {
		t.Fatalf("stdout = %q, want banned generated stems rejected before lookup", stdout.String())
	}
	if !strings.Contains(stdout.String(), "noviq\n") || !strings.Contains(stdout.String(), "trynex\n") {
		t.Fatalf("stdout = %q, want accepted generated stems only", stdout.String())
	}
	if !strings.Contains(stderr.String(), "banned 1") {
		t.Fatalf("stderr = %q, want lexical-ban aggregate feedback", stderr.String())
	}
	for _, fragment := range []string{
		"generation diagnostics",
		"banned_substring: 2",
	} {
		if !strings.Contains(stderr.String(), fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, stderr.String())
		}
	}
}

func TestRunTextWorkflowRejectsWeakGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  quality_profile: industrial\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"theravia", "noviq"}}},
			{result: openai.BatchResult{Stems: []string{"veloria", "traktor"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-generate", "industrial infrastructure names",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if strings.Contains(stdout.String(), "theravia") || strings.Contains(stdout.String(), "veloria") {
		t.Fatalf("stdout = %q, want weak generated stems rejected before lookup", stdout.String())
	}
	if !strings.Contains(stdout.String(), "noviq\n") || !strings.Contains(stdout.String(), "traktor\n") {
		t.Fatalf("stdout = %q, want stronger generated stems accepted", stdout.String())
	}
	if !strings.Contains(stderr.String(), "quality_rejected 1") {
		t.Fatalf("stderr = %q, want quality rejection aggregate feedback", stderr.String())
	}
	for _, fragment := range []string{
		"generation diagnostics",
		"quality.pharma_like_suffix: 2",
		"quality.soft_open_ending: 2",
		"quality.mushy_vowel_flow: 2",
		"quality.cv_alternation_mush: 1",
		"quality.weak_consonant_shape: 1",
	} {
		if !strings.Contains(stderr.String(), fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, stderr.String())
		}
	}
}

func TestRunGenerationRunSummaryIncludesDiagnostics(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "run-summary.json")
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("openai:\n  model: yaml-model\ngenerate:\n  count: 2\n  batch_size: 2\n  quality_profile: industrial\n  avoid_substrings: dev,cloud\n  avoid_prefixes: dev,neo\n  avoid_suffixes: ia,ora\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		model: "gpt-4o-mini",
		responses: []fakeStemResponse{
			{result: openai.BatchResult{
				Stems: []string{"devspark", "theravia"},
				Usage: &openai.Usage{InputTokens: 120, OutputTokens: 18, CachedInputTokens: 40},
			}},
			{result: openai.BatchResult{
				Stems: []string{"noviq", "traktor"},
				Usage: &openai.Usage{InputTokens: 80, OutputTokens: 12},
			}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-run-summary", summaryPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-generate", "industrial infrastructure names",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, string(data))
	}
	generation, ok := got["generation"].(map[string]any)
	if !ok {
		t.Fatalf("generation = %#v, want generation object", got["generation"])
	}
	if generation["model"] != "yaml-model" || generation["generate_count"] != float64(2) || generation["quality_profile"] != "industrial" {
		t.Fatalf("generation = %#v, want resolved generation settings", generation)
	}
	if got := generation["avoid_prefixes"].([]any); len(got) != 2 || got[0] != "dev" || got[1] != "neo" {
		t.Fatalf("avoid_prefixes = %#v, want [dev neo]", generation["avoid_prefixes"])
	}
	if got := generation["avoid_suffixes"].([]any); len(got) != 2 || got[0] != "ia" || got[1] != "ora" {
		t.Fatalf("avoid_suffixes = %#v, want [ia ora]", generation["avoid_suffixes"])
	}
	if generation["accepted_count"] != float64(2) {
		t.Fatalf("accepted_count = %#v, want 2", generation["accepted_count"])
	}
	if generation["input_tokens"] != float64(200) || generation["output_tokens"] != float64(30) || generation["cached_input_tokens"] != float64(40) {
		t.Fatalf("generation = %#v, want token totals", generation)
	}
	if generation["pricing_available"] != true {
		t.Fatalf("generation = %#v, want pricing_available true", generation)
	}
	cost, ok := generation["estimated_cost_usd"].(float64)
	if !ok || math.Abs(cost-4.5e-05) > 1e-12 {
		t.Fatalf("generation = %#v, want estimated_cost_usd ~= 0.000045", generation)
	}
	diagnostics, ok := got["diagnostics"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics = %#v, want diagnostics object", got["diagnostics"])
	}
	if diagnostics["banned"] != float64(2) || diagnostics["banned_substrings"] != float64(1) || diagnostics["banned_suffixes"] != float64(1) {
		t.Fatalf("diagnostics = %#v, want banned substring/suffix accounting", diagnostics)
	}
	if diagnostics["quality_rejected"] != float64(0) {
		t.Fatalf("diagnostics = %#v, want quality_rejected 0 after lexical filtering", diagnostics)
	}
	if stderr.Len() == 0 {
		t.Fatalf("stderr = %q, want normal generation status output", stderr.String())
	}
}

func TestRenderGenerationUsageLinesPricingUnavailable(t *testing.T) {
	lines := renderGenerationUsageLines("custom-model", openai.UsageTotals{
		Model:             "custom-model",
		Calls:             1,
		CallsWithUsage:    1,
		InputTokens:       100,
		OutputTokens:      20,
		CachedInputTokens: 10,
	})
	joined := strings.Join(lines, "\n")
	for _, fragment := range []string{
		"generation usage",
		"model: custom-model",
		"input_tokens: 100",
		"output_tokens: 20",
		"cached_input_tokens: 10",
		"estimated_cost_usd: pricing unavailable",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("renderGenerationUsageLines() missing %q:\n%s", fragment, joined)
		}
	}
}

func TestRunInteractiveGenerationDiagnosticsSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  quality_profile: industrial\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"theravia", "noviq"}}},
			{result: openai.BatchResult{Stems: []string{"veloria", "traktor"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-interactive",
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-generate", "industrial infrastructure names",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty interactive stdout", stdout.String())
	}

	progress := stderr.String()
	for _, fragment := range []string{
		"generation diagnostics",
		"quality.pharma_like_suffix: 2",
		"Done: checked 2 | emitted 2 | strong 2\n",
	} {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunJSONLWorkflowWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"example"}}},
			{result: openai.BatchResult{Stems: []string{"noviq"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-format", "jsonl",
		"-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("jsonl lines = %d, want 3: %q", len(lines), stdout.String())
	}
	var got []match.CandidateResult
	for _, line := range lines {
		var result match.CandidateResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		got = append(got, result)
	}
	if got[0].Candidate != "missing" || got[1].Candidate != "example" || got[2].Candidate != "noviq" {
		t.Fatalf("got candidates = %#v, want deterministic manual+generated order", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in jsonl mode", stderr.String())
	}
}

func TestRunJSONLWorkflowWithRunSummary(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "run-summary.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-format", "jsonl",
		"-run-summary", summaryPath,
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in jsonl mode", stderr.String())
	}
	var result match.CandidateResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v", err)
	}
	if result.Candidate != "example" {
		t.Fatalf("stdout result = %#v, want example", result)
	}

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(summary) error = %v\n%s", err, string(data))
	}
	if got["format"] != "jsonl" || got["total_checked_stems"] != float64(1) {
		t.Fatalf("summary = %#v, want jsonl summary fields", got)
	}
}

func TestRunTextWorkflowWithDegradedGeneratedBatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  max_attempts: 3\n  retry_count: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"missing", "bad.name"}}},
			{result: openai.BatchResult{Stems: []string{"noviq", "trynex"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantStdout := "" +
		"missing\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"noviq\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"trynex\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"summary\n" +
		"  total_candidates: 3\n" +
		"  emitted_results: 3\n" +
		"  present_in_any: 0\n" +
		"  absent_in_all: 3\n"
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}

	progress := stderr.String()
	wantProgress := []string{
		"generation: batch 1 attempt 1 requesting 2 stems",
		"generation: batch 1 attempt 1 accepted 0, invalid 1, banned 0, quality_rejected 0, duplicates 1, need 2 more",
		"generation: batch 1 attempt 2 requesting 2 stems",
		"generation: batch 1 attempt 2 accepted 2, invalid 0, banned 0, quality_rejected 0, duplicates 0, need 0 more",
		"generation: complete, accepted 2 stems",
		"generation diagnostics",
		"invalid: 1",
		"duplicates: 1",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunTextWorkflowGeneratedFailureReportsClearly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  max_attempts: 2\n  retry_count: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{result: openai.BatchResult{Stems: []string{"missing", "bad.name"}}},
			{result: openai.BatchResult{Stems: []string{"missing", "still bad"}}},
		},
	}

	originalGetWorkingDir := getWorkingDir
	originalNewStemGenerator := newStemGenerator
	defer func() {
		getWorkingDir = originalGetWorkingDir
		newStemGenerator = originalNewStemGenerator
	}()
	getWorkingDir = func() (string, error) { return dir, nil }
	newStemGenerator = func(config.Config) (openai.StemGenerator, error) { return generator, nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-no-interactive",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "missing",
		"-generate", "short invented brand stems",
	}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want generation failure")
	}
	if !strings.Contains(err.Error(), "generation produced 0 usable stems out of 2 requested") {
		t.Fatalf("error = %v, want bounded fulfillment failure", err)
	}
	if !strings.Contains(stderr.String(), "generation: failed after accepting 0 of 2 requested stems") {
		t.Fatalf("stderr = %q, want clear failure notice", stderr.String())
	}
}
