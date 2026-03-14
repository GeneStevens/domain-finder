package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/config"
)

func TestPromptBuilderWithMultipleConstraints(t *testing.T) {
	builder := PromptBuilder{}
	got := builder.BuildUserPrompt(PromptInput{
		QualityProfile:      "industrial",
		Theme:               "invented product names",
		Style:               "developer tool",
		MaxLength:           12,
		MaxSyllables:        3,
		Prefix:              "dev",
		Suffix:              "io",
		AvoidSubstrings:     []string{"stack", "cloud"},
		AvoidPrefixes:       []string{"dev", "neo"},
		AvoidSuffixes:       []string{"ia", "ora"},
		MaxCostUSD:          1.25,
		TargetAvailableHits: 11,
		TargetStrongHits:    7,
		MaxStallBatches:     4,
		AdaptiveRefill:      true,
		MinBatchSize:        2,
	}, 25)

	wantFragments := []string{
		"Generate 25 candidate stems.",
		"Theme: invented product names",
		"Style: developer tool",
		"Quality profile: industrial",
		"infrastructure-like stems",
		"no more than 12 letters",
		"no more than 3 syllables",
		"start with `dev`",
		"end with `io`",
		"do not return stems containing any of these substrings",
		"do not return stems starting with any of these prefixes",
		"do not return stems ending with any of these suffixes",
		"`stack`",
		"`cloud`",
		"`neo`",
		"`ora`",
		"estimated spend reaches $1.25",
		"11 available hits are found",
		"7 strong all-zone hits",
		"4 consecutive stall batches",
		"adaptive refill may shrink the effective batch size",
		"never below 2",
		"Do not include bullets, numbering, commentary, or duplicate stems.",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("prompt missing %q:\n%s", fragment, got)
		}
	}
}

func TestNewPromptInputFromConfig(t *testing.T) {
	got := NewPromptInput("security names", config.GenerateConfig{
		QualityProfile:      "industrial",
		Style:               "security product",
		MaxLength:           10,
		MaxSyllables:        2,
		Prefix:              "sec",
		Suffix:              "ix",
		AvoidSubstrings:     []string{"dev", "cloud"},
		AvoidPrefixes:       []string{"dev"},
		AvoidSuffixes:       []string{"ia", "ora"},
		MaxCostUSD:          0.75,
		TargetAvailableHits: 4,
		TargetStrongHits:    5,
		MaxStallBatches:     3,
		AdaptiveRefill:      true,
		MinBatchSize:        2,
	})

	if got.QualityProfile != "industrial" || got.Theme != "security names" || got.Style != "security product" || got.MaxLength != 10 || got.MaxSyllables != 2 || got.Prefix != "sec" || got.Suffix != "ix" || len(got.AvoidSubstrings) != 2 || len(got.AvoidPrefixes) != 1 || len(got.AvoidSuffixes) != 2 || got.MaxCostUSD != 0.75 || got.TargetAvailableHits != 4 || got.TargetStrongHits != 5 || got.MaxStallBatches != 3 || !got.AdaptiveRefill || got.MinBatchSize != 2 {
		t.Fatalf("NewPromptInput() = %#v, want populated constraint input", got)
	}
}

func TestBuildContractAndRender(t *testing.T) {
	builder := PromptBuilder{}
	contract := builder.BuildContract(config.Config{
		OpenAI: config.OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		Generate: config.GenerateConfig{
			Count:               8,
			BatchSize:           4,
			MaxAttemptsPerBatch: 3,
			RetryCount:          2,
			QualityProfile:      "industrial",
			MaxLength:           12,
			MaxSyllables:        3,
			Prefix:              "dev",
			Suffix:              "io",
			Style:               "developer tool",
			AvoidSubstrings:     []string{"stack", "cloud"},
			AvoidPrefixes:       []string{"dev", "neo"},
			AvoidSuffixes:       []string{"ia", "ora"},
			MaxCostUSD:          1.25,
			TargetAvailableHits: 11,
			TargetStrongHits:    7,
			MaxStallBatches:     4,
			AdaptiveRefill:      true,
			MinBatchSize:        2,
		},
	}, "short product name stems")

	if contract.UserPrompt == "" || contract.SystemPrompt == "" {
		t.Fatalf("BuildContract() = %#v, want populated prompts", contract)
	}

	rendered := RenderContract(contract)
	wantFragments := []string{
		"generation dry run",
		"model: gpt-4o-mini",
		"generate_count: 8",
		"batch_size: 4",
		"max_attempts: 3",
		"retry_count: 2",
		"quality_profile: industrial",
		"theme: short product name stems",
		"style: developer tool",
		"max_length: 12",
		"max_syllables: 3",
		"prefix: dev",
		"suffix: io",
		"avoid_substrings: stack, cloud",
		"avoid_prefixes: dev, neo",
		"avoid_suffixes: ia, ora",
		"max_cost_usd: 1.25",
		"target_available_hits: 11",
		"target_strong_hits: 7",
		"max_stall_batches: 4",
		"adaptive_refill: true",
		"min_batch_size: 2",
		"system prompt",
		"user prompt",
		"start with `dev`",
		"end with `io`",
		"`stack`",
		"`neo`",
		"`ora`",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("RenderContract() missing %q:\n%s", fragment, rendered)
		}
	}
}

