package index

import (
	"fmt"
	"io"
	"sort"

	"github.com/gene/domain-finder/internal/zonefile"
)

// LoadExact opens a zone file, streams parsed records, and builds an exact
// index for the explicitly provided zone name.
func LoadExact(zoneName, path string) (*Exact, error) {
	if zoneName == "" {
		return nil, fmt.Errorf("load zone: empty zone name")
	}

	stream, err := zonefile.Open(path)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	index := NewExact(zoneName)
	reader := zonefile.NewReader(stream.Reader)

	for {
		record, err := reader.Next()
		if err == io.EOF {
			return index, nil
		}
		if err != nil {
			return nil, fmt.Errorf("load zone %q from %q: %w", zoneName, path, err)
		}
		index.AddRecord(record)
	}
}

// LoadMulti loads multiple explicitly named zones from path mappings.
func LoadMulti(zonePaths map[string]string) (*Multi, error) {
	multi := NewMulti()
	for _, zoneName := range sortedKeys(zonePaths) {
		index, err := LoadExact(zoneName, zonePaths[zoneName])
		if err != nil {
			return nil, err
		}
		if err := multi.Register(index); err != nil {
			return nil, err
		}
	}
	return multi, nil
}

func sortedKeys(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
