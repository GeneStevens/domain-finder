package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gene/domain-finder/internal/match"
)

func TestWriteJSONL(t *testing.T) {
	var buf bytes.Buffer
	results := []match.CandidateResult{
		{
			Candidate:    "example",
			PresentInAny: true,
			AbsentInAll:  false,
			Zones: []match.ZonePresence{
				{Zone: "com", Present: true},
				{Zone: "net", Present: true},
			},
		},
	}

	if err := WriteJSONL(&buf, results); err != nil {
		t.Fatalf("WriteJSONL() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}

	var got match.CandidateResult
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Candidate != "example" {
		t.Fatalf("Candidate = %q, want %q", got.Candidate, "example")
	}
	if len(got.Zones) != 2 || got.Zones[0].Zone != "com" || got.Zones[1].Zone != "net" {
		t.Fatalf("zones = %#v, want deterministic order", got.Zones)
	}
	if !strings.Contains(lines[0], `"present_in_any":true`) || !strings.Contains(lines[0], `"absent_in_all":false`) {
		t.Fatalf("json line missing stable fields: %s", lines[0])
	}
}
