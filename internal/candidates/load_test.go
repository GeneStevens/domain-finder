package candidates

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func TestLoadCLIOnly(t *testing.T) {
	got, err := Load(Sources{CLI: []string{"EXAMPLE", "missing"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example", "missing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadFileOnly(t *testing.T) {
	got, err := Load(Sources{File: fixturePath("small", "candidates.txt")})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"missing", "example", "mixedcase"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadStdinOnly(t *testing.T) {
	got, err := Load(Sources{
		UseStdin: true,
		Stdin:    strings.NewReader("# comment\n\nEXAMPLE\nmissing\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example", "missing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadCLIMergedWithFileAndDeduped(t *testing.T) {
	got, err := Load(Sources{
		CLI:  []string{"EXAMPLE", "cli-only", "mixedcase"},
		File: fixturePath("small", "candidates.txt"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example", "cli-only", "mixedcase", "missing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadFileAndStdinMerged(t *testing.T) {
	got, err := Load(Sources{
		File:     fixturePath("small", "candidates.txt"),
		UseStdin: true,
		Stdin:    strings.NewReader("stdin-only\nMIXEDCASE\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"missing", "example", "mixedcase", "stdin-only"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadAllSourcesMergedAndDeduped(t *testing.T) {
	got, err := Load(Sources{
		CLI:      []string{"EXAMPLE", "cli-only"},
		File:     fixturePath("small", "candidates.txt"),
		UseStdin: true,
		Stdin:    strings.NewReader("missing\nstdin-only\ncli-only\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example", "cli-only", "missing", "mixedcase", "stdin-only"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadRejectsInvalidCandidate(t *testing.T) {
	if _, err := Load(Sources{CLI: []string{"example.net"}}); err == nil {
		t.Fatal("Load() error = nil, want error for FQDN candidate")
	}
}

func TestLoadRejectsInvalidStdinCandidate(t *testing.T) {
	if _, err := Load(Sources{
		UseStdin: true,
		Stdin:    strings.NewReader("example.net\n"),
	}); err == nil {
		t.Fatal("Load() error = nil, want error for invalid stdin candidate")
	}
}
