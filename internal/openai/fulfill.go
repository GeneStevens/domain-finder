package openai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/genestevens/domain-finder/internal/candidates"
	"github.com/genestevens/domain-finder/internal/config"
)

type ErrorKind string

const (
	ErrorTransient ErrorKind = "transient"
	ErrorProtocol  ErrorKind = "protocol"
	ErrorQuality   ErrorKind = "quality"
)

const defaultRetryBackoff = 100 * time.Millisecond

// GenerationError classifies OpenAI generation failures so the fulfillment
// policy can distinguish retryable transport/API issues from poor model output.
type GenerationError struct {
	Kind       ErrorKind
	Message    string
	StatusCode int
	Err        error
}

func (e *GenerationError) Error() string {
	switch {
	case e == nil:
		return "<nil>"
	case e.StatusCode > 0 && e.Message != "":
		return fmt.Sprintf("%s (status %d): %s", e.Kind, e.StatusCode, e.Message)
	case e.Message != "":
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	case e.Err != nil:
		return fmt.Sprintf("%s: %v", e.Kind, e.Err)
	default:
		return string(e.Kind)
	}
}

func (e *GenerationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsTransient(err error) bool {
	var genErr *GenerationError
	return errors.As(err, &genErr) && genErr.Kind == ErrorTransient
}

func IsQuality(err error) bool {
	var genErr *GenerationError
	return errors.As(err, &genErr) && genErr.Kind == ErrorQuality
}

// Policy controls bounded generation fulfillment.
type Policy struct {
	BatchSize           int
	MaxAttemptsPerBatch int
	RetryCount          int
	RetryBackoff        time.Duration
}

// PolicyFromConfig builds a generation policy from resolved config.
func PolicyFromConfig(cfg config.GenerateConfig) Policy {
	return Policy{
		BatchSize:           cfg.BatchSize,
		MaxAttemptsPerBatch: cfg.MaxAttemptsPerBatch,
		RetryCount:          cfg.RetryCount,
		RetryBackoff:        defaultRetryBackoff,
	}
}

type EventType string

const (
	EventBatchRequest EventType = "batch_request"
	EventBatchResult  EventType = "batch_result"
	EventRetry        EventType = "retry"
	EventFailed       EventType = "failed"
	EventComplete     EventType = "complete"
)

// Event reports concise batch-generation progress.
type Event struct {
	Type            EventType
	Batch           int
	Attempt         int
	Requested       int
	Accepted        int
	Invalid         int
	Banned          int
	QualityRejected int
	Duplicates      int
	RemainingBatch  int
	RemainingTotal  int
	Retry           int
	RetryCount      int
	Err             error
}

// FulfillmentError reports that bounded generation could not satisfy the request.
type FulfillmentError struct {
	Requested int
	Accepted  int
	Cause     error
}

func (e *FulfillmentError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("generation produced %d usable stems out of %d requested: %v", e.Accepted, e.Requested, e.Cause)
}

func (e *FulfillmentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Fulfiller makes a bounded effort to satisfy one generation request.
type Fulfiller struct {
	Generator StemGenerator
	Policy    Policy
	Sleep     func(context.Context, time.Duration) error
}

// NewFulfiller creates a batch fulfiller with default timing behavior.
func NewFulfiller(generator StemGenerator, cfg config.GenerateConfig) *Fulfiller {
	return &Fulfiller{
		Generator: generator,
		Policy:    PolicyFromConfig(cfg),
		Sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

// Fulfill requests generated stems until the requested total is satisfied or
// the bounded attempt policy is exhausted.
func (f *Fulfiller) Fulfill(ctx context.Context, prompt string, totalRequested int, accept func([]string, int) (candidates.BatchReport, error), notify func(Event) error) error {
	if totalRequested <= 0 {
		return nil
	}
	totalAccepted := 0
	batchNumber := 0

	for remainingTotal := totalRequested; remainingTotal > 0; {
		batchNumber++
		target := remainingTotal
		if f.Policy.BatchSize > 0 && f.Policy.BatchSize < target {
			target = f.Policy.BatchSize
		}
		batchAccepted := 0
		var lastErr error

		for attempt := 1; attempt <= max(1, f.Policy.MaxAttemptsPerBatch) && batchAccepted < target; attempt++ {
			need := target - batchAccepted
			if err := notifyEvent(notify, Event{
				Type:           EventBatchRequest,
				Batch:          batchNumber,
				Attempt:        attempt,
				Requested:      need,
				RemainingBatch: need,
				RemainingTotal: remainingTotal,
			}); err != nil {
				return err
			}

			rawBatch, err := f.requestWithRetry(ctx, prompt, need, batchNumber, attempt, notify)
			if err != nil {
				lastErr = err
				if IsQuality(err) {
					if err := notifyEvent(notify, Event{
						Type:           EventBatchResult,
						Batch:          batchNumber,
						Attempt:        attempt,
						Requested:      need,
						RemainingBatch: need,
						RemainingTotal: remainingTotal,
						Err:            err,
					}); err != nil {
						return err
					}
					continue
				}
				if err := notifyEvent(notify, Event{
					Type:           EventFailed,
					Batch:          batchNumber,
					Attempt:        attempt,
					Requested:      need,
					RemainingBatch: need,
					RemainingTotal: remainingTotal,
					Err:            err,
				}); err != nil {
					return err
				}
				return &FulfillmentError{Requested: totalRequested, Accepted: totalAccepted, Cause: err}
			}

			report, err := accept(rawBatch, need)
			if err != nil {
				return err
			}
			batchAccepted += len(report.Accepted)
			totalAccepted += len(report.Accepted)
			remainingTotal = totalRequested - totalAccepted

			if err := notifyEvent(notify, Event{
				Type:            EventBatchResult,
				Batch:           batchNumber,
				Attempt:         attempt,
				Requested:       need,
				Accepted:        len(report.Accepted),
				Invalid:         report.Invalid,
				Banned:          report.LexicalRejected,
				QualityRejected: report.QualityRejected,
				Duplicates:      report.Duplicates,
				RemainingBatch:  target - batchAccepted,
				RemainingTotal:  remainingTotal,
			}); err != nil {
				return err
			}
		}

		if batchAccepted < target {
			if lastErr == nil {
				lastErr = &GenerationError{Kind: ErrorQuality, Message: "batch attempts exhausted without enough usable stems"}
			}
			err := &FulfillmentError{Requested: totalRequested, Accepted: totalAccepted, Cause: lastErr}
			if notifyErr := notifyEvent(notify, Event{
				Type:           EventFailed,
				Batch:          batchNumber,
				Accepted:       batchAccepted,
				RemainingBatch: target - batchAccepted,
				RemainingTotal: totalRequested - totalAccepted,
				Err:            err,
			}); notifyErr != nil {
				return notifyErr
			}
			return err
		}
	}

	return notifyEvent(notify, Event{Type: EventComplete, Accepted: totalAccepted})
}

func (f *Fulfiller) requestWithRetry(ctx context.Context, prompt string, count, batch, attempt int, notify func(Event) error) ([]string, error) {
	var lastErr error
	for retry := 0; retry <= max(0, f.Policy.RetryCount); retry++ {
		rawBatch, err := f.Generator.GenerateBatch(ctx, prompt, count)
		if err == nil {
			return rawBatch, nil
		}
		lastErr = err
		if !IsTransient(err) || retry == f.Policy.RetryCount {
			return nil, err
		}
		if err := notifyEvent(notify, Event{
			Type:       EventRetry,
			Batch:      batch,
			Attempt:    attempt,
			Requested:  count,
			Retry:      retry + 1,
			RetryCount: f.Policy.RetryCount,
			Err:        err,
		}); err != nil {
			return nil, err
		}
		if sleep := f.Sleep; sleep != nil {
			if err := sleep(ctx, f.Policy.RetryBackoff); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func notifyEvent(notify func(Event) error, event Event) error {
	if notify == nil {
		return nil
	}
	return notify(event)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
