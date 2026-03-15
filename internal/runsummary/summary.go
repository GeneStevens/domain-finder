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
	Model                   string   `json:"model"`
	Prompt                  string   `json:"prompt"`
	Style                   string   `json:"style,omitempty"`
	GenerateCount           int      `json:"generate_count"`
	BatchSize               int      `json:"batch_size"`
	AdaptiveRefill          bool     `json:"adaptive_refill,omitempty"`
	MinBatchSize            int      `json:"min_batch_size,omitempty"`
	FinalEffectiveBatchSize int      `json:"final_effective_batch_size,omitempty"`
	MaxAttempts             int      `json:"max_attempts"`
	RetryCount              int      `json:"retry_count"`
	QualityProfile          string   `json:"quality_profile,omitempty"`
	MinLength               int      `json:"min_length,omitempty"`
	AvoidSubstrings         []string `json:"avoid_substrings,omitempty"`
	AvoidPrefixes           []string `json:"avoid_prefixes,omitempty"`
	AvoidSuffixes           []string `json:"avoid_suffixes,omitempty"`
	MaxCostUSD              float64  `json:"max_cost_usd,omitempty"`
	TargetAvailableHits     int      `json:"target_available_hits,omitempty"`
	TargetStrongHits        int      `json:"target_strong_hits,omitempty"`
	MaxStallBatches         int      `json:"max_stall_batches,omitempty"`
	AcceptedCount           int      `json:"accepted_count"`
	AvailableHits           int      `json:"available_hits,omitempty"`
	UnderfilledBatches      int      `json:"underfilled_batches,omitempty"`
	UnderfilledStems        int      `json:"underfilled_stems,omitempty"`
	StopReason              string   `json:"stop_reason,omitempty"`
	InputTokens             int      `json:"input_tokens,omitempty"`
	OutputTokens            int      `json:"output_tokens,omitempty"`
	CachedInputTokens       int      `json:"cached_input_tokens,omitempty"`
	PricingAvailable        bool     `json:"pricing_available"`
	EstimatedCostUSD        float64  `json:"estimated_cost_usd,omitempty"`
}

type Diagnostics struct {
	Invalid          int           `json:"invalid"`
	Banned           int           `json:"banned"`
	TooShort         int           `json:"too_short,omitempty"`
	BannedSubstrings int           `json:"banned_substrings,omitempty"`
	BannedPrefixes   int           `json:"banned_prefixes,omitempty"`
	BannedSuffixes   int           `json:"banned_suffixes,omitempty"`
	QualityRejected  int           `json:"quality_rejected"`
	FamilyRejected   int           `json:"family_rejected,omitempty"`
	Duplicates       int           `json:"duplicates"`
	QualityReasons   []ReasonCount `json:"quality_reasons,omitempty"`
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
		Invalid:          source.Invalid,
		Banned:           source.Banned,
		TooShort:         source.TooShort,
		BannedSubstrings: source.BannedSubstrings,
		BannedPrefixes:   source.BannedPrefixes,
		BannedSuffixes:   source.BannedSuffixes,
		QualityRejected:  source.QualityRejected,
		FamilyRejected:   source.FamilyRejected,
		Duplicates:       source.Duplicates,
		QualityReasons:   reasons,
	}
}

func Write(w io.Writer, artifact Artifact) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(artifact)
}
