package candidates

import "testing"

func TestNormalizeCandidate(t *testing.T) {
	got, err := NormalizeCandidate(" Example.NET. ")
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if got != "example.net" {
		t.Fatalf("NormalizeCandidate() = %q, want %q", got, "example.net")
	}
}

func TestNormalizeCandidateRejectsRelativeLabel(t *testing.T) {
	if _, err := NormalizeCandidate("example"); err == nil {
		t.Fatal("NormalizeCandidate(example) error = nil, want error")
	}
}
