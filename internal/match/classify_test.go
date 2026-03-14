package match

import (
	"reflect"
	"testing"

	"github.com/gene/domain-finder/internal/index"
)

func TestClassifyAcrossMultipleZones(t *testing.T) {
	multi := index.NewMulti()
	com := index.NewExact("com")
	com.Add("example.com")
	net := index.NewExact("net")
	net.Add("example.net")

	if err := multi.Register(net); err != nil {
		t.Fatalf("Register(net) error = %v", err)
	}
	if err := multi.Register(com); err != nil {
		t.Fatalf("Register(com) error = %v", err)
	}

	got := Classify(multi, "EXAMPLE.NET.")
	want := CandidateResult{
		Candidate: "example.net",
		Zones: []ZonePresence{
			{Zone: "com", Present: false},
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
	multi := index.NewMulti()
	org := index.NewExact("org")
	if err := multi.Register(org); err != nil {
		t.Fatalf("Register(org) error = %v", err)
	}

	got := Classify(multi, "missing.org")
	if got.PresentInAny {
		t.Fatal("PresentInAny = true, want false")
	}
	if !got.AbsentInAll {
		t.Fatal("AbsentInAll = false, want true")
	}
}

func TestClassifyAllPreservesCandidateOrderAndZoneOrder(t *testing.T) {
	multi := index.NewMulti()
	zones := []struct {
		name   string
		domain string
	}{
		{name: "net", domain: "example.net"},
		{name: "com", domain: "example.com"},
	}
	for _, zone := range zones {
		idx := index.NewExact(zone.name)
		idx.Add(zone.domain)
		if err := multi.Register(idx); err != nil {
			t.Fatalf("Register(%s) error = %v", zone.name, err)
		}
	}

	got := ClassifyAll(multi, []string{"example.net", "missing.net"})
	if len(got) != 2 {
		t.Fatalf("len(ClassifyAll()) = %d, want 2", len(got))
	}
	if got[0].Candidate != "example.net" || got[1].Candidate != "missing.net" {
		t.Fatalf("candidate order = %#v, want preserved input order", got)
	}
	wantZones := []ZonePresence{
		{Zone: "com", Present: false},
		{Zone: "net", Present: true},
	}
	if !reflect.DeepEqual(got[0].Zones, wantZones) {
		t.Fatalf("zones = %#v, want %#v", got[0].Zones, wantZones)
	}
}
