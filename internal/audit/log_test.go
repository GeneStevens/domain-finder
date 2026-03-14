package audit

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/genestevens/domain-finder/internal/match"
)

func TestNewRecord(t *testing.T) {
	record := NewRecord(match.CandidateResult{
		Candidate:    "example",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: false},
		},
	}, "file", []string{"com", "net"}, true, false)

	if record.Stem != "example" || record.Backend != "file" {
		t.Fatalf("record = %#v, want stem/backend populated", record)
	}
	if record.State != StatePartial {
		t.Fatalf("State = %q, want partial", record.State)
	}
	if len(record.Zones) != 2 || record.Zones[0].Available || !record.Zones[1].Available {
		t.Fatalf("Zones = %#v, want availability derived from presence", record.Zones)
	}
	if !record.ReportEmitted || record.InteractiveEmitted {
		t.Fatalf("record flags = %#v, want report=true interactive=false", record)
	}
}

func TestNewRecordTakenState(t *testing.T) {
	record := NewRecord(match.CandidateResult{
		Candidate:    "takenstem",
		PresentInAny: true,
		Zones: []match.ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: true},
		},
	}, "file", []string{"com", "net"}, true, false)

	if record.State != StateTaken {
		t.Fatalf("State = %q, want taken", record.State)
	}
}

func TestLoggerWritesJSONL(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	err := logger.Write(Record{
		Stem:               "missing",
		Backend:            "file",
		RequestedZones:     []string{"com", "net"},
		Zones:              []ZoneAvailability{{Zone: "com", Available: true}, {Zone: "net", Available: true}},
		State:              StateAll,
		ReportEmitted:      true,
		InteractiveEmitted: true,
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var got Record
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Stem != "missing" || got.State != StateAll || !got.InteractiveEmitted {
		t.Fatalf("got = %#v, want round-tripped record", got)
	}
}
