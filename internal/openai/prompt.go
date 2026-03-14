package openai

import (
	"fmt"
	"strings"

	"github.com/genestevens/domain-finder/internal/config"
)

const systemPrompt = "You generate candidate domain stems. Return JSON only. Produce concise single-label stems only, no TLDs, no dots, no spaces, no numbering, no explanations, lowercase preferred."

// PromptInput describes one generation request before it is turned into the
// final OpenAI instruction payload.
type PromptInput struct {
	Theme        string
	Style        string
	MaxLength    int
	MaxSyllables int
	Prefix       string
	Suffix       string
}

// PromptBuilder constructs disciplined generation instructions.
type PromptBuilder struct{}

// NewPromptInput builds a prompt input from resolved config and the CLI theme.
func NewPromptInput(theme string, generate config.GenerateConfig) PromptInput {
	return PromptInput{
		Theme:        strings.TrimSpace(theme),
		Style:        strings.TrimSpace(generate.Style),
		MaxLength:    generate.MaxLength,
		MaxSyllables: generate.MaxSyllables,
		Prefix:       strings.TrimSpace(generate.Prefix),
		Suffix:       strings.TrimSpace(generate.Suffix),
	}
}

// BuildUserPrompt creates the final user prompt text sent to OpenAI.
func (PromptBuilder) BuildUserPrompt(input PromptInput, count int) string {
	lines := []string{
		fmt.Sprintf("Generate %d candidate stems.", count),
		"Return a JSON object with a single `stems` array of strings.",
		"Each stem must be a single label only: no TLDs, no dots, no spaces, and no explanations.",
		"Prefer lowercase alphabetic stems; only use hyphens when they materially improve readability.",
	}
	if input.Theme != "" {
		lines = append(lines, "Theme: "+input.Theme)
	}
	if input.Style != "" {
		lines = append(lines, "Style: "+input.Style)
	}
	if input.MaxLength > 0 {
		lines = append(lines, fmt.Sprintf("Constraint: each stem must be no more than %d letters.", input.MaxLength))
	}
	if input.MaxSyllables > 0 {
		lines = append(lines, fmt.Sprintf("Constraint: each stem should be no more than %d syllables.", input.MaxSyllables))
	}
	if input.Prefix != "" {
		lines = append(lines, fmt.Sprintf("Constraint: prefer stems that start with `%s`.", input.Prefix))
	}
	if input.Suffix != "" {
		lines = append(lines, fmt.Sprintf("Constraint: prefer stems that end with `%s`.", input.Suffix))
	}
	lines = append(lines, "Do not include bullets, numbering, commentary, or duplicate stems.")
	return strings.Join(lines, "\n")
}
