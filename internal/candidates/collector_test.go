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
