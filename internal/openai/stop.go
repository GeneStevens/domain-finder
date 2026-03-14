package openai

import "fmt"

type StopReason string

const (
	StopReasonCountReached         StopReason = "count_reached"
	StopReasonCostCapReached       StopReason = "cost_cap_reached"
	StopReasonAvailableHitsReached StopReason = "available_hit_target_reached"
	StopReasonStrongHitsReached    StopReason = "strong_hit_target_reached"
	StopReasonStallReached         StopReason = "stall_limit_reached"
)

type StopConditions struct {
	MaxAccepted         int
	MaxCostUSD          float64
	TargetAvailableHits int
	TargetStrongHits    int
	MaxStallBatches     int
}

type StopSnapshot struct {
	Reason              StopReason
	Accepted            int
	AvailableHits       int
	StrongHits          int
	StallBatches        int
	MaxAccepted         int
	MaxCostUSD          float64
	TargetAvailableHits int
	TargetStrongHits    int
	MaxStallBatches     int
	PricingAvailable    bool
	EstimatedCostUSD    float64
}

type StopController struct {
	model      string
	conditions StopConditions
	accepted   int
	available  int
	strongHits int
	stall      int
	totals     UsageTotals
}

type StopError struct {
	Snapshot StopSnapshot
}

func (e *StopError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("generation stop requested: %s", e.Snapshot.Reason)
}

func NewStopController(model string, conditions StopConditions, initialAvailableHits, initialStrongHits int) (*StopController, error) {
	if conditions.MaxCostUSD > 0 && !HasPricing(model) {
		return nil, fmt.Errorf("generate max cost requires known pricing for model %q", model)
	}
	controller := &StopController{
		model:      model,
		conditions: conditions,
		available:  initialAvailableHits,
		strongHits: initialStrongHits,
	}
	return controller, nil
}

func (c *StopController) Snapshot() StopSnapshot {
	if c == nil {
		return StopSnapshot{}
	}
	return StopSnapshot{
		Accepted:            c.accepted,
		AvailableHits:       c.available,
		StrongHits:          c.strongHits,
		StallBatches:        c.stall,
		MaxAccepted:         c.conditions.MaxAccepted,
		MaxCostUSD:          c.conditions.MaxCostUSD,
		TargetAvailableHits: c.conditions.TargetAvailableHits,
		TargetStrongHits:    c.conditions.TargetStrongHits,
		MaxStallBatches:     c.conditions.MaxStallBatches,
		PricingAvailable:    c.totals.PricingAvailable,
		EstimatedCostUSD:    c.totals.EstimatedCostUSD,
	}
}

func (c *StopController) InitialDecision() *StopSnapshot {
	if c == nil {
		return nil
	}
	return c.check("")
}

func (c *StopController) ObserveBatch(accepted int, availableHitDelta int, strongHitDelta int, usage *Usage) *StopSnapshot {
	if c == nil {
		return nil
	}
	c.accepted += accepted
	c.available += availableHitDelta
	c.strongHits += strongHitDelta
	if accepted == 0 && strongHitDelta == 0 {
		c.stall++
	} else {
		c.stall = 0
	}
	c.totals.AddCall(c.model, usage)
	return c.check("")
}

func (c *StopController) check(_ string) *StopSnapshot {
	snapshot := c.Snapshot()
	switch {
	case snapshot.MaxCostUSD > 0 && snapshot.PricingAvailable && snapshot.EstimatedCostUSD >= snapshot.MaxCostUSD:
		snapshot.Reason = StopReasonCostCapReached
		return &snapshot
	case snapshot.TargetAvailableHits > 0 && snapshot.AvailableHits >= snapshot.TargetAvailableHits:
		snapshot.Reason = StopReasonAvailableHitsReached
		return &snapshot
	case snapshot.TargetStrongHits > 0 && snapshot.StrongHits >= snapshot.TargetStrongHits:
		snapshot.Reason = StopReasonStrongHitsReached
		return &snapshot
	case snapshot.MaxStallBatches > 0 && snapshot.StallBatches >= snapshot.MaxStallBatches:
		snapshot.Reason = StopReasonStallReached
		return &snapshot
	case snapshot.MaxAccepted > 0 && snapshot.Accepted >= snapshot.MaxAccepted:
		snapshot.Reason = StopReasonCountReached
		return &snapshot
	default:
		return nil
	}
}

func HasPricing(model string) bool {
	_, ok := pricingTable[model]
	return ok
}

func StopReasonLabel(reason StopReason) string {
	switch reason {
	case StopReasonCostCapReached:
		return "cost cap reached"
	case StopReasonAvailableHitsReached:
		return "available-hit target reached"
	case StopReasonStrongHitsReached:
		return "strong-hit target reached"
	case StopReasonStallReached:
		return "stall limit reached"
	case StopReasonCountReached:
		return "generate count reached"
	default:
		return "(none)"
	}
}
