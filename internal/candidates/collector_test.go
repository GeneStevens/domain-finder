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
	if len(report.Accepted) != 2 || report.Accepted[0] != "noviq" || report.Accepted[1] != "trynex" {
		t.Fatalf("Accepted = %#v, want [noviq trynex]", report.Accepted)
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
