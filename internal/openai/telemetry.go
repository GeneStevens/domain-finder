package openai

import "fmt"

// Usage captures token accounting returned by the OpenAI API.
type Usage struct {
	InputTokens       int
	OutputTokens      int
	CachedInputTokens int
}

// BatchResult carries one generated batch plus any available API usage data.
type BatchResult struct {
	Stems []string
	Usage *Usage
}

type Pricing struct {
	InputPerMillionUSD       float64
	CachedInputPerMillionUSD float64
	OutputPerMillionUSD      float64
}

type UsageEstimate struct {
	Model            string
	Usage            Usage
	PricingAvailable bool
	CostUSD          float64
}

type UsageTotals struct {
	Model             string
	Calls             int
	CallsWithUsage    int
	InputTokens       int
	OutputTokens      int
	CachedInputTokens int
	PricingAvailable  bool
	EstimatedCostUSD  float64
}

var pricingTable = map[string]Pricing{
	"gpt-4o-mini": {
		InputPerMillionUSD:       0.15,
		CachedInputPerMillionUSD: 0.075,
		OutputPerMillionUSD:      0.60,
	},
	"gpt-4o": {
		InputPerMillionUSD:       2.50,
		CachedInputPerMillionUSD: 1.25,
		OutputPerMillionUSD:      10.00,
	},
}

func EstimateUsage(model string, usage Usage) UsageEstimate {
	estimate := UsageEstimate{
		Model: model,
		Usage: usage,
	}
	pricing, ok := pricingTable[model]
	if !ok {
		return estimate
	}
	estimate.PricingAvailable = true
	uncachedInput := usage.InputTokens - usage.CachedInputTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}
	estimate.CostUSD =
		(float64(uncachedInput) * pricing.InputPerMillionUSD / 1_000_000.0) +
			(float64(usage.CachedInputTokens) * pricing.CachedInputPerMillionUSD / 1_000_000.0) +
			(float64(usage.OutputTokens) * pricing.OutputPerMillionUSD / 1_000_000.0)
	return estimate
}

func (t *UsageTotals) AddCall(model string, usage *Usage) UsageEstimate {
	t.Model = model
	t.Calls++
	if usage == nil {
		return UsageEstimate{Model: model}
	}
	t.CallsWithUsage++
	t.InputTokens += usage.InputTokens
	t.OutputTokens += usage.OutputTokens
	t.CachedInputTokens += usage.CachedInputTokens
	estimate := EstimateUsage(model, *usage)
	if estimate.PricingAvailable {
		t.PricingAvailable = true
		t.EstimatedCostUSD += estimate.CostUSD
	}
	return estimate
}

func (t UsageTotals) HasUsage() bool {
	return t.CallsWithUsage > 0
}

func FormatCostUSD(value float64) string {
	return fmt.Sprintf("$%.6f", value)
}
