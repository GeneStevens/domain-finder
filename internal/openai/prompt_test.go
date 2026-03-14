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
		Theme:           "invented product names",
		Style:           "developer tool",
		MaxLength:       12,
		MaxSyllables:    3,
		Prefix:          "dev",
		Suffix:          "io",
		AvoidSubstrings: []string{"stack", "cloud"},
	}, 25)

	wantFragments := []string{
		"Generate 25 candidate stems.",
		"Theme: invented product names",
		"Style: developer tool",
		"no more than 12 letters",
		"no more than 3 syllables",
		"start with `dev`",
		"end with `io`",
		"do not return stems containing any of these substrings",
		"`stack`",
		"`cloud`",
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
		Style:           "security product",
		MaxLength:       10,
		MaxSyllables:    2,
		Prefix:          "sec",
		Suffix:          "ix",
		AvoidSubstrings: []string{"dev", "cloud"},
	})

	if got.Theme != "security names" || got.Style != "security product" || got.MaxLength != 10 || got.MaxSyllables != 2 || got.Prefix != "sec" || got.Suffix != "ix" || len(got.AvoidSubstrings) != 2 {
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
			MaxLength:           12,
			MaxSyllables:        3,
			Prefix:              "dev",
			Suffix:              "io",
			Style:               "developer tool",
			AvoidSubstrings:     []string{"stack", "cloud"},
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
		"theme: short product name stems",
		"style: developer tool",
		"max_length: 12",
		"max_syllables: 3",
		"prefix: dev",
		"suffix: io",
		"avoid_substrings: stack, cloud",
		"system prompt",
		"user prompt",
		"start with `dev`",
		"end with `io`",
		"`stack`",
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
			MaxLength:           12,
			MaxSyllables:        3,
			Prefix:              "dev",
			Suffix:              "io",
			Style:               "developer tool",
			AvoidSubstrings:     []string{"stack", "cloud"},
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
	if got["system_prompt"] == "" || got["user_prompt"] == "" {
		t.Fatalf("prompts missing in %#v", got)
	}
}
