package zonefile

import "strings"

// Record is the minimal parsed representation of a zone file line.
type Record struct {
	Domain string
	Raw    string
}

// ParseLine extracts the first whitespace-delimited token from a basic zone
// record line. This is intentionally conservative for the foundation stage:
// comments and blank lines are ignored, and we assume the first token is the
// owner name for valid records.
func ParseLine(line string) (Record, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return Record{}, false
	}
	if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
		return Record{}, false
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return Record{}, false
	}

	domain := NormalizeDomain(fields[0])
	if domain == "" {
		return Record{}, false
	}

	return Record{
		Domain: domain,
		Raw:    line,
	}, true
}

// NormalizeDomain lowercases, trims surrounding whitespace, and removes a
// single trailing dot. This matches the exact-match form we want for future
// indexing work.
func NormalizeDomain(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")
	if value == "." {
		return ""
	}
	return value
}
