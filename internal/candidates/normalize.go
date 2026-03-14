package candidates

import (
	"fmt"
	"strings"

	"github.com/gene/domain-finder/internal/zonefile"
)

// NormalizeCandidate normalizes and validates a candidate as a full FQDN.
// Relative labels such as "example" are rejected in this phase.
func NormalizeCandidate(value string) (string, error) {
	normalized := zonefile.NormalizeDomain(value)
	if normalized == "" {
		return "", fmt.Errorf("candidate %q is empty after normalization", value)
	}
	if !strings.Contains(normalized, ".") {
		return "", fmt.Errorf("candidate %q must be a full domain name", value)
	}
	if strings.HasPrefix(normalized, ".") || strings.HasSuffix(normalized, ".") || strings.Contains(normalized, "..") {
		return "", fmt.Errorf("candidate %q is not a valid full domain name", value)
	}
	return normalized, nil
}
