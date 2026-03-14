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
	found := false
	for _, reason := range got.Reasons {
		if reason == ReasonPharmaLikeSuffix {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("reasons = %#v, want pharma_like_suffix", got.Reasons)
	}
}

func TestEvaluateIndustrialAcceptsStrongerStem(t *testing.T) {
	got := Evaluate("kinrox", ProfileIndustrial)
	if !got.Accepted {
		t.Fatalf("Evaluate() = %#v, want accepted", got)
	}
	if got.Score < 1 {
		t.Fatalf("score = %d, want positive score", got.Score)
	}
}

func TestEvaluateIndustrialRejectsCVMush(t *testing.T) {
	got := Evaluate("aevoria", ProfileIndustrial)
	if got.Accepted {
		t.Fatalf("Evaluate() = %#v, want rejected", got)
	}
	if len(got.Reasons) == 0 {
		t.Fatalf("reasons = %#v, want rejection reasons", got.Reasons)
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
