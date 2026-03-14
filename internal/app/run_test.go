package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
}

type fakeStemResponse struct {
	stems []string
	err   error
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

func (f *fakeStemGenerator) GenerateBatch(_ context.Context, _ string, count int) ([]string, error) {
	f.calls = append(f.calls, count)
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("unexpected GenerateBatch call")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.stems, response.err
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
		"checking: example... [1/2]",
		"checking: missing... [2/2]",
		"example",
		"NET",
		"missing",
		"COM NET",
		"✓",
		"Done: checked 2 | emitted 2 | strong 1\n",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
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
		"checking: missing... [1/3]",
		"checking: example... [2/3]",
		"checking: mixedcase... [3/3]",
		"missing",
		"COM NET",
		"✓",
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
	if !strings.Contains(stderr.String(), "missing") || !strings.Contains(stderr.String(), "\x1b[1;97;42m✓\x1b[0m") {
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
		responses: []fakeStemResponse{
			{stems: []string{"brandfoo", "noviq"}},
			{stems: []string{"example", "trynex"}},
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
		"generation: batch 1 attempt 1 accepted 2, invalid 0, duplicates 0, need 0 more",
		"generation: batch 2 attempt 1 requesting 2 stems",
		"generation: batch 2 attempt 1 accepted 2, invalid 0, duplicates 0, need 0 more",
		"generation: complete, accepted 4 stems",
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
			{stems: []string{"brandfoo", "example"}},
			{stems: []string{"noviq"}},
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
		"generation: batch 1 attempt 1 accepted 2, invalid 0, duplicates 0, need 0 more",
		"generation: batch 2 attempt 1 requesting 1 stems",
		"generation: batch 2 attempt 1 accepted 1, invalid 0, duplicates 0, need 0 more",
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
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n"
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{stems: []string{"shieldr", "trynex"}},
			{stems: []string{"noviq", "guardio"}},
			{stems: []string{"secbase"}},
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
		"  max_length: 10\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n"
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
		"theme: short product stems",
		"style: security product",
		"max_length: 10",
		"max_syllables: 2",
		"prefix: neo",
		"suffix: ix",
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
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n"
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
		"-generate-max-length", "12",
		"-generate-max-syllables", "3",
		"-generate-prefix", "dev",
		"-generate-suffix", "io",
		"-generate-style", "developer tool",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	wantFragments := []string{
		"model: cli-model",
		"generate_count: 8",
		"batch_size: 4",
		"max_length: 12",
		"max_syllables: 3",
		"prefix: dev",
		"suffix: io",
		"style: developer tool",
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
		"  max_length: 10\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n"
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
	constraints, ok := got["constraints"].(map[string]any)
	if !ok {
		t.Fatalf("constraints = %#v, want object", got["constraints"])
	}
	if constraints["max_length"] != float64(10) || constraints["max_syllables"] != float64(2) || constraints["prefix"] != "neo" || constraints["suffix"] != "ix" {
		t.Fatalf("constraints = %#v, want stable constraint shape", constraints)
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
		"  max_length: 9\n" +
		"  max_syllables: 2\n" +
		"  prefix: neo\n" +
		"  suffix: ix\n" +
		"  style: security product\n"
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
		"-generate-max-length", "12",
		"-generate-max-syllables", "3",
		"-generate-prefix", "dev",
		"-generate-suffix", "io",
		"-generate-style", "developer tool",
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
	constraints := got["constraints"].(map[string]any)
	if constraints["max_length"] != float64(12) || constraints["max_syllables"] != float64(3) || constraints["prefix"] != "dev" || constraints["suffix"] != "io" {
		t.Fatalf("constraints = %#v, want CLI override values", constraints)
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

func TestRunJSONLWorkflowWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{stems: []string{"example"}},
			{stems: []string{"noviq"}},
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

func TestRunTextWorkflowWithDegradedGeneratedBatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain-finder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 2\n  max_attempts: 3\n  retry_count: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		responses: []fakeStemResponse{
			{stems: []string{"missing", "bad.name"}},
			{stems: []string{"noviq", "trynex"}},
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
		"generation: batch 1 attempt 1 accepted 0, invalid 1, duplicates 1, need 2 more",
		"generation: batch 1 attempt 2 requesting 2 stems",
		"generation: batch 1 attempt 2 accepted 2, invalid 0, duplicates 0, need 0 more",
		"generation: complete, accepted 2 stems",
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
			{stems: []string{"missing", "bad.name"}},
			{stems: []string{"missing", "still bad"}},
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
