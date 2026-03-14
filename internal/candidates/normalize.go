package candidates

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeCandidate normalizes and validates a candidate as a single-label
// domain stem. Loaded zones determine which FQDNs are checked.
func NormalizeCandidate(value string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "", fmt.Errorf("candidate %q is empty after normalization", value)
	}
	if strings.Contains(normalized, ".") {
		return "", fmt.Errorf("candidate %q must be a single-label stem, not a full domain name", value)
	}
	if strings.HasPrefix(normalized, "-") || strings.HasSuffix(normalized, "-") {
		return "", fmt.Errorf("candidate %q is not a valid stem", value)
	}
	for _, r := range normalized {
		if unicode.IsLower(r) || unicode.IsDigit(r) || r == '-' {
			continue
		}
		return "", fmt.Errorf("candidate %q is not a valid stem", value)
	}
	return normalized, nil
}
