package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gene/domain-finder/internal/match"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func TestRunTextWorkflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example.net",
		"-candidate", "missing.net",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "" +
		"summary\n" +
		"  total_candidates: 2\n" +
		"  emitted_results: 2\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 1\n" +
		"example.net\n" +
		"  summary: present in at least one loaded zone\n" +
		"  com: absent\n" +
		"  net: present\n" +
		"missing.net\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTextWorkflowWithCandidateFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-filter", "absent-in-all",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "" +
		"summary\n" +
		"  total_candidates: 3\n" +
		"  emitted_results: 1\n" +
		"  present_in_any: 2\n" +
		"  absent_in_all: 1\n" +
		"  filtered_out: 2\n" +
		"missing.net\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTextWorkflowWithCandidateStdin(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-filter", "absent-in-all",
		"-candidate-stdin",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
	}, strings.NewReader("# comment\n\nmissing.net\nEXAMPLE.NET.\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "" +
		"summary\n" +
		"  total_candidates: 2\n" +
		"  emitted_results: 1\n" +
		"  present_in_any: 1\n" +
		"  absent_in_all: 1\n" +
		"  filtered_out: 1\n" +
		"missing.net\n" +
		"  summary: absent in all loaded zones\n" +
		"  com: absent\n" +
		"  net: absent\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTextWorkflowWithMergedCandidateSources(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-filter", "all",
		"-candidate-stdin",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "EXAMPLE.NET.",
		"-candidate", "cli-only.com",
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader("missing.net\nstdin-only.net\ncli-only.com\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := stdout.String()
	wantFragments := []string{
		"  total_candidates: 5\n",
		"example.net\n",
		"cli-only.com\n",
		"missing.net\n",
		"example.com\n",
		"stdin-only.net\n",
	}
	for _, fragment := range wantFragments {
		if !bytes.Contains(stdout.Bytes(), []byte(fragment)) {
			t.Fatalf("stdout missing %q:\n%s", fragment, got)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSONLWorkflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{
		"-format", "jsonl",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate", "example.net",
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got match.CandidateResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Candidate != "example.net" || !got.PresentInAny || got.AbsentInAll {
		t.Fatalf("got = %#v, want present example.net result", got)
	}
	if len(got.Zones) != 2 || got.Zones[0].Zone != "com" || got.Zones[1].Zone != "net" {
		t.Fatalf("zones = %#v, want deterministic order", got.Zones)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTextWorkflowToFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "results.txt")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-out", outPath,
		"-filter", "absent-in-all",
		"-candidate-stdin",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader("missing.net\n"), &stdout, &stderr)
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
	if !bytes.Contains(data, []byte("missing.net")) || bytes.Contains(data, []byte("example.net\n  summary")) {
		t.Fatalf("file output = %q, want only filtered text results", string(data))
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSONLWorkflowToFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "results.jsonl")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"-out", outPath,
		"-format", "jsonl",
		"-filter", "absent-in-all",
		"-candidate-stdin",
		"-zone", "net=" + fixturePath("small", "net.zone.slice"),
		"-zone", "com=" + fixturePath("small", "com.zone"),
		"-candidate-file", fixturePath("small", "candidates.txt"),
	}, strings.NewReader("missing.net\n"), &stdout, &stderr)
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

	var got match.CandidateResult
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Candidate != "missing.net" || got.PresentInAny || !got.AbsentInAll {
		t.Fatalf("got = %#v, want only filtered missing.net result", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
