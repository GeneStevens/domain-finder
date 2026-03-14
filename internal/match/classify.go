package match

import (
	"fmt"

	"github.com/genestevens/domain-finder/internal/index"
)

// ComposeLookupName builds the FQDN checked for a candidate stem in one zone.
func ComposeLookupName(candidate, zone string) string {
	return fmt.Sprintf("%s.%s", candidate, zone)
}

// Classify returns a stable result for one candidate stem across all loaded zones.
func Classify(multi *index.Multi, candidate string) CandidateResult {
	zoneNames := multi.ZoneNames()
	zones := make([]ZonePresence, 0, len(zoneNames))
	presentInAny := false

	for _, zoneName := range zoneNames {
		present := multi.Contains(zoneName, ComposeLookupName(candidate, zoneName))
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
