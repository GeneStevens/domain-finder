package candidates

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Sources describes all candidate input sources in deterministic merge order:
// CLI values first, then file entries, then stdin entries.
type Sources struct {
	CLI      []string
	File     string
	Stdin    io.Reader
	UseStdin bool
}

// Load collects candidates from all configured sources, normalizes them,
// validates them as single-label stems, and deduplicates while preserving
// first-seen order.
func Load(s Sources) ([]string, error) {
	values := make([]string, 0, len(s.CLI))
	values = append(values, s.CLI...)

	if s.File != "" {
		fileValues, err := loadFile(s.File)
		if err != nil {
			return nil, err
		}
		values = append(values, fileValues...)
	}

	if s.UseStdin {
		if s.Stdin == nil {
			return nil, fmt.Errorf("candidate stdin is enabled but no stdin reader was provided")
		}
		stdinValues, err := loadReader(s.Stdin, "stdin")
		if err != nil {
			return nil, err
		}
		values = append(values, stdinValues...)
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized, err := NormalizeCandidate(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out, nil
}

func loadFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open candidate file %q: %w", path, err)
	}
	defer file.Close()

	return loadReader(file, fmt.Sprintf("candidate file %q", path))
}

func loadReader(r io.Reader, sourceName string) ([]string, error) {
	scanner := bufio.NewScanner(r)
	values := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		values = append(values, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", sourceName, err)
	}
	return values, nil
}
