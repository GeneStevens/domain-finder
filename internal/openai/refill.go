package openai

type AdaptiveRefillPolicy struct {
	Enabled      bool
	MinBatchSize int
}

type AdaptiveRefillSnapshot struct {
	Enabled            bool
	BaseBatchSize      int
	EffectiveBatchSize int
	MinBatchSize       int
}

type AdaptiveRefillController struct {
	policy                AdaptiveRefillPolicy
	baseBatchSize         int
	effectiveBatchSize    int
	minBatchSize          int
	consecutiveUnderfills int
}

func NewAdaptiveRefillController(batchSize int, policy AdaptiveRefillPolicy) *AdaptiveRefillController {
	if batchSize <= 0 {
		batchSize = 1
	}
	minBatchSize := policy.MinBatchSize
	if minBatchSize <= 0 {
		minBatchSize = 2
	}
	if minBatchSize > batchSize {
		minBatchSize = batchSize
	}
	return &AdaptiveRefillController{
		policy:             policy,
		baseBatchSize:      batchSize,
		effectiveBatchSize: batchSize,
		minBatchSize:       minBatchSize,
	}
}

func (c *AdaptiveRefillController) NextBatchSize(remaining int) int {
	if c == nil {
		if remaining <= 0 {
			return 0
		}
		return remaining
	}
	target := remaining
	if c.effectiveBatchSize > 0 && c.effectiveBatchSize < target {
		target = c.effectiveBatchSize
	}
	if target < 0 {
		return 0
	}
	return target
}

func (c *AdaptiveRefillController) ObserveBatch(target, accepted int) AdaptiveRefillSnapshot {
	if c == nil {
		return AdaptiveRefillSnapshot{}
	}
	if !c.policy.Enabled {
		return c.Snapshot()
	}
	if accepted < target {
		c.consecutiveUnderfills++
		if c.consecutiveUnderfills >= 2 && c.effectiveBatchSize > c.minBatchSize {
			next := c.effectiveBatchSize / 2
			if next < c.minBatchSize {
				next = c.minBatchSize
			}
			c.effectiveBatchSize = next
			c.consecutiveUnderfills = 0
		}
		return c.Snapshot()
	}
	c.consecutiveUnderfills = 0
	return c.Snapshot()
}

func (c *AdaptiveRefillController) Snapshot() AdaptiveRefillSnapshot {
	if c == nil {
		return AdaptiveRefillSnapshot{}
	}
	return AdaptiveRefillSnapshot{
		Enabled:            c.policy.Enabled,
		BaseBatchSize:      c.baseBatchSize,
		EffectiveBatchSize: c.effectiveBatchSize,
		MinBatchSize:       c.minBatchSize,
	}
}
