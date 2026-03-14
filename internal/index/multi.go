package index

import (
	"fmt"
	"sort"
)

// Multi aggregates exact-match indexes keyed by explicit zone name.
type Multi struct {
	zones map[string]*Exact
}

// NewMulti creates an empty multi-zone collection.
func NewMulti() *Multi {
	return &Multi{zones: make(map[string]*Exact)}
}

// Register adds or replaces a zone index by explicit name.
func (m *Multi) Register(index *Exact) error {
	if index == nil {
		return fmt.Errorf("register zone: nil index")
	}
	if index.ZoneName() == "" {
		return fmt.Errorf("register zone: empty zone name")
	}
	m.zones[index.ZoneName()] = index
	return nil
}

// Zone returns the registered zone index by name.
func (m *Multi) Zone(zoneName string) (*Exact, bool) {
	index, ok := m.zones[zoneName]
	return index, ok
}

// Contains reports whether the candidate exists in a specific named zone.
func (m *Multi) Contains(zoneName, candidate string) bool {
	index, ok := m.zones[zoneName]
	if !ok {
		return false
	}
	return index.Contains(candidate)
}

// ZoneNames returns registered zone names in sorted order.
func (m *Multi) ZoneNames() []string {
	names := make([]string, 0, len(m.zones))
	for zoneName := range m.zones {
		names = append(names, zoneName)
	}
	sort.Strings(names)
	return names
}
