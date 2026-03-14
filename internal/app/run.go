package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gene/domain-finder/internal/candidates"
	"github.com/gene/domain-finder/internal/index"
	"github.com/gene/domain-finder/internal/match"
	"github.com/gene/domain-finder/internal/output"
	"github.com/gene/domain-finder/internal/report"
)

// Run executes the CLI entrypoint.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("domainfinder", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var zones zoneFlag
	var cliCandidates candidateFlag
	format := fs.String("format", "text", "output format: text | jsonl")
	filterValue := fs.String("filter", "all", "result filter: all | absent-in-all")
	outPath := fs.String("out", "", "write output to this file instead of stdout")
	candidateFile := fs.String("candidate-file", "", "read candidates from a text file")
	candidateStdin := fs.Bool("candidate-stdin", false, "read candidates from stdin")
	fs.Var(&zones, "zone", "named zone file in the form zone=path (repeatable)")
	fs.Var(&cliCandidates, "candidate", "full candidate domain name to query (repeatable)")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: domainfinder -zone com=path/to/com.zone -candidate example.com [more flags]\n\n")
		fmt.Fprintf(stderr, "Loads named zone files and performs exact-match lookups using full FQDN candidates.\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(zones) == 0 {
		fs.Usage()
		return fmt.Errorf("at least one -zone flag is required")
	}
	if len(cliCandidates) == 0 && *candidateFile == "" && !*candidateStdin {
		fs.Usage()
		return fmt.Errorf("provide at least one -candidate, -candidate-file, or -candidate-stdin")
	}

	filterMode, err := report.ParseFilterMode(*filterValue)
	if err != nil {
		return err
	}

	loadedCandidates, err := candidates.Load(candidates.Sources{
		CLI:      cliCandidates,
		File:     *candidateFile,
		Stdin:    stdin,
		UseStdin: *candidateStdin,
	})
	if err != nil {
		return err
	}

	multi, err := index.LoadMulti(zones)
	if err != nil {
		return err
	}

	allResults := match.ClassifyAll(multi, loadedCandidates)
	filteredResults := report.ApplyFilter(allResults, filterMode)
	summary := report.Summarize(allResults, filteredResults)

	writer := stdout
	if *outPath != "" {
		file, err := os.Create(*outPath)
		if err != nil {
			return fmt.Errorf("create output file %q: %w", *outPath, err)
		}
		defer file.Close()
		writer = file
	}

	switch *format {
	case "text":
		return output.WriteText(writer, filteredResults, summary)
	case "jsonl":
		return output.WriteJSONL(writer, filteredResults)
	default:
		return fmt.Errorf("unsupported -format %q: want text or jsonl", *format)
	}
}

type zoneFlag map[string]string

func (z *zoneFlag) String() string {
	if z == nil {
		return ""
	}
	parts := make([]string, 0, len(*z))
	for name, path := range *z {
		parts = append(parts, name+"="+path)
	}
	return strings.Join(parts, ",")
}

func (z *zoneFlag) Set(value string) error {
	if *z == nil {
		*z = make(map[string]string)
	}
	name, path, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
		return fmt.Errorf("invalid -zone value %q: want zone=path", value)
	}
	(*z)[strings.TrimSpace(name)] = strings.TrimSpace(path)
	return nil
}

type candidateFlag []string

func (c *candidateFlag) String() string {
	return strings.Join(*c, ",")
}

func (c *candidateFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("candidate must not be empty")
	}
	*c = append(*c, strings.TrimSpace(value))
	return nil
}
