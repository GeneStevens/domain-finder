package match

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type fakeLookup struct {
	zones   []string
	present map[string]bool
}

func (f fakeLookup) ZoneNames() []string {
	return append([]string(nil), f.zones...)
}

func (f fakeLookup) Contains(_ context.Context, zone, stem string) (bool, error) {
	return f.present[zone+"/"+stem], nil
}

type errLookup struct{}

func (errLookup) ZoneNames() []string { return []string{"com"} }
func (errLookup) Contains(_ context.Context, zone, stem string) (bool, error) {
	return false, fmt.Errorf("boom for %s/%s", zone, stem)
}

func TestClassifyAcrossMultipleZones(t *testing.T) {
	got, err := Classify(context.Background(), fakeLookup{
		zones: []string{"com", "net"},
		present: map[string]bool{
			"com/example": true,
			"net/example": true,
		},
	}, "example")
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}

	want := CandidateResult{
		Candidate: "example",
		Zones: []ZonePresence{
			{Zone: "com", Present: true},
			{Zone: "net", Present: true},
		},
		PresentInAny: true,
		AbsentInAll:  false,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify() = %#v, want %#v", got, want)
	}
}

func TestClassifyAbsentInAll(t *testing.T) {
	got, err := Classify(context.Background(), fakeLookup{
		zones:   []string{"org"},
		present: map[string]bool{},
	}, "missing")
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if got.PresentInAny {
		t.Fatal("PresentInAny = true, want false")
	}
	if !got.AbsentInAll {
		t.Fatal("AbsentInAll = false, want true")
	}
}

func TestClassifyAllPreservesCandidateOrderAndZoneOrder(t *testing.T) {
	got, err := ClassifyAll(context.Background(), fakeLookup{
		zones: []string{"com", "net"},
		present: map[string]bool{
			"com/example": true,
			"net/example": true,
		},
	}, []string{"example", "missing"})
	if err != nil {
		t.Fatalf("ClassifyAll() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ClassifyAll()) = %d, want 2", len(got))
	}
	if got[0].Candidate != "example" || got[1].Candidate != "missing" {
		t.Fatalf("candidate order = %#v, want preserved input order", got)
	}
	wantZones := []ZonePresence{
		{Zone: "com", Present: true},
		{Zone: "net", Present: true},
	}
	if !reflect.DeepEqual(got[0].Zones, wantZones) {
		t.Fatalf("zones = %#v, want %#v", got[0].Zones, wantZones)
	}
}

func TestClassifyPropagatesLookupError(t *testing.T) {
	if _, err := Classify(context.Background(), errLookup{}, "example"); err == nil {
		t.Fatal("Classify() error = nil, want lookup error")
	}
}