func TestRenderContractJSON(t *testing.T) {
	builder := PromptBuilder{}
	contract := builder.BuildContract(config.Config{
		OpenAI: config.OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		Generate: config.GenerateConfig{
			Count:               8,
			BatchSize:           4,
			MaxAttemptsPerBatch: 3,
			RetryCount:          2,
			QualityProfile:      "industrial",
			MaxLength:           12,
			MaxSyllables:        3,
			Prefix:              "dev",
			Suffix:              "io",
			Style:               "developer tool",
			AvoidSubstrings:     []string{"stack", "cloud"},
			AvoidPrefixes:       []string{"dev", "neo"},
			AvoidSuffixes:       []string{"ia", "ora"},
			MaxCostUSD:          1.25,
			TargetAvailableHits: 11,
			TargetStrongHits:    7,
			MaxStallBatches:     4,
			AdaptiveRefill:      true,
			MinBatchSize:        2,
		},
	}, "short product name stems")

	raw, err := RenderContractJSON(contract)
	if err != nil {
		t.Fatalf("RenderContractJSON() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got["model"] != "gpt-4o-mini" {
		t.Fatalf("model = %#v, want gpt-4o-mini", got["model"])
	}
	if got["generate_count"] != float64(8) {
		t.Fatalf("generate_count = %#v, want 8", got["generate_count"])
	}
	if got["batch_size"] != float64(4) {
		t.Fatalf("batch_size = %#v, want 4", got["batch_size"])
	}
	if got["theme"] != "short product name stems" {
		t.Fatalf("theme = %#v, want short product name stems", got["theme"])
	}
	if got["quality_profile"] != "industrial" {
		t.Fatalf("quality_profile = %#v, want industrial", got["quality_profile"])
	}
	constraints, ok := got["constraints"].(map[string]any)
	if !ok {
		t.Fatalf("constraints = %#v, want object", got["constraints"])
	}
	if constraints["max_length"] != float64(12) || constraints["prefix"] != "dev" || constraints["suffix"] != "io" {
		t.Fatalf("constraints = %#v, want populated stable constraint object", constraints)
	}
	avoid := constraints["avoid_substrings"].([]any)
	if len(avoid) != 2 || avoid[0] != "stack" || avoid[1] != "cloud" {
		t.Fatalf("avoid_substrings = %#v, want [stack cloud]", avoid)
	}
	avoidPrefixes := constraints["avoid_prefixes"].([]any)
	if len(avoidPrefixes) != 2 || avoidPrefixes[0] != "dev" || avoidPrefixes[1] != "neo" {
		t.Fatalf("avoid_prefixes = %#v, want [dev neo]", avoidPrefixes)
	}
	avoidSuffixes := constraints["avoid_suffixes"].([]any)
	if len(avoidSuffixes) != 2 || avoidSuffixes[0] != "ia" || avoidSuffixes[1] != "ora" {
		t.Fatalf("avoid_suffixes = %#v, want [ia ora]", avoidSuffixes)
	}
	if constraints["max_cost_usd"] != 1.25 || constraints["target_available_hits"] != float64(11) || constraints["target_strong_hits"] != float64(7) || constraints["max_stall_batches"] != float64(4) {
		t.Fatalf("constraints = %#v, want stop-condition fields", constraints)
	}
	if constraints["adaptive_refill"] != true || constraints["min_batch_size"] != float64(2) {
		t.Fatalf("constraints = %#v, want adaptive refill fields", constraints)
	}
	if got["system_prompt"] == "" || got["user_prompt"] == "" {
		t.Fatalf("prompts missing in %#v", got)
	}
}
