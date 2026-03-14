package index

import "github.com/gene/domain-finder/internal/zonefile"

// Exact stores a single named zone as a normalized exact-match set.
type Exact struct {
	zoneName string
	domains  map[string]struct{}
}

// NewExact creates an empty exact-match index for a specific zone name.
func NewExact(zoneName string) *Exact {
	return &Exact{
		zoneName: zoneName,
		domains:  make(map[string]struct{}),
	}
}

// ZoneName returns the explicit name assigned to this zone.
func (x *Exact) ZoneName() string {
	return x.zoneName
}

// Add stores a normalized domain. Empty values are ignored.
func (x *Exact) Add(domain string) {
	domain = zonefile.NormalizeDomain(domain)
	if domain == "" {
		return
	}
	x.domains[domain] = struct{}{}
}

// AddRecord stores the normalized domain from a parsed zone record.
func (x *Exact) AddRecord(record zonefile.Record) {
	x.Add(record.Domain)
}

// Contains reports whether the candidate exists in the zone.
func (x *Exact) Contains(candidate string) bool {
	candidate = zonefile.NormalizeDomain(candidate)
	if candidate == "" {
		return false
	}
	_, ok := x.domains[candidate]
	return ok
}

// Count returns the number of indexed domains in this zone.
func (x *Exact) Count() int {
	return len(x.domains)
}
