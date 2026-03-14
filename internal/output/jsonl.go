package output

import (
	"encoding/json"
	"io"

	"github.com/genestevens/domain-finder/internal/match"
)

// WriteJSONL renders one JSON object per candidate result.
func WriteJSONL(w io.Writer, results []match.CandidateResult) error {
	encoder := json.NewEncoder(w)
	for _, result := range results {
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return nil
}
