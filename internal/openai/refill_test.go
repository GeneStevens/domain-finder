package openai

import "testing"

func TestAdaptiveRefillShrinksAfterRepeatedUnderfill(t *testing.T) {
	controller := NewAdaptiveRefillController(8, AdaptiveRefillPolicy{Enabled: true, MinBatchSize: 2})

	if got := controller.NextBatchSize(20); got != 8 {
		t.Fatalf("NextBatchSize() = %d, want 8", got)
	}
	controller.ObserveBatch(8, 5)
	if got := controller.NextBatchSize(20); got != 8 {
		t.Fatalf("NextBatchSize() after one underfill = %d, want 8", got)
	}
	controller.ObserveBatch(8, 3)
	if got := controller.NextBatchSize(20); got != 4 {
		t.Fatalf("NextBatchSize() after repeated underfill = %d, want 4", got)
	}
}

func TestAdaptiveRefillRespectsMinimumBatchSize(t *testing.T) {
	controller := NewAdaptiveRefillController(8, AdaptiveRefillPolicy{Enabled: true, MinBatchSize: 2})

	controller.ObserveBatch(8, 1)
	controller.ObserveBatch(8, 1)
	controller.ObserveBatch(4, 1)
	controller.ObserveBatch(4, 1)
	if got := controller.NextBatchSize(20); got != 2 {
		t.Fatalf("NextBatchSize() = %d, want 2", got)
	}
	controller.ObserveBatch(2, 0)
	controller.ObserveBatch(2, 0)
	if got := controller.NextBatchSize(20); got != 2 {
		t.Fatalf("NextBatchSize() after extra underfill = %d, want min 2", got)
	}
}

func TestAdaptiveRefillDisabledKeepsBaseBatchSize(t *testing.T) {
	controller := NewAdaptiveRefillController(8, AdaptiveRefillPolicy{Enabled: false, MinBatchSize: 2})

	controller.ObserveBatch(8, 1)
	controller.ObserveBatch(8, 1)
	if got := controller.NextBatchSize(20); got != 8 {
		t.Fatalf("NextBatchSize() = %d, want 8", got)
	}
}
