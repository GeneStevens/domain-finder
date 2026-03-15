package namescore

import "testing"

func TestEvaluateGoodExamplesPassNormalThreshold(t *testing.T) {
	tests := []string{"crux", "fortis", "vectra", "vertex"}
	for _, stem := range tests {
		score := Evaluate(stem)
		if score.Total < 50 {
			t.Fatalf("Evaluate(%q) total = %d, want >= 50", stem, score.Total)
		}
	}
}

func TestEvaluateBadExamplesFailNormalThreshold(t *testing.T) {
	tests := []string{"grithlex", "vargphlix", "mmorath"}
	for _, stem := range tests {
		score := Evaluate(stem)
		if score.Total >= 50 {
			t.Fatalf("Evaluate(%q) total = %d, want < 50", stem, score.Total)
		}
	}
}

func TestEvaluateHardRejectsPhoneticFailures(t *testing.T) {
	if score := Evaluate("vargphlix"); score.Phonetic != 0 {
		t.Fatalf("Evaluate(vargphlix).Phonetic = %d, want 0", score.Phonetic)
	}
	if score := Evaluate("aeiouiaeio"); score.Phonetic != 0 {
		t.Fatalf("Evaluate(aeiouiaeio).Phonetic = %d, want 0", score.Phonetic)
	}
}

func TestEffectiveMinScoreStrictRaisesFloor(t *testing.T) {
	if got := EffectiveMinScore(50, QualityStrict); got != 70 {
		t.Fatalf("EffectiveMinScore(50, strict) = %d, want 70", got)
	}
	if got := EffectiveMinScore(82, QualityStrict); got != 82 {
		t.Fatalf("EffectiveMinScore(82, strict) = %d, want 82", got)
	}
}
