package candidates

import "testing"

func TestAddGeneratedReportLimitedRejectsAvoidSubstrings(t *testing.T) {
	collector := NewCollector()
	report := collector.AddGeneratedReportLimited([]string{
		"devspark",
		"noviq",
		"cloudbase",
		"trynex",
	}, 4, GeneratedPolicy{AvoidSubstrings: []string{"dev", "cloud"}})

	if report.LexicalRejected != 2 {
		t.Fatalf("LexicalRejected = %d, want 2", report.LexicalRejected)
	}
	if report.BannedSubstrings != 2 {
		t.Fatalf("BannedSubstrings = %d, want 2", report.BannedSubstrings)
	}
	if len(report.Accepted) != 2 || report.Accepted[0] != "noviq" || report.Accepted[1] != "trynex" {
		t.Fatalf("Accepted = %#v, want [noviq trynex]", report.Accepted)
	}
}

func TestAddGeneratedReportLimitedRejectsAvoidPrefixesAndSuffixes(t *testing.T) {
	collector := NewCollector()
	report := collector.AddGeneratedReportLimited([]string{
		"devforge",
		"kinoria",
		"qentil",
	}, 3, GeneratedPolicy{
		AvoidPrefixes: []string{"dev"},
		AvoidSuffixes: []string{"ia", "ora"},
	})

	if report.BannedPrefixes != 1 {
		t.Fatalf("BannedPrefixes = %d, want 1", report.BannedPrefixes)
	}
	if report.BannedSuffixes != 1 {
		t.Fatalf("BannedSuffixes = %d, want 1", report.BannedSuffixes)
	}
	if report.LexicalRejected != 2 {
		t.Fatalf("LexicalRejected = %d, want 2", report.LexicalRejected)
	}
	if len(report.Accepted) != 1 || report.Accepted[0] != "qentil" {
		t.Fatalf("Accepted = %#v, want [qentil]", report.Accepted)
	}
}

func TestAddGeneratedReportLimitedRejectsWeakQuality(t *testing.T) {
	collector := NewCollector()
	report := collector.AddGeneratedReportLimited([]string{
		"theravia",
		"veloria",
		"traktor",
	}, 3, GeneratedPolicy{QualityProfile: "industrial"})

	if report.QualityRejected != 2 {
		t.Fatalf("QualityRejected = %d, want 2", report.QualityRejected)
	}
	if len(report.Accepted) != 1 || report.Accepted[0] != "traktor" {
		t.Fatalf("Accepted = %#v, want [traktor]", report.Accepted)
	}
	if report.QualityReasons["pharma_like_suffix"] == 0 {
		t.Fatalf("QualityReasons = %#v, want pharma_like_suffix accounting", report.QualityReasons)
	}
}

func TestAddGeneratedReportLimitedRejectsFamilyRepetition(t *testing.T) {
	collector := NewCollector()
	report := collector.AddGeneratedReportLimited([]string{
		"kinrox",
		"kinrax",
		"kinrex",
		"qentil",
	}, 4, GeneratedPolicy{QualityProfile: "industrial"})

	if report.FamilyRejected != 1 {
		t.Fatalf("FamilyRejected = %d, want 1", report.FamilyRejected)
	}
	if len(report.Accepted) != 3 {
		t.Fatalf("Accepted = %#v, want 3 accepted stems", report.Accepted)
	}
}
