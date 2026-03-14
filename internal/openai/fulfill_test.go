package openai

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/genestevens/domain-finder/internal/candidates"
	"github.com/genestevens/domain-finder/internal/config"
)

type scriptedGenerator struct {
	responses []scriptedResponse
	calls     []int
	model     string
}

type scriptedResponse struct {
	result BatchResult
	err    error
}

func (g *scriptedGenerator) GenerateBatch(_ context.Context, _ string, count int) (BatchResult, error) {
	g.calls = append(g.calls, count)
	if len(g.responses) == 0 {
		return BatchResult{}, fmt.Errorf("unexpected GenerateBatch call")
	}
	response := g.responses[0]
	g.responses = g.responses[1:]
	return response.result, response.err
}

func (g *scriptedGenerator) ModelName() string {
	return g.model
}

func TestFulfillDuplicateHeavyBatch(t *testing.T) {
	collector := candidates.NewCollector()
	if _, err := collector.AddAll([]string{"brandfoo"}); err != nil {
		t.Fatal(err)
	}
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{result: BatchResult{Stems: []string{"brandfoo", "brandfoo", "brandfoo"}}},
			{result: BatchResult{Stems: []string{"noviq", "trynex"}}},
		},
	}
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var accepted []string
	_, _, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddAllReportLimited(batch.Stems, limit)
		accepted = append(accepted, report.Accepted...)
		return report, nil
	}, nil)
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if !reflect.DeepEqual(accepted, []string{"noviq", "trynex"}) {
		t.Fatalf("accepted = %#v, want [noviq trynex]", accepted)
	}
	if !reflect.DeepEqual(generator.calls, []int{2, 2}) {
		t.Fatalf("calls = %#v, want [2 2]", generator.calls)
	}
}

func TestFulfillRetryThenSuccess(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{err: &GenerationError{Kind: ErrorTransient, Message: "rate limited", StatusCode: 429}},
			{result: BatchResult{Stems: []string{"brandfoo", "noviq"}}},
		},
	}
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          1,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var events []Event
	_, _, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		return candidates.NewCollector().AddAllReportLimited(batch.Stems, limit), nil
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if len(generator.calls) != 2 || generator.calls[0] != 2 || generator.calls[1] != 2 {
		t.Fatalf("calls = %#v, want [2 2]", generator.calls)
	}
	if len(events) < 3 || events[1].Type != EventRetry || events[len(events)-1].Type != EventComplete {
		t.Fatalf("events = %#v, want retry and complete events", events)
	}
}

func TestFulfillRetryThenFail(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{err: &GenerationError{Kind: ErrorTransient, Message: "rate limited", StatusCode: 429}},
			{err: &GenerationError{Kind: ErrorTransient, Message: "still rate limited", StatusCode: 429}},
		},
	}
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          1,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	_, _, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		return candidates.NewCollector().AddAllReportLimited(batch.Stems, limit), nil
	}, nil)
	if err == nil {
		t.Fatal("Fulfill() error = nil, want failure")
	}
	var fulfillmentErr *FulfillmentError
	if !errors.As(err, &fulfillmentErr) {
		t.Fatalf("Fulfill() error = %v, want FulfillmentError", err)
	}
	if fulfillmentErr.Accepted != 0 || fulfillmentErr.Requested != 2 {
		t.Fatalf("FulfillmentError = %#v, want accepted 0 requested 2", fulfillmentErr)
	}
}

func TestFulfillUnderfilledBatchContinues(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{result: BatchResult{Stems: []string{"brandfoo", "brandfoo"}}},
			{result: BatchResult{Stems: []string{"brandfoo", "brandfoo"}}},
			{result: BatchResult{Stems: []string{"noviq", "traktor"}}},
		},
	}
	collector := candidates.NewCollector()
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var accepted []string
	var events []Event
	_, underfills, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddAllReportLimited(batch.Stems, limit)
		accepted = append(accepted, report.Accepted...)
		return report, nil
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if !reflect.DeepEqual(accepted, []string{"brandfoo", "noviq"}) {
		t.Fatalf("accepted = %#v, want [brandfoo noviq]", accepted)
	}
	if underfills.Batches != 1 || underfills.Stems != 1 {
		t.Fatalf("underfills = %#v, want 1 batch and 1 stem", underfills)
	}
	foundUnderfill := false
	for _, event := range events {
		if event.Type == EventBatchResult && event.Underfilled == 1 {
			foundUnderfill = true
		}
	}
	if !foundUnderfill {
		t.Fatalf("events = %#v, want underfilled batch event", events)
	}
}

