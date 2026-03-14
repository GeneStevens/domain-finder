package backend

import (
	"context"
	"path/filepath"
	"testing"
)

func backendFixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func TestFileContainsStemAcrossZones(t *testing.T) {
	lookup, err := LoadFile(map[string]string{
		"com": backendFixturePath("small", "com.zone"),
		"net": backendFixturePath("small", "net.zone.slice"),
	})
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	present, err := lookup.Contains(context.Background(), "com", "example")
	if err != nil {
		t.Fatalf("Contains(com, example) error = %v", err)
	}
	if !present {
		t.Fatal("Contains(com, example) = false, want true")
	}

	absent, err := lookup.Contains(context.Background(), "net", "missing")
	if err != nil {
		t.Fatalf("Contains(net, missing) error = %v", err)
	}
	if absent {
		t.Fatal("Contains(net, missing) = true, want false")
	}
}
