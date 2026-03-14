package candidates

import "testing"

func TestNormalizeCandidate(t *testing.T) {
	got, err := NormalizeCandidate(" Example-Name ")
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if got != "example-name" {
		t.Fatalf("NormalizeCandidate() = %q, want %q", got, "example-name")
	}
}

func TestNormalizeCandidateRejectsFQDN(t *testing.T) {
	if _, err := NormalizeCandidate("example.net"); err == nil {
		t.Fatal("NormalizeCandidate(example.net) error = nil, want error")
	}
}
