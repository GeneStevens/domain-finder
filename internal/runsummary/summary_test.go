package runsummary

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/genestevens/domain-finder/internal/candidates"
)

func TestNewDiagnosticsSortsReasons(t *testing.T) {
	got := NewDiagnostics(candidates.GenerationDiagnostics{
		Invalid:          1,
		Banned:           5,
		TooShort:         2,
		BannedSubstrings: 2,
		BannedPrefixes:   1,
		BannedSuffixes:   2,
		QualityRejected:  5,
		FamilyRejected:   2,
		Duplicates:       3,
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
	if got.TooShort != 2 {
		t.Fatalf("TooShort = %d, want 2", got.TooShort)
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
			Model:                   "gpt-4o-mini",
			Prompt:                  "industrial infrastructure names",
			GenerateCount:           4,
			BatchSize:               2,
			AdaptiveRefill:          true,
			MinBatchSize:            2,
			FinalEffectiveBatchSize: 1,
			MaxAttempts:             3,
			RetryCount:              2,
			QualityProfile:          "industrial",
			MinLength:               6,
			AvoidPrefixes:           []string{"dev", "neo"},
			AvoidSuffixes:           []string{"ia", "ora"},
			MaxCostUSD:              1.00,
			TargetAvailableHits:     6,
			TargetStrongHits:        3,
			MaxStallBatches:         4,
			AcceptedCount:           3,
			AvailableHits:           5,
			UnderfilledBatches:      2,
			UnderfilledStems:        5,
			StopReason:              "strong_hit_target_reached",
			InputTokens:             120,
			OutputTokens:            18,
			CachedInputTokens:       40,
			PricingAvailable:        true,
			EstimatedCostUSD:        0.0000258,
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
	generation, ok := got["generation"].(map[string]any)
	if !ok {
		t.Fatalf("generation = %#v, want generation object", got["generation"])
	}
	if generation["input_tokens"] != float64(120) || generation["output_tokens"] != float64(18) {
		t.Fatalf("generation = %#v, want token fields", generation)
	}
	if generation["pricing_available"] != true {
		t.Fatalf("generation = %#v, want pricing_available true", generation)
	}
	if _, ok := generation["avoid_prefixes"]; !ok {
		t.Fatalf("generation = %#v, want avoid_prefixes", generation)
	}
	if _, ok := generation["avoid_suffixes"]; !ok {
		t.Fatalf("generation = %#v, want avoid_suffixes", generation)
	}
	if generation["min_length"] != float64(6) {
		t.Fatalf("generation = %#v, want min_length", generation)
	}
	if generation["max_cost_usd"] != float64(1) || generation["target_available_hits"] != float64(6) || generation["target_strong_hits"] != float64(3) || generation["max_stall_batches"] != float64(4) {
		t.Fatalf("generation = %#v, want stop condition fields", generation)
	}
	if generation["available_hits"] != float64(5) {
		t.Fatalf("generation = %#v, want available_hits field", generation)
	}
	if generation["adaptive_refill"] != true || generation["min_batch_size"] != float64(2) || generation["final_effective_batch_size"] != float64(1) {
		t.Fatalf("generation = %#v, want adaptive refill fields", generation)
	}
	if generation["underfilled_batches"] != float64(2) || generation["underfilled_stems"] != float64(5) {
		t.Fatalf("generation = %#v, want underfill fields", generation)
	}
	if generation["stop_reason"] != "strong_hit_target_reached" {
		t.Fatalf("generation = %#v, want stop_reason field", generation)
	}
}
