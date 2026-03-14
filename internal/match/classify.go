package match

import (
	"github.com/gene/domain-finder/internal/index"
	"github.com/gene/domain-finder/internal/zonefile"
)

// Classify returns a stable result for one candidate across all loaded zones.
func Classify(multi *index.Multi, candidate string) CandidateResult {
	normalized := zonefile.NormalizeDomain(candidate)
	zoneNames := multi.ZoneNames()
	zones := make([]ZonePresence, 0, len(zoneNames))
	presentInAny := false

	for _, zoneName := range zoneNames {
		present := multi.Contains(zoneName, normalized)
		zones = append(zones, ZonePresence{
			Zone:    zoneName,
			Present: present,
		})
		if present {
			presentInAny = true
		}
	}

	return CandidateResult{
		Candidate:    normalized,
		Zones:        zones,
		PresentInAny: presentInAny,
		AbsentInAll:  !presentInAny,
	}
}

// ClassifyAll returns stable results for all candidates in the given order.
func ClassifyAll(multi *index.Multi, candidates []string) []CandidateResult {
	results := make([]CandidateResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, Classify(multi, candidate))
	}
	return results
}
