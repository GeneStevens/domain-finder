package genquality

import "testing"

func TestEvaluateIndustrialRejectsWeakPharmaLikeStem(t *testing.T) {
	got := Evaluate("theravia", ProfileIndustrial)
	if got.Accepted {
		t.Fatalf("Evaluate() = %#v, want rejected", got)
	}
	if got.Score >= 1 {
		t.Fatalf("score = %d, want rejection score", got.Score)
	}
	if len(got.Reasons) == 0 {
		t.Fatalf("reasons = %#v, want rejection reasons", got.Reasons)
	}
}

func TestEvaluateIndustrialAcceptsStrongerStem(t *testing.T) {
	got := Evaluate("traktor", ProfileIndustrial)
	if !got.Accepted {
		t.Fatalf("Evaluate() = %#v, want accepted", got)
	}
	if got.Score < 1 {
		t.Fatalf("score = %d, want positive score", got.Score)
	}
}

func TestNormalizeProfile(t *testing.T) {
	got, err := NormalizeProfile("disabled")
	if err != nil {
		t.Fatalf("NormalizeProfile() error = %v", err)
	}
	if got != ProfileOff {
		t.Fatalf("NormalizeProfile() = %q, want off", got)
	}

	if _, err := NormalizeProfile("mystery"); err == nil {
		t.Fatal("NormalizeProfile() error = nil, want unsupported profile error")
	}
}
