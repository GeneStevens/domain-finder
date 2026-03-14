package audit

import (
	"encoding/json"
	"io"

	"github.com/genestevens/domain-finder/internal/match"
)

// State describes the high-level availability state of a checked stem.
type State string

const (
	StateAll     State = "all"
	StatePartial State = "partial"
	StateTaken   State = "taken"
)

// ZoneAvailability is the machine-readable per-zone availability view for one
// checked stem.
type ZoneAvailability struct {
	Zone      string `json:"zone"`
	Available bool   `json:"available"`
}

// Record is one durable audit JSONL entry for a checked stem.
type Record struct {
	Stem               string             `json:"stem"`
	Backend            string             `json:"backend"`
	RequestedZones     []string           `json:"requested_zones"`
	Zones              []ZoneAvailability `json:"zones"`
	State              State              `json:"state"`
	ReportEmitted      bool               `json:"report_emitted"`
	InteractiveEmitted bool               `json:"interactive_emitted"`
}

// Logger writes one JSONL audit record per checked stem.
type Logger struct {
	enc *json.Encoder
}

// NewLogger creates an audit JSONL logger.
func NewLogger(w io.Writer) *Logger {
	return &Logger{enc: json.NewEncoder(w)}
}

// Write emits one audit record.
func (l *Logger) Write(record Record) error {
	if l == nil || l.enc == nil {
		return nil
	}
	return l.enc.Encode(record)
}

// NewRecord derives a stable audit record from an existing classification
// result.
func NewRecord(result match.CandidateResult, backend string, requestedZones []string, reportEmitted, interactiveEmitted bool) Record {
	zones := make([]ZoneAvailability, 0, len(result.Zones))
	for _, zone := range result.Zones {
		zones = append(zones, ZoneAvailability{
			Zone:      zone.Zone,
			Available: !zone.Present,
		})
	}

	requested := append([]string(nil), requestedZones...)
	return Record{
		Stem:               result.Candidate,
		Backend:            backend,
		RequestedZones:     requested,
		Zones:              zones,
		State:              stateFor(result),
		ReportEmitted:      reportEmitted,
		InteractiveEmitted: interactiveEmitted,
	}
}

func stateFor(result match.CandidateResult) State {
	switch {
	case result.AbsentInAll:
		return StateAll
	case !hasAvailableZone(result):
		return StateTaken
	case result.PresentInAny:
		return StatePartial
	default:
		return StatePartial
	}
}

func hasAvailableZone(result match.CandidateResult) bool {
	for _, zone := range result.Zones {
		if !zone.Present {
			return true
		}
	}
	return false
}
