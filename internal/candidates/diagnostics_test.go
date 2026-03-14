package candidates

import (
	"reflect"
	"testing"
)

func TestGenerationDiagnosticsMergeAndLines(t *testing.T) {
	var diagnostics GenerationDiagnostics
	diagnostics.MergeBatch(BatchReport{
		Invalid:         2,
		Duplicates:      3,
		LexicalRejected: 4,
		QualityRejected: 5,
		QualityReasons: map[string]int{
			"pharma_like_suffix": 3,
			"soft_open_ending":   2,
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
		"  banned_substring: 4",
		"  quality.soft_open_ending: 5",
		"  quality.pharma_like_suffix: 3",
		"  quality.vowel_heavy: 1",
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
