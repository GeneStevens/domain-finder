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

// Contract is the fully resolved generation contract that would be sent to
// OpenAI for one run.
type Contract struct {
	Model               string
	GenerateCount       int
	BatchSize           int
	MaxAttemptsPerBatch int
	RetryCount          int
	Theme               string
	Style               string
	MaxLength           int
	MaxSyllables        int
	Prefix              string
	Suffix              string
	SystemPrompt        string
	UserPrompt          string
}

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

// BuildContract resolves the full OpenAI generation contract for inspection or
// request construction.
func (b PromptBuilder) BuildContract(cfg config.Config, theme string) Contract {
	input := NewPromptInput(theme, cfg.Generate)
	return Contract{
		Model:               cfg.OpenAI.Model,
		GenerateCount:       cfg.Generate.Count,
		BatchSize:           cfg.Generate.BatchSize,
		MaxAttemptsPerBatch: cfg.Generate.MaxAttemptsPerBatch,
		RetryCount:          cfg.Generate.RetryCount,
		Theme:               input.Theme,
		Style:               input.Style,
		MaxLength:           input.MaxLength,
		MaxSyllables:        input.MaxSyllables,
		Prefix:              input.Prefix,
		Suffix:              input.Suffix,
		SystemPrompt:        systemPrompt,
		UserPrompt:          b.BuildUserPrompt(input, cfg.Generate.Count),
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

// RenderContract produces a human-readable prompt-inspection block.
func RenderContract(contract Contract) string {
	var out strings.Builder
	fmt.Fprintf(&out, "generation dry run\n")
	fmt.Fprintf(&out, "  model: %s\n", contract.Model)
	fmt.Fprintf(&out, "  generate_count: %d\n", contract.GenerateCount)
	fmt.Fprintf(&out, "  batch_size: %d\n", contract.BatchSize)
	fmt.Fprintf(&out, "  max_attempts: %d\n", contract.MaxAttemptsPerBatch)
	fmt.Fprintf(&out, "  retry_count: %d\n", contract.RetryCount)
	fmt.Fprintf(&out, "  theme: %s\n", renderOptional(contract.Theme))
	fmt.Fprintf(&out, "  style: %s\n", renderOptional(contract.Style))
	fmt.Fprintf(&out, "  max_length: %s\n", renderOptionalInt(contract.MaxLength))
	fmt.Fprintf(&out, "  max_syllables: %s\n", renderOptionalInt(contract.MaxSyllables))
	fmt.Fprintf(&out, "  prefix: %s\n", renderOptional(contract.Prefix))
	fmt.Fprintf(&out, "  suffix: %s\n", renderOptional(contract.Suffix))
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "system prompt\n")
	fmt.Fprintf(&out, "%s\n", contract.SystemPrompt)
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "user prompt\n")
	fmt.Fprintf(&out, "%s\n", contract.UserPrompt)
	return out.String()
}

func renderOptional(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func renderOptionalInt(value int) string {
	if value <= 0 {
		return "(none)"
	}
	return fmt.Sprintf("%d", value)
}
