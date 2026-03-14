package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gene/domain-finder/internal/config"
	"github.com/gene/domain-finder/internal/match"
	"github.com/gene/domain-finder/internal/openai"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

type fakeStemGenerator struct {
	batches [][]string
	calls   []int
}

func (f *fakeStemGenerator) GenerateBatch(_ context.Context, _ string, count int) ([]string, error) {
	f.calls = append(f.calls, count)
	if len(f.batches) == 0 {
		return nil, fmt.Errorf("unexpected GenerateBatch call")
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	return batch, nil
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

	wantStdout := "" +
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
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}

	progress := stderr.String()
	wantProgress := []string{
		"Zone files loaded: COM, NET\n",
		"Searching 2 domains | filter: all\n",
		"candidate",
		"> [1/2] example",
		"  example",
		"  missing",
		"Done: checked 2, emitted 2\n",
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
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}

	progress := stderr.String()
	wantProgress := []string{
		"Searching 3 domains | filter: absent-in-all\n",
		"> [1/3] missing",
		"> [2/3] example",
		"> [3/3] mixedcase",
		"  missing",
		"Done: checked 3, emitted 1\n",
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
	if !strings.Contains(stderr.String(), "Done: checked 3, emitted 1\n") {
		t.Fatalf("stderr = %q, want interactive completion", stderr.String())
	}
}

func TestRunTextWorkflowWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domainfinder.yaml"), []byte("generate:\n  count: 4\n  batch_size: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		batches: [][]string{
			{"brandfoo", "missing"},
			{"example", "noviq"},
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
		"example\n" +
		"  summary: present in at least one loaded zone\n" +
		"  com: present\n" +
		"  net: present\n" +
		"noviq\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n" +
		"summary\n" +
		"  total_candidates: 4\n" +
		"  emitted_results: 4\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 3\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in fallback text mode", stderr.String())
	}
	if len(generator.calls) != 2 || generator.calls[0] != 2 || generator.calls[1] != 2 {
		t.Fatalf("generator calls = %#v, want [2 2]", generator.calls)
	}
}

func TestRunTextWorkflowInteractiveWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domainfinder.yaml"), []byte("generate:\n  count: 3\n  batch_size: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		batches: [][]string{
			{"brandfoo", "example"},
			{"noviq"},
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

	wantStdout := "" +
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
		"summary\n" +
		"  total_candidates: 4\n" +
		"  emitted_results: 3\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 3\n" +
		"  filtered_out: 1\n"
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}

	progress := stderr.String()
	wantProgress := []string{
		"Searching 4 domains | filter: absent-in-all\n",
		"> [1/4] missing",
		"> [2/4] brandfoo",
		"> [3/4] example",
		"> [4/4] noviq",
		"  missing",
		"  brandfoo",
		"  noviq",
		"Done: checked 4, emitted 3\n",
	}
	for _, fragment := range wantProgress {
		if !strings.Contains(progress, fragment) {
			t.Fatalf("stderr missing %q:\n%s", fragment, progress)
		}
	}
}

func TestRunJSONLWorkflowWithGeneratedStems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domainfinder.yaml"), []byte("generate:\n  count: 2\n  batch_size: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := &fakeStemGenerator{
		batches: [][]string{
			{"example"},
			{"noviq"},
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
