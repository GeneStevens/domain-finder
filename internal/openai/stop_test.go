package openai

import "testing"

func TestStopControllerStopsOnCostCap(t *testing.T) {
	controller, err := NewStopController("gpt-4o-mini", StopConditions{
		MaxAccepted: 10,
		MaxCostUSD:  0.00002,
	}, 0)
	if err != nil {
		t.Fatalf("NewStopController() error = %v", err)
	}
	stop := controller.ObserveBatch(2, 0, &Usage{InputTokens: 120, OutputTokens: 18, CachedInputTokens: 40})
	if stop == nil || stop.Reason != StopReasonCostCapReached {
		t.Fatalf("ObserveBatch() = %#v, want cost-cap stop", stop)
	}
}

func TestStopControllerStopsOnStrongHitTarget(t *testing.T) {
	controller, err := NewStopController("gpt-4o-mini", StopConditions{
		MaxAccepted:      10,
		TargetStrongHits: 3,
	}, 1)
	if err != nil {
		t.Fatalf("NewStopController() error = %v", err)
	}
	stop := controller.ObserveBatch(2, 2, nil)
	if stop == nil || stop.Reason != StopReasonStrongHitsReached {
		t.Fatalf("ObserveBatch() = %#v, want strong-hit stop", stop)
	}
}

func TestStopControllerStopsOnStallLimit(t *testing.T) {
	controller, err := NewStopController("gpt-4o-mini", StopConditions{
		MaxAccepted:     10,
		MaxStallBatches: 2,
	}, 0)
	if err != nil {
		t.Fatalf("NewStopController() error = %v", err)
	}
	if stop := controller.ObserveBatch(0, 0, nil); stop != nil {
		t.Fatalf("first stall = %#v, want nil", stop)
	}
	stop := controller.ObserveBatch(0, 0, nil)
	if stop == nil || stop.Reason != StopReasonStallReached {
		t.Fatalf("ObserveBatch() = %#v, want stall stop", stop)
	}
}

func TestNewStopControllerRejectsUnknownPricingForCostCap(t *testing.T) {
	if _, err := NewStopController("custom-model", StopConditions{MaxAccepted: 10, MaxCostUSD: 1.0}, 0); err == nil {
		t.Fatal("NewStopController() error = nil, want pricing validation failure")
	}
}
