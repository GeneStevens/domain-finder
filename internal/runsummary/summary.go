package runsummary

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/genestevens/domain-finder/internal/candidates"
)

type Artifact struct {
	Backend           string       `json:"backend"`
	RequestedZones    []string     `json:"requested_zones"`
	FilterMode        string       `json:"filter_mode"`
	Interactive       bool         `json:"interactive"`
	Format            string       `json:"format"`
	TotalCheckedStems int          `json:"total_checked_stems"`
	EmittedResults    int          `json:"emitted_results"`
	StrongHits        int          `json:"strong_hits"`
	PresentInAny      int          `json:"present_in_any"`
	Generation        *Generation  `json:"generation,omitempty"`
	Diagnostics       *Diagnostics `json:"diagnostics,omitempty"`
}

type Generation struct {
	Model             string   `json:"model"`
	Prompt            string   `json:"prompt"`
	Style             string   `json:"style,omitempty"`
	GenerateCount     int      `json:"generate_count"`
	BatchSize         int      `json:"batch_size"`
	MaxAttempts       int      `json:"max_attempts"`
	RetryCount        int      `json:"retry_count"`
	QualityProfile    string   `json:"quality_profile,omitempty"`
	AvoidSubstrings   []string `json:"avoid_substrings,omitempty"`
	AcceptedCount     int      `json:"accepted_count"`
	InputTokens       int      `json:"input_tokens,omitempty"`
	OutputTokens      int      `json:"output_tokens,omitempty"`
	CachedInputTokens int      `json:"cached_input_tokens,omitempty"`
	PricingAvailable  bool     `json:"pricing_available"`
	EstimatedCostUSD  float64  `json:"estimated_cost_usd,omitempty"`
}

type Diagnostics struct {
	Invalid         int           `json:"invalid"`
	Banned          int           `json:"banned"`
	QualityRejected int           `json:"quality_rejected"`
	Duplicates      int           `json:"duplicates"`
	QualityReasons  []ReasonCount `json:"quality_reasons,omitempty"`
}

type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

func NewDiagnostics(source candidates.GenerationDiagnostics) *Diagnostics {
	if !source.HasData() {
		return nil
	}
	reasons := make([]ReasonCount, 0, len(source.QualityReasons))
	for reason, count := range source.QualityReasons {
		reasons = append(reasons, ReasonCount{Reason: reason, Count: count})
	}
	sort.Slice(reasons, func(i, j int) bool {
		if reasons[i].Count != reasons[j].Count {
			return reasons[i].Count > reasons[j].Count
		}
		return reasons[i].Reason < reasons[j].Reason
	})
	return &Diagnostics{
		Invalid:         source.Invalid,
		Banned:          source.Banned,
		QualityRejected: source.QualityRejected,
		Duplicates:      source.Duplicates,
		QualityReasons:  reasons,
	}
}

func Write(w io.Writer, artifact Artifact) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(artifact)
}
