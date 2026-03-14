package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/genestevens/domain-finder/internal/config"
	"github.com/genestevens/domain-finder/internal/genquality"
)

const systemPrompt = "You generate candidate domain stems. Return JSON only. Produce concise single-label stems only, no TLDs, no dots, no spaces, no numbering, no explanations, lowercase preferred."

// PromptInput describes one generation request before it is turned into the
// final OpenAI instruction payload.
type PromptInput struct {
	QualityProfile   string
	Theme            string
	Style            string
	MaxLength        int
	MaxSyllables     int
	Prefix           string
	Suffix           string
	AvoidSubstrings  []string
	AvoidPrefixes    []string
	AvoidSuffixes    []string
	MaxCostUSD       float64
	TargetStrongHits int
	MaxStallBatches  int
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
	QualityProfile      string
	Theme               string
	Style               string
	MaxLength           int
	MaxSyllables        int
	Prefix              string
	Suffix              string
	AvoidSubstrings     []string
	AvoidPrefixes       []string
	AvoidSuffixes       []string
	MaxCostUSD          float64
	TargetStrongHits    int
	MaxStallBatches     int
	SystemPrompt        string
	UserPrompt          string
}

// NewPromptInput builds a prompt input from resolved config and the CLI theme.
func NewPromptInput(theme string, generate config.GenerateConfig) PromptInput {
	return PromptInput{
		QualityProfile:   strings.TrimSpace(generate.QualityProfile),
		Theme:            strings.TrimSpace(theme),
		Style:            strings.TrimSpace(generate.Style),
		MaxLength:        generate.MaxLength,
		MaxSyllables:     generate.MaxSyllables,
		Prefix:           strings.TrimSpace(generate.Prefix),
		Suffix:           strings.TrimSpace(generate.Suffix),
		AvoidSubstrings:  append([]string(nil), generate.AvoidSubstrings...),
		AvoidPrefixes:    append([]string(nil), generate.AvoidPrefixes...),
		AvoidSuffixes:    append([]string(nil), generate.AvoidSuffixes...),
		MaxCostUSD:       generate.MaxCostUSD,
		TargetStrongHits: generate.TargetStrongHits,
		MaxStallBatches:  generate.MaxStallBatches,
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
		QualityProfile:      input.QualityProfile,
		Theme:               input.Theme,
		Style:               input.Style,
		MaxLength:           input.MaxLength,
		MaxSyllables:        input.MaxSyllables,
		Prefix:              input.Prefix,
		Suffix:              input.Suffix,
		AvoidSubstrings:     append([]string(nil), input.AvoidSubstrings...),
		AvoidPrefixes:       append([]string(nil), input.AvoidPrefixes...),
		AvoidSuffixes:       append([]string(nil), input.AvoidSuffixes...),
		MaxCostUSD:          input.MaxCostUSD,
		TargetStrongHits:    input.TargetStrongHits,
		MaxStallBatches:     input.MaxStallBatches,
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
	if input.QualityProfile == genquality.ProfileIndustrial {
		lines = append(lines, "Quality profile: industrial. Prefer compact, harder-edged, infrastructure-like stems with stronger consonant anchors, denser consonant structure, and harder final consonants.")
		lines = append(lines, "Favor compact 5-7 letter company-name shapes and avoid soft startup-mush, pharma/biotech-like endings, weak generic enterprise-tech shapes, and repetitive same-family near variants.")
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
	if len(input.AvoidSubstrings) > 0 {
		lines = append(lines, fmt.Sprintf("Hard constraint: do not return stems containing any of these substrings: %s.", formatQuotedList(input.AvoidSubstrings)))
	}
	if len(input.AvoidPrefixes) > 0 {
		lines = append(lines, fmt.Sprintf("Hard constraint: do not return stems starting with any of these prefixes: %s.", formatQuotedList(input.AvoidPrefixes)))
	}
	if len(input.AvoidSuffixes) > 0 {
		lines = append(lines, fmt.Sprintf("Hard constraint: do not return stems ending with any of these suffixes: %s.", formatQuotedList(input.AvoidSuffixes)))
	}
	if input.MaxCostUSD > 0 {
		lines = append(lines, fmt.Sprintf("Run policy: generation may stop once estimated spend reaches $%.2f.", input.MaxCostUSD))
	}
	if input.TargetStrongHits > 0 {
		lines = append(lines, fmt.Sprintf("Run policy: generation may stop once %d strong all-zone hits are found.", input.TargetStrongHits))
	}
	if input.MaxStallBatches > 0 {
		lines = append(lines, fmt.Sprintf("Run policy: generation may stop after %d consecutive stall batches with no accepted stems and no strong-hit progress.", input.MaxStallBatches))
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
	fmt.Fprintf(&out, "  quality_profile: %s\n", renderOptional(contract.QualityProfile))
	fmt.Fprintf(&out, "  theme: %s\n", renderOptional(contract.Theme))
	fmt.Fprintf(&out, "  style: %s\n", renderOptional(contract.Style))
	fmt.Fprintf(&out, "  max_length: %s\n", renderOptionalInt(contract.MaxLength))
	fmt.Fprintf(&out, "  max_syllables: %s\n", renderOptionalInt(contract.MaxSyllables))
	fmt.Fprintf(&out, "  prefix: %s\n", renderOptional(contract.Prefix))
	fmt.Fprintf(&out, "  suffix: %s\n", renderOptional(contract.Suffix))
	fmt.Fprintf(&out, "  avoid_substrings: %s\n", renderOptionalList(contract.AvoidSubstrings))
	fmt.Fprintf(&out, "  avoid_prefixes: %s\n", renderOptionalList(contract.AvoidPrefixes))
	fmt.Fprintf(&out, "  avoid_suffixes: %s\n", renderOptionalList(contract.AvoidSuffixes))
	fmt.Fprintf(&out, "  max_cost_usd: %s\n", renderOptionalFloat(contract.MaxCostUSD))
	fmt.Fprintf(&out, "  target_strong_hits: %s\n", renderOptionalInt(contract.TargetStrongHits))
	fmt.Fprintf(&out, "  max_stall_batches: %s\n", renderOptionalInt(contract.MaxStallBatches))
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "system prompt\n")
	fmt.Fprintf(&out, "%s\n", contract.SystemPrompt)
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "user prompt\n")
	fmt.Fprintf(&out, "%s\n", contract.UserPrompt)
	return out.String()
}

// RenderContractJSON produces a stable machine-readable JSON representation of
// the fully resolved generation contract.
func RenderContractJSON(contract Contract) ([]byte, error) {
	type constraints struct {
		MaxLength        int      `json:"max_length,omitempty"`
		MaxSyllables     int      `json:"max_syllables,omitempty"`
		Prefix           string   `json:"prefix,omitempty"`
		Suffix           string   `json:"suffix,omitempty"`
		AvoidSubstrings  []string `json:"avoid_substrings,omitempty"`
		AvoidPrefixes    []string `json:"avoid_prefixes,omitempty"`
		AvoidSuffixes    []string `json:"avoid_suffixes,omitempty"`
		MaxCostUSD       float64  `json:"max_cost_usd,omitempty"`
		TargetStrongHits int      `json:"target_strong_hits,omitempty"`
		MaxStallBatches  int      `json:"max_stall_batches,omitempty"`
	}
	type view struct {
		Model          string      `json:"model"`
		GenerateCount  int         `json:"generate_count"`
		BatchSize      int         `json:"batch_size"`
		MaxAttempts    int         `json:"max_attempts"`
		RetryCount     int         `json:"retry_count"`
		QualityProfile string      `json:"quality_profile,omitempty"`
		Theme          string      `json:"theme"`
		Style          string      `json:"style,omitempty"`
		Constraints    constraints `json:"constraints"`
		SystemPrompt   string      `json:"system_prompt"`
		UserPrompt     string      `json:"user_prompt"`
	}

	payload := view{
		Model:          contract.Model,
		GenerateCount:  contract.GenerateCount,
		BatchSize:      contract.BatchSize,
		MaxAttempts:    contract.MaxAttemptsPerBatch,
		RetryCount:     contract.RetryCount,
		QualityProfile: contract.QualityProfile,
		Theme:          contract.Theme,
		Style:          contract.Style,
		Constraints: constraints{
			MaxLength:        contract.MaxLength,
			MaxSyllables:     contract.MaxSyllables,
			Prefix:           contract.Prefix,
			Suffix:           contract.Suffix,
			AvoidSubstrings:  append([]string(nil), contract.AvoidSubstrings...),
			AvoidPrefixes:    append([]string(nil), contract.AvoidPrefixes...),
			AvoidSuffixes:    append([]string(nil), contract.AvoidSuffixes...),
			MaxCostUSD:       contract.MaxCostUSD,
			TargetStrongHits: contract.TargetStrongHits,
			MaxStallBatches:  contract.MaxStallBatches,
		},
		SystemPrompt: contract.SystemPrompt,
		UserPrompt:   contract.UserPrompt,
	}

	return json.MarshalIndent(payload, "", "  ")
}

func renderOptionalList(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

func formatQuotedList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("`%s`", value))
	}
	return strings.Join(quoted, ", ")
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

func renderOptionalFloat(value float64) string {
	if value <= 0 {
		return "(none)"
	}
	return fmt.Sprintf("%.2f", value)
}
