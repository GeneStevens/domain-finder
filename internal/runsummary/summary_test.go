package runsummary

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/genestevens/domain-finder/internal/candidates"
)

func TestNewDiagnosticsSortsReasons(t *testing.T) {
	got := NewDiagnostics(candidates.GenerationDiagnostics{
		Invalid:         1,
		Banned:          2,
		QualityRejected: 5,
		Duplicates:      3,
		QualityReasons: map[string]int{
			"soft_open_ending":   2,
			"pharma_like_suffix": 4,
			"mushy_vowel_flow":   4,
		},
	})
	if got == nil {
		t.Fatal("NewDiagnostics() = nil, want diagnostics")
	}
	if len(got.QualityReasons) != 3 {
		t.Fatalf("QualityReasons len = %d, want 3", len(got.QualityReasons))
	}
	if got.QualityReasons[0].Reason != "mushy_vowel_flow" || got.QualityReasons[0].Count != 4 {
		t.Fatalf("QualityReasons[0] = %#v, want mushy_vowel_flow/4", got.QualityReasons[0])
	}
	if got.QualityReasons[1].Reason != "pharma_like_suffix" || got.QualityReasons[1].Count != 4 {
		t.Fatalf("QualityReasons[1] = %#v, want pharma_like_suffix/4", got.QualityReasons[1])
	}
}

func TestWriteArtifactJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, Artifact{
		Backend:           "file",
		RequestedZones:    []string{"com", "net"},
		FilterMode:        "all",
		Interactive:       true,
		Format:            "text",
		TotalCheckedStems: 4,
		EmittedResults:    3,
		StrongHits:        2,
		PresentInAny:      1,
		Generation: &Generation{
			Model:          "gpt-4o-mini",
			Prompt:         "industrial infrastructure names",
			GenerateCount:  4,
			BatchSize:      2,
			MaxAttempts:    3,
			RetryCount:     2,
			QualityProfile: "industrial",
			AcceptedCount:  3,
		},
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got["backend"] != "file" || got["filter_mode"] != "all" {
		t.Fatalf("got = %#v, want backend/filter fields", got)
	}
	if got["interactive"] != true || got["format"] != "text" {
		t.Fatalf("got = %#v, want interactive format fields", got)
	}
}
