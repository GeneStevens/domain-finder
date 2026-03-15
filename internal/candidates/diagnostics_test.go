package candidates

import (
	"reflect"
	"testing"
)

func TestGenerationDiagnosticsMergeAndLines(t *testing.T) {
	var diagnostics GenerationDiagnostics
	diagnostics.MergeBatch(BatchReport{
		Invalid:            2,
		Duplicates:         3,
		LexicalRejected:    7,
		TooShort:           2,
		ScoreRejected:      3,
		PhoneticRejected:   2,
		StructuralRejected: 1,
		BannedSubstrings:   4,
		BannedPrefixes:     2,
		BannedSuffixes:     1,
		QualityRejected:    5,
		FamilyRejected:     2,
		QualityReasons: map[string]int{
			"pharma_like_suffix": 3,
			"soft_open_ending":   2,
		},
		ScoreBuckets: map[string]int{
			"0-29":   2,
			"30-49":  1,
			"50-69":  2,
			"70-84":  1,
			"85-100": 1,
		},
	})
	diagnostics.MergeBatch(BatchReport{
		Invalid:    1,
		Duplicates: 2,
		QualityReasons: map[string]int{
			"soft_open_ending": 3,
			"vowel_heavy":      1,
		},
	})

	want := []string{
		"generation diagnostics",
		"  too_short: 2",
		"  score_rejected: 3",
		"  phonetic_rejected: 2",
		"  structural_rejected: 1",
		"  banned_substring: 4",
		"  banned_prefix: 2",
		"  banned_suffix: 1",
		"  quality.soft_open_ending: 5",
		"  quality.pharma_like_suffix: 3",
		"  quality.vowel_heavy: 1",
		"  score.0-29: 2",
		"  score.30-49: 1",
		"  score.50-69: 2",
		"  score.70-84: 1",
		"  score.85-100: 1",
		"  family_rejected: 2",
		"  invalid: 3",
		"  duplicates: 5",
	}
	if got := diagnostics.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Lines() = %#v, want %#v", got, want)
	}
}

func TestGenerationDiagnosticsLinesEmpty(t *testing.T) {
	var diagnostics GenerationDiagnostics
	if got := diagnostics.Lines(); got != nil {
		t.Fatalf("Lines() = %#v, want nil", got)
	}
}
