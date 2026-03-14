package match

import (
	"context"

	"github.com/genestevens/domain-finder/internal/backend"
)

// Classify returns a stable result for one candidate stem across all loaded zones.
func Classify(ctx context.Context, lookup backend.Lookup, candidate string) (CandidateResult, error) {
	zoneNames := lookup.ZoneNames()
	zones := make([]ZonePresence, 0, len(zoneNames))
	presentInAny := false

	for _, zoneName := range zoneNames {
		present, err := lookup.Contains(ctx, zoneName, candidate)
		if err != nil {
			return CandidateResult{}, err
		}
		zones = append(zones, ZonePresence{
			Zone:    zoneName,
			Present: present,
		})
		if present {
			presentInAny = true
		}
	}

	return CandidateResult{
		Candidate:    candidate,
		Zones:        zones,
		PresentInAny: presentInAny,
		AbsentInAll:  !presentInAny,
	}, nil
}

// ClassifyAll returns stable results for all candidates in the given order.
func ClassifyAll(ctx context.Context, lookup backend.Lookup, candidates []string) ([]CandidateResult, error) {
	results := make([]CandidateResult, 0, len(candidates))
	for _, candidate := range candidates {
		result, err := Classify(ctx, lookup, candidate)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}
