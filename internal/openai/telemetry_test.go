package openai

import (
	"math"
	"testing"
)

func TestEstimateUsageUnknownModel(t *testing.T) {
	got := EstimateUsage("custom-model", Usage{InputTokens: 100, OutputTokens: 20})
	if got.PricingAvailable {
		t.Fatalf("EstimateUsage() = %#v, want pricing unavailable", got)
	}
	if got.CostUSD != 0 {
		t.Fatalf("EstimateUsage() cost = %f, want 0", got.CostUSD)
	}
}

func TestUsageTotalsAddCallAccumulates(t *testing.T) {
	var totals UsageTotals
	first := totals.AddCall("gpt-4o-mini", &Usage{
		InputTokens:       120,
		OutputTokens:      18,
		CachedInputTokens: 40,
	})
	second := totals.AddCall("gpt-4o-mini", &Usage{
		InputTokens:       80,
		OutputTokens:      12,
		CachedInputTokens: 0,
	})

	if totals.Calls != 2 || totals.CallsWithUsage != 2 {
		t.Fatalf("totals = %#v, want 2 calls with usage", totals)
	}
	if totals.InputTokens != 200 || totals.OutputTokens != 30 || totals.CachedInputTokens != 40 {
		t.Fatalf("totals = %#v, want accumulated tokens", totals)
	}
	want := first.CostUSD + second.CostUSD
	if math.Abs(totals.EstimatedCostUSD-want) > 1e-12 {
		t.Fatalf("EstimatedCostUSD = %f, want %f", totals.EstimatedCostUSD, want)
	}
}

func TestUsageTotalsAddCallWithoutUsage(t *testing.T) {
	var totals UsageTotals
	estimate := totals.AddCall("gpt-4o-mini", nil)
	if totals.Calls != 1 || totals.CallsWithUsage != 0 {
		t.Fatalf("totals = %#v, want one call without usage", totals)
	}
	if estimate.PricingAvailable || estimate.CostUSD != 0 {
		t.Fatalf("estimate = %#v, want zero-cost unavailable estimate", estimate)
	}
}
