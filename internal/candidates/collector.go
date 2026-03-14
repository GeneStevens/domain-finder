package candidates

// Collector incrementally normalizes and deduplicates candidate stems while
// preserving first-seen order.
type Collector struct {
	seen map[string]struct{}
}

// NewCollector creates an empty collector.
func NewCollector() *Collector {
	return &Collector{seen: make(map[string]struct{})}
}

// Add normalizes a raw candidate and returns the normalized stem if it was new.
func (c *Collector) Add(raw string) (string, bool, error) {
	normalized, err := NormalizeCandidate(raw)
	if err != nil {
		return "", false, err
	}
	if _, ok := c.seen[normalized]; ok {
		return normalized, false, nil
	}
	c.seen[normalized] = struct{}{}
	return normalized, true, nil
}

// AddAll adds many raw candidates and returns only the newly accepted stems in input order.
func (c *Collector) AddAll(raws []string) ([]string, error) {
	out := make([]string, 0, len(raws))
	for _, raw := range raws {
		normalized, added, err := c.Add(raw)
		if err != nil {
			return nil, err
		}
		if added {
			out = append(out, normalized)
		}
	}
	return out, nil
}
