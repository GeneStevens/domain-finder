package index

import "testing"

func TestExactStoresNormalizedDomains(t *testing.T) {
	idx := NewExact("net")
	idx.Add("EXAMPLE.NET.")
	idx.Add(" example.net ")
	idx.Add("")

	if idx.ZoneName() != "net" {
		t.Fatalf("ZoneName() = %q, want %q", idx.ZoneName(), "net")
	}
	if !idx.Contains("example.net") {
		t.Fatal("Contains(example.net) = false, want true")
	}
	if !idx.Contains("EXAMPLE.NET.") {
		t.Fatal("Contains(EXAMPLE.NET.) = false, want true")
	}
	if idx.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", idx.Count())
	}
}
