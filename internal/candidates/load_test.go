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
	got, err := Load(Sources{CLI: []string{"EXAMPLE.NET.", "missing.net"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example.net", "missing.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadFileOnly(t *testing.T) {
	got, err := Load(Sources{File: fixturePath("small", "candidates.txt")})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"missing.net", "example.net", "example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadStdinOnly(t *testing.T) {
	got, err := Load(Sources{
		UseStdin: true,
		Stdin:    strings.NewReader("# comment\n\nEXAMPLE.NET.\nmissing.net\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example.net", "missing.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadCLIMergedWithFileAndDeduped(t *testing.T) {
	got, err := Load(Sources{
		CLI:  []string{"EXAMPLE.NET.", "cli-only.com", "example.com"},
		File: fixturePath("small", "candidates.txt"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example.net", "cli-only.com", "example.com", "missing.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadFileAndStdinMerged(t *testing.T) {
	got, err := Load(Sources{
		File:     fixturePath("small", "candidates.txt"),
		UseStdin: true,
		Stdin:    strings.NewReader("stdin-only.net\nEXAMPLE.COM.\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"missing.net", "example.net", "example.com", "stdin-only.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadAllSourcesMergedAndDeduped(t *testing.T) {
	got, err := Load(Sources{
		CLI:      []string{"EXAMPLE.NET.", "cli-only.com"},
		File:     fixturePath("small", "candidates.txt"),
		UseStdin: true,
		Stdin:    strings.NewReader("missing.net\nstdin-only.net\ncli-only.com\n"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"example.net", "cli-only.com", "missing.net", "example.com", "stdin-only.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadRejectsInvalidCandidate(t *testing.T) {
	if _, err := Load(Sources{CLI: []string{"example"}}); err == nil {
		t.Fatal("Load() error = nil, want error for non-FQDN candidate")
	}
}

func TestLoadRejectsInvalidStdinCandidate(t *testing.T) {
	if _, err := Load(Sources{
		UseStdin: true,
		Stdin:    strings.NewReader("example\n"),
	}); err == nil {
		t.Fatal("Load() error = nil, want error for invalid stdin candidate")
	}
}