func TestFulfillMultipleUnderfilledBatchesContinueUntilProgress(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{result: BatchResult{Stems: []string{"bad.name", "still bad"}}},
			{result: BatchResult{Stems: []string{"also bad", "bad.again"}}},
			{result: BatchResult{Stems: []string{"noviq", "traktor"}}},
		},
	}
	collector := candidates.NewCollector()
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var accepted []string
	_, underfills, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddAllReportLimited(batch.Stems, limit)
		accepted = append(accepted, report.Accepted...)
		return report, nil
	}, nil)
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if !reflect.DeepEqual(accepted, []string{"noviq", "traktor"}) {
		t.Fatalf("accepted = %#v, want [noviq traktor]", accepted)
	}
	if underfills.Batches != 1 || underfills.Stems != 2 {
		t.Fatalf("underfills = %#v, want 1 batch and 2 stems", underfills)
	}
}

func TestFulfillBannedSubstringsAreRejected(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{result: BatchResult{Stems: []string{"devspark", "noviq", "cloudbase"}}},
			{result: BatchResult{Stems: []string{"trynex"}}},
		},
	}
	collector := candidates.NewCollector()
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
		AvoidSubstrings:     []string{"dev", "cloud"},
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var accepted []string
	var events []Event
	_, _, _, err := fulfiller.Fulfill(context.Background(), "short brand stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddGeneratedReportLimited(batch.Stems, limit, candidates.GeneratedPolicy{
			AvoidSubstrings: []string{"dev", "cloud"},
		})
		accepted = append(accepted, report.Accepted...)
		return report, nil
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if !reflect.DeepEqual(accepted, []string{"noviq", "trynex"}) {
		t.Fatalf("accepted = %#v, want [noviq trynex]", accepted)
	}
	if len(events) < 3 || events[1].Banned != 2 {
		t.Fatalf("events = %#v, want lexical rejection accounting", events)
	}
}

func TestFulfillQualityRejectedStems(t *testing.T) {
	generator := &scriptedGenerator{
		responses: []scriptedResponse{
			{result: BatchResult{Stems: []string{"veloria", "theravia", "noviq"}}},
			{result: BatchResult{Stems: []string{"traktor"}}},
		},
	}
	collector := candidates.NewCollector()
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
		QualityProfile:      "industrial",
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var accepted []string
	var events []Event
	_, _, _, err := fulfiller.Fulfill(context.Background(), "industrial infrastructure stems", 2, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddGeneratedReportLimited(batch.Stems, limit, candidates.GeneratedPolicy{
			QualityProfile: "industrial",
		})
		accepted = append(accepted, report.Accepted...)
		return report, nil
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if !reflect.DeepEqual(accepted, []string{"noviq", "traktor"}) {
		t.Fatalf("accepted = %#v, want [noviq traktor]", accepted)
	}
	if len(events) < 3 || events[1].QualityRejected != 2 {
		t.Fatalf("events = %#v, want quality rejection accounting", events)
	}
}

func TestFulfillAccumulatesUsageTotals(t *testing.T) {
	generator := &scriptedGenerator{
		model: "gpt-4o-mini",
		responses: []scriptedResponse{
			{result: BatchResult{
				Stems: []string{"noviq", "traktor"},
				Usage: &Usage{InputTokens: 120, OutputTokens: 18, CachedInputTokens: 40},
			}},
			{result: BatchResult{
				Stems: []string{"cinder"},
				Usage: &Usage{InputTokens: 80, OutputTokens: 12},
			}},
		},
	}
	collector := candidates.NewCollector()
	fulfiller := NewFulfiller(generator, config.GenerateConfig{
		BatchSize:           2,
		MaxAttemptsPerBatch: 2,
		RetryCount:          0,
	})
	fulfiller.Sleep = func(context.Context, time.Duration) error { return nil }

	var events []Event
	totals, _, _, err := fulfiller.Fulfill(context.Background(), "industrial infrastructure stems", 3, func(batch BatchResult, limit int) (candidates.BatchReport, error) {
		return collector.AddGeneratedReportLimited(batch.Stems, limit, candidates.GeneratedPolicy{}), nil
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Fulfill() error = %v", err)
	}
	if totals.InputTokens != 200 || totals.OutputTokens != 30 || totals.CachedInputTokens != 40 {
		t.Fatalf("totals = %#v, want accumulated usage", totals)
	}
	if !totals.PricingAvailable || totals.EstimatedCostUSD <= 0 {
		t.Fatalf("totals = %#v, want pricing and cost", totals)
	}
	if len(events) < 4 || !events[1].LastEstimate.PricingAvailable || !events[len(events)-1].Totals.PricingAvailable {
		t.Fatalf("events = %#v, want usage estimates in batch and completion events", events)
	}
}
