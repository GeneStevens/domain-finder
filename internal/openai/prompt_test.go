package openai

import (
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/config"
)

func TestPromptBuilderWithMultipleConstraints(t *testing.T) {
	builder := PromptBuilder{}
	got := builder.BuildUserPrompt(PromptInput{
		Theme:        "invented product names",
		Style:        "developer tool",
		MaxLength:    12,
		MaxSyllables: 3,
		Prefix:       "dev",
		Suffix:       "io",
	}, 25)

	wantFragments := []string{
		"Generate 25 candidate stems.",
		"Theme: invented product names",
		"Style: developer tool",
		"no more than 12 letters",
		"no more than 3 syllables",
		"start with `dev`",
		"end with `io`",
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
		Style:        "security product",
		MaxLength:    10,
		MaxSyllables: 2,
		Prefix:       "sec",
		Suffix:       "ix",
	})

	if got.Theme != "security names" || got.Style != "security product" || got.MaxLength != 10 || got.MaxSyllables != 2 || got.Prefix != "sec" || got.Suffix != "ix" {
		t.Fatalf("NewPromptInput() = %#v, want populated constraint input", got)
	}
}
