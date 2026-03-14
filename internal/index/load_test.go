package index

import (
	"path/filepath"
	"testing"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func TestLoadExactPlain(t *testing.T) {
	idx, err := LoadExact("com", fixturePath("small", "com.zone"))
	if err != nil {
		t.Fatalf("LoadExact() error = %v", err)
	}

	if idx.ZoneName() != "com" {
		t.Fatalf("ZoneName() = %q, want %q", idx.ZoneName(), "com")
	}
	if idx.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", idx.Count())
	}
	if !idx.Contains("example.com") {
		t.Fatal("Contains(example.com) = false, want true")
	}
	if idx.Contains("missing.com") {
		t.Fatal("Contains(missing.com) = true, want false")
	}
}

func TestLoadExactGzipWithoutExtension(t *testing.T) {
	idx, err := LoadExact("net", fixturePath("small", "net.zone.slice"))
	if err != nil {
		t.Fatalf("LoadExact() error = %v", err)
	}

	if idx.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", idx.Count())
	}
	if !idx.Contains("example.net") {
		t.Fatal("Contains(example.net) = false, want true")
	}
	if !idx.Contains("MIXEDCASE.NET.") {
		t.Fatal("Contains(MIXEDCASE.NET.) = false, want true")
	}
}
