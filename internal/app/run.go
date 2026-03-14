package app

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/genestevens/domain-finder/internal/audit"
	"github.com/genestevens/domain-finder/internal/backend"
	"github.com/genestevens/domain-finder/internal/candidates"
	"github.com/genestevens/domain-finder/internal/config"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/openai"
	"github.com/genestevens/domain-finder/internal/output"
	"github.com/genestevens/domain-finder/internal/report"
	"github.com/genestevens/domain-finder/internal/termui"
)

var stderrIsTTY = termui.IsTTY
var loadConfig = config.Load
var newStemGenerator = func(cfg config.Config) (openai.StemGenerator, error) {
	return openai.NewClient(cfg)
}
var getWorkingDir = os.Getwd
var openPostgresBackend = func(dsn string, zones []string) (backend.Lookup, io.Closer, error) {
	lookup, err := backend.OpenPostgres(dsn, zones, sql.Open)
	if err != nil {
		return nil, nil, err
	}
	return lookup, lookup, nil
}

// Run executes the CLI entrypoint.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("domain-finder", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var zones zoneFlag
	var cliCandidates candidateFlag
	backendName := fs.String("backend", "file", "lookup backend: file | postgres")
	format := fs.String("format", "text", "output format: text | jsonl")
	filterValue := fs.String("filter", "all", "result filter: all | absent-in-all")
	outPath := fs.String("out", "", "write output to this file instead of stdout")
	auditLogPath := fs.String("audit-log", "", "write one audit JSONL record per checked stem to this file")
	pgDSN := fs.String("pg-dsn", "", "PostgreSQL DSN for the postgres backend")
	candidateFile := fs.String("candidate-file", "", "read candidates from a text file")
	candidateStdin := fs.Bool("candidate-stdin", false, "read candidates from stdin")
	generatePrompt := fs.String("generate", "", "generate candidate stems from this prompt")
	generateDryRun := fs.Bool("generate-dry-run", false, "print the resolved generation contract and exit without calling OpenAI")
	generateDryRunFormat := fs.String("generate-dry-run-format", "text", "dry-run inspection format: text | json")
	generateStyle := fs.String("generate-style", "", "style guidance for generation, such as invented SaaS or developer tool")
	generateCount := fs.Int("generate-count", 0, "total number of stems to generate")
	generateBatchSize := fs.Int("generate-batch-size", 0, "number of stems requested per generation batch")
	generateMaxLength := fs.Int("generate-max-length", 0, "preferred maximum letters per generated stem")
	generateMaxSyllables := fs.Int("generate-max-syllables", 0, "preferred maximum syllables per generated stem")
	generateSuffix := fs.String("generate-suffix", "", "prefer generated stems ending with this text")
	generatePrefix := fs.String("generate-prefix", "", "prefer generated stems starting with this text")
	generateAvoidSubstrings := fs.String("generate-avoid-substrings", "", "comma-separated substrings that generated stems must not contain")
	generateModel := fs.String("generate-model", "", "OpenAI model for stem generation")
	forceInteractive := fs.Bool("interactive", false, "force interactive text console")
	noInteractive := fs.Bool("no-interactive", false, "disable interactive text console")
	hideInteractiveTaken := fs.Bool("interactive-hide-taken", false, "suppress durable 'taken' rows in the interactive compact table")
	forceColor := fs.Bool("color", false, "force ANSI color/styling in interactive mode")
	noColor := fs.Bool("no-color", false, "disable ANSI color/styling in interactive mode")
	fs.Var(&zones, "zone", "zone input: file backend uses zone=path, postgres backend uses zone name (repeatable)")
	fs.Var(&cliCandidates, "candidate", "candidate stem/label to query across loaded zones (repeatable)")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: domain-finder -zone com=path/to/com.zone -candidate example [more flags]\n\n")
		fmt.Fprintf(stderr, "Loads named zone files, checks candidate stems across all loaded zones, and can generate new stems with OpenAI.\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	trimmedGeneratePrompt := strings.TrimSpace(*generatePrompt)
	if *generateDryRun && trimmedGeneratePrompt == "" {
		fs.Usage()
		return fmt.Errorf("-generate-dry-run requires -generate")
	}
	if *generateDryRun && *generateDryRunFormat != "text" && *generateDryRunFormat != "json" {
		return fmt.Errorf("unsupported -generate-dry-run-format %q: want text or json", *generateDryRunFormat)
	}
	if *generateDryRun && strings.TrimSpace(*auditLogPath) != "" {
		return fmt.Errorf("-audit-log cannot be used with -generate-dry-run")
	}
	if len(zones) == 0 && !*generateDryRun {
		fs.Usage()
		return fmt.Errorf("at least one -zone flag is required")
	}
	if len(cliCandidates) == 0 && *candidateFile == "" && !*candidateStdin && trimmedGeneratePrompt == "" {
		fs.Usage()
		return fmt.Errorf("provide at least one -candidate, -candidate-file, -candidate-stdin, or -generate")
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

	collector := candidates.NewCollector()
	initialCandidates, err := collector.AddAll(loadedCandidates)
	if err != nil {
		return err
	}

	writer := stdout
	if *outPath != "" {
		file, err := os.Create(*outPath)
		if err != nil {
			return fmt.Errorf("create output file %q: %w", *outPath, err)
		}
		defer file.Close()
		writer = file
	}

	var auditLogger *audit.Logger
	if *auditLogPath != "" {
		file, err := os.Create(*auditLogPath)
		if err != nil {
			return fmt.Errorf("create audit log %q: %w", *auditLogPath, err)
		}
		defer file.Close()
		auditLogger = audit.NewLogger(file)
	}

	var cfg config.Config
	var generator openai.StemGenerator
	var lookup backend.Lookup
	var lookupCloser io.Closer
	if trimmedGeneratePrompt != "" || *backendName == "postgres" || *generateDryRun {
		workingDir, err := getWorkingDir()
		if err != nil {
			return fmt.Errorf("determine working directory: %w", err)
		}
		cfg, err = loadConfig(workingDir, os.LookupEnv, config.CLIOverrides{
			OpenAIModel:             strings.TrimSpace(*generateModel),
			GenerateCount:           *generateCount,
			GenerateBatchSize:       *generateBatchSize,
			GenerateMaxLength:       *generateMaxLength,
			GenerateMaxSyllables:    *generateMaxSyllables,
			GenerateSuffix:          strings.TrimSpace(*generateSuffix),
			GeneratePrefix:          strings.TrimSpace(*generatePrefix),
			GenerateStyle:           strings.TrimSpace(*generateStyle),
			GenerateAvoidSubstrings: strings.TrimSpace(*generateAvoidSubstrings),
			PostgresDSN:             strings.TrimSpace(*pgDSN),
		})
		if err != nil {
			return err
		}
	}
	if *generateDryRun {
		contract := openai.PromptBuilder{}.BuildContract(cfg, trimmedGeneratePrompt)
		switch *generateDryRunFormat {
		case "text":
			_, err := fmt.Fprint(writer, openai.RenderContract(contract))
			return err
		case "json":
			raw, err := openai.RenderContractJSON(contract)
			if err != nil {
				return err
			}
			_, err = writer.Write(append(raw, '\n'))
			return err
		default:
			return fmt.Errorf("unsupported -generate-dry-run-format %q: want text or json", *generateDryRunFormat)
		}
	}
	if trimmedGeneratePrompt != "" {
		generator, err = newStemGenerator(cfg)
		if err != nil {
			return err
		}
	}
	lookup, lookupCloser, err = loadLookupBackend(*backendName, zones, cfg)
	if err != nil {
		return err
	}
	if lookupCloser != nil {
		defer lookupCloser.Close()
	}

	switch *format {
	case "text":
		if termui.ShouldUseInteractive(*format, *forceInteractive, *noInteractive, stderr, stderrIsTTY) {
			interactiveWriter := io.Writer(nil)
			if *outPath != "" {
				interactiveWriter = writer
			}
			useColor := termui.ShouldUseColor(*forceColor, *noColor, stderr, stderrIsTTY)
			return runInteractiveTextMode(context.Background(), *backendName, lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, interactiveWriter, stderr, useColor, *hideInteractiveTaken, auditLogger)
		}
		return runDeterministicTextMode(context.Background(), *backendName, lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, writer, stderr, auditLogger)
	case "jsonl":
		allResults, filteredResults, err := processCandidates(context.Background(), lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, nil, nil, func(event progressEvent) error {
			return writeAuditRecord(auditLogger, *backendName, lookup.ZoneNames(), event.Result, event.ReportEmitted, false)
		})
		if err != nil {
			return err
		}
		_ = allResults
		return output.WriteJSONL(writer, filteredResults)
	default:
		return fmt.Errorf("unsupported -format %q: want text or jsonl", *format)
	}
}

func runDeterministicTextMode(ctx context.Context, backendName string, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, statusWriter io.Writer, auditLogger *audit.Logger) error {
	allResults, filteredResults, err := processCandidates(ctx, lookup, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, nil, makeGenerationNotifier(statusWriter), func(event progressEvent) error {
		return writeAuditRecord(auditLogger, backendName, lookup.ZoneNames(), event.Result, event.ReportEmitted, false)
	})
	if err != nil {
		return err
	}
	summary := report.Summarize(allResults, filteredResults)
	return output.WriteText(resultWriter, filteredResults, summary)
}

func runInteractiveTextMode(ctx context.Context, backendName string, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, progressWriter io.Writer, color, hideTaken bool, auditLogger *audit.Logger) error {
	totalPlanned := len(initialCandidates)
	if generatePrompt != "" {
		totalPlanned += cfg.Generate.Count
	}
	console := termui.NewConsole(progressWriter, lookup.ZoneNames(), initialCandidates, color, hideTaken)
	if err := console.Start(totalPlanned, filterMode); err != nil {
		return err
	}

	allResults, emittedResults, err := processCandidates(ctx, lookup, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, func(event progressEvent) error {
		if err := console.UpdateActive(event.Index, totalPlanned, event.Candidate); err != nil {
			return err
		}
		interactiveEmitted := event.ReportEmitted && console.ShouldEmitRow(event.Result)
		if err := writeAuditRecord(auditLogger, backendName, lookup.ZoneNames(), event.Result, event.ReportEmitted, interactiveEmitted); err != nil {
			return err
		}
		if interactiveEmitted {
			if err := console.EmitRow(event.Result); err != nil {
				return err
			}
		}
		if !event.ReportEmitted || resultWriter == nil {
			return nil
		}
		return output.WriteTextResult(resultWriter, event.Result)
	}, func(event openai.Event) error {
		return console.Note(renderGenerationEvent(event))
	}, nil)
	if err != nil {
		return err
	}

	summary := report.Summarize(allResults, emittedResults)
	if err := console.Finish(summary); err != nil {
		return err
	}
	if resultWriter == nil {
		return nil
	}
	return output.WriteTextSummary(resultWriter, summary)
}

type progressEvent struct {
	Index         int
	Candidate     string
	Result        match.CandidateResult
	Emitted       bool
	ReportEmitted bool
}

func processCandidates(ctx context.Context, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, onProgress func(progressEvent) error, onGenerationEvent func(openai.Event) error, onAudit func(progressEvent) error) ([]match.CandidateResult, []match.CandidateResult, error) {
	allResults := make([]match.CandidateResult, 0, len(initialCandidates))
	emittedResults := make([]match.CandidateResult, 0, len(initialCandidates))
	processedCount := 0

	processBatch := func(candidates []string) error {
		for _, candidate := range candidates {
			processedCount++
			result, err := match.Classify(ctx, lookup, candidate)
			if err != nil {
				return err
			}
			allResults = append(allResults, result)
			emitted := report.ShouldEmit(result, filterMode)
			if emitted {
				emittedResults = append(emittedResults, result)
			}
			event := progressEvent{
				Index:         processedCount,
				Candidate:     candidate,
				Result:        result,
				Emitted:       emitted,
				ReportEmitted: emitted,
			}
			if onAudit != nil {
				if err := onAudit(event); err != nil {
					return err
				}
			}
			if onProgress != nil {
				if err := onProgress(event); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := processBatch(initialCandidates); err != nil {
		return nil, nil, err
	}

	if generatePrompt == "" {
		return allResults, emittedResults, nil
	}

	fulfiller := openai.NewFulfiller(generator, cfg.Generate)
	err := fulfiller.Fulfill(ctx, generatePrompt, cfg.Generate.Count, func(rawBatch []string, limit int) (candidates.BatchReport, error) {
		report := collector.AddGeneratedReportLimited(rawBatch, limit, candidates.GeneratedPolicy{
			AvoidSubstrings: cfg.Generate.AvoidSubstrings,
		})
		if err := processBatch(report.Accepted); err != nil {
			return candidates.BatchReport{}, err
		}
		return report, nil
	}, onGenerationEvent)
	if err != nil {
		return nil, nil, err
	}

	return allResults, emittedResults, nil
}

func writeAuditRecord(logger *audit.Logger, backendName string, requestedZones []string, result match.CandidateResult, reportEmitted, interactiveEmitted bool) error {
	if logger == nil {
		return nil
	}
	return logger.Write(audit.NewRecord(result, backendName, requestedZones, reportEmitted, interactiveEmitted))
}

func makeGenerationNotifier(w io.Writer) func(openai.Event) error {
	if w == nil {
		return nil
	}
	return func(event openai.Event) error {
		line := renderGenerationEvent(event)
		if line == "" {
			return nil
		}
		_, err := fmt.Fprintln(w, line)
		return err
	}
}

func renderGenerationEvent(event openai.Event) string {
	switch event.Type {
	case openai.EventBatchRequest:
		return fmt.Sprintf("generation: batch %d attempt %d requesting %d stems", event.Batch, event.Attempt, event.Requested)
	case openai.EventBatchResult:
		if event.Err != nil && openai.IsQuality(event.Err) {
			return fmt.Sprintf("generation: batch %d attempt %d produced unusable output, need %d more", event.Batch, event.Attempt, event.RemainingBatch)
		}
		return fmt.Sprintf("generation: batch %d attempt %d accepted %d, invalid %d, banned %d, duplicates %d, need %d more", event.Batch, event.Attempt, event.Accepted, event.Invalid, event.Rejected, event.Duplicates, event.RemainingBatch)
	case openai.EventRetry:
		return fmt.Sprintf("generation: retrying batch %d attempt %d (%d/%d) after transient error", event.Batch, event.Attempt, event.Retry, event.RetryCount)
	case openai.EventComplete:
		return fmt.Sprintf("generation: complete, accepted %d stems", event.Accepted)
	case openai.EventFailed:
		if event.Err == nil {
			return "generation: failed"
		}
		var fulfillmentErr *openai.FulfillmentError
		if errors.As(event.Err, &fulfillmentErr) {
			return fmt.Sprintf("generation: failed after accepting %d of %d requested stems", fulfillmentErr.Accepted, fulfillmentErr.Requested)
		}
		return fmt.Sprintf("generation: failed: %v", event.Err)
	default:
		return ""
	}
}

type zoneFlag []string

func (z *zoneFlag) String() string { return strings.Join(*z, ",") }

func (z *zoneFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("zone must not be empty")
	}
	*z = append(*z, strings.TrimSpace(value))
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

func loadLookupBackend(name string, zones []string, cfg config.Config) (backend.Lookup, io.Closer, error) {
	switch name {
	case "file":
		parsedZones, err := parseFileZones(zones)
		if err != nil {
			return nil, nil, err
		}
		lookup, err := backend.LoadFile(parsedZones)
		return lookup, nil, err
	case "postgres":
		parsedZones, err := parsePostgresZones(zones)
		if err != nil {
			return nil, nil, err
		}
		return openPostgresBackend(cfg.Postgres.DSN, parsedZones)
	default:
		return nil, nil, fmt.Errorf("unsupported -backend %q: want file or postgres", name)
	}
}

func parseFileZones(values []string) (map[string]string, error) {
	out := make(map[string]string, len(values))
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("file backend requires -zone zone=path, got %q", value)
		}
		out[strings.TrimSpace(name)] = strings.TrimSpace(path)
	}
	return out, nil
}

func parsePostgresZones(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.Contains(value, "=") {
			return nil, fmt.Errorf("postgres backend requires -zone zone-name, got %q", value)
		}
		zone := strings.TrimSpace(value)
		if zone == "" {
			return nil, fmt.Errorf("postgres backend requires non-empty zone names")
		}
		if _, ok := seen[zone]; ok {
			continue
		}
		seen[zone] = struct{}{}
		out = append(out, zone)
	}
	sort.Strings(out)
	return out, nil
}
