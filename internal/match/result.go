package match

// ZonePresence reports whether a candidate is present in a named zone.
type ZonePresence struct {
	Zone    string `json:"zone"`
	Present bool   `json:"present"`
}

// CandidateResult is the stable per-candidate classification result used by
// output layers. Candidate is stored in normalized FQDN form.
type CandidateResult struct {
	Candidate    string         `json:"candidate"`
	Zones        []ZonePresence `json:"zones"`
	PresentInAny bool           `json:"present_in_any"`
	AbsentInAll  bool           `json:"absent_in_all"`
}
