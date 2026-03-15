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
	"github.com/genestevens/domain-finder/internal/genquality"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/namescore"
	"github.com/genestevens/domain-finder/internal/openai"
	"github.com/genestevens/domain-finder/internal/output"
	"github.com/genestevens/domain-finder/internal/report"
	"github.com/genestevens/domain-finder/internal/runsummary"
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
	runSummaryPath := fs.String("run-summary", "", "write one machine-readable JSON run summary to this file")
	pgDSN := fs.String("pg-dsn", "", "PostgreSQL DSN for the postgres backend")
	candidateFile := fs.String("candidate-file", "", "read candidates from a text file")
	candidateStdin := fs.Bool("candidate-stdin", false, "read candidates from stdin")
	generatePrompt := fs.String("generate", "", "generate candidate stems from this prompt")
	generateDryRun := fs.Bool("generate-dry-run", false, "print the resolved generation contract and exit without calling OpenAI")
	generateDryRunFormat := fs.String("generate-dry-run-format", "text", "dry-run inspection format: text | json")
	generateStyle := fs.String("generate-style", "", "style guidance for generation, such as invented SaaS or developer tool")
	generateCount := fs.Int("generate-count", 0, "total number of stems to generate")
	generateBatchSize := fs.Int("generate-batch-size", 0, "number of stems requested per generation batch")
	generateAdaptiveRefill := fs.Bool("generate-adaptive-refill", false, "shrink effective generation batch size after repeated underfilled batches")
	generateMinBatchSize := fs.Int("generate-min-batch-size", 0, "minimum effective batch size when adaptive refill is enabled")
	generateQualityProfile := fs.String("generate-quality-profile", "", "generated-stem quality profile: industrial | off")
	generatePhoneticQuality := fs.String("generate-phonetic-quality", "", "generated-name scoring profile: normal | strict")
	generateMinLength := fs.Int("generate-min-length", 0, "minimum letters per generated stem")
	generateMinScore := fs.Int("generate-min-score", 0, "minimum generated-name score required before lookup")
	generateMaxLength := fs.Int("generate-max-length", 0, "preferred maximum letters per generated stem")
	generateMaxSyllables := fs.Int("generate-max-syllables", 0, "preferred maximum syllables per generated stem")
	generateSuffix := fs.String("generate-suffix", "", "prefer generated stems ending with this text")
	generatePrefix := fs.String("generate-prefix", "", "prefer generated stems starting with this text")
	generateAvoidSubstrings := fs.String("generate-avoid-substrings", "", "comma-separated substrings that generated stems must not contain")
	generateAvoidPrefixes := fs.String("generate-avoid-prefixes", "", "comma-separated prefixes that generated stems must not start with")
	generateAvoidSuffixes := fs.String("generate-avoid-suffixes", "", "comma-separated suffixes that generated stems must not end with")
	generateMaxCostUSD := fs.Float64("generate-max-cost-usd", 0, "stop generation once estimated spend reaches this USD cap")
	generateTargetAvailableHits := fs.Int("generate-target-available-hits", 0, "stop generation once this many available candidates have been found")
	generateTargetStrongHits := fs.Int("generate-target-strong-hits", 0, "stop generation once this many strong all-zone hits are found")
	generateMaxStallBatches := fs.Int("generate-max-stall-batches", 0, "stop generation after this many consecutive stall batches")
	generateModel := fs.String("generate-model", "", "OpenAI model for stem generation")
	forceInteractive := fs.Bool("interactive", false, "force interactive text console")
	noInteractive := fs.Bool("no-interactive", false, "disable interactive text console")
	hideInteractiveTaken := fs.Bool("interactive-hide-taken", false, "suppress durable 'taken' rows in the interactive compact table")
	showInteractivePartials := fs.Bool("interactive-show-partials", false, "keep partial available-zone hits as durable rows in interactive mode")
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
	if *generateDryRun && strings.TrimSpace(*runSummaryPath) != "" {
		return fmt.Errorf("-run-summary cannot be used with -generate-dry-run")
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

	var runSummaryFile *os.File
	if *runSummaryPath != "" {
		file, err := os.Create(*runSummaryPath)
		if err != nil {
			return fmt.Errorf("create run summary %q: %w", *runSummaryPath, err)
		}
		defer file.Close()
		runSummaryFile = file
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
			OpenAIModel:                 strings.TrimSpace(*generateModel),
			GenerateCount:               *generateCount,
			GenerateBatchSize:           *generateBatchSize,
			GenerateAdaptiveRefill:      *generateAdaptiveRefill,
			GenerateMinBatchSize:        *generateMinBatchSize,
			GenerateQualityProfile:      strings.TrimSpace(*generateQualityProfile),
			GeneratePhoneticQuality:     strings.TrimSpace(*generatePhoneticQuality),
			GenerateMinLength:           *generateMinLength,
			GenerateMinScore:            *generateMinScore,
			GenerateMaxLength:           *generateMaxLength,
			GenerateMaxSyllables:        *generateMaxSyllables,
			GenerateSuffix:              strings.TrimSpace(*generateSuffix),
			GeneratePrefix:              strings.TrimSpace(*generatePrefix),
			GenerateStyle:               strings.TrimSpace(*generateStyle),
			GenerateAvoidSubstrings:     strings.TrimSpace(*generateAvoidSubstrings),
			GenerateAvoidPrefixes:       strings.TrimSpace(*generateAvoidPrefixes),
			GenerateAvoidSuffixes:       strings.TrimSpace(*generateAvoidSuffixes),
			GenerateMaxCostUSD:          *generateMaxCostUSD,
			GenerateTargetAvailableHits: *generateTargetAvailableHits,
			GenerateTargetStrongHits:    *generateTargetStrongHits,
			GenerateMaxStallBatches:     *generateMaxStallBatches,
			PostgresDSN:                 strings.TrimSpace(*pgDSN),
		})
		if err != nil {
			return err
		}
		if trimmedGeneratePrompt != "" || *generateDryRun {
			cfg.Generate.QualityProfile, err = genquality.NormalizeProfile(cfg.Generate.QualityProfile)
			if err != nil {
				return err
			}
			cfg.Generate.PhoneticQuality, err = namescore.NormalizeQuality(cfg.Generate.PhoneticQuality)
			if err != nil {
				return err
			}
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

	interactiveUsed := false
	var outcome runOutcome
	switch *format {
	case "text":
		if termui.ShouldUseInteractive(*format, *forceInteractive, *noInteractive, stderr, stderrIsTTY) {
			interactiveUsed = true
			interactiveWriter := io.Writer(nil)
			if *outPath != "" {
				interactiveWriter = writer
			}
			useColor := termui.ShouldUseColor(*forceColor, *noColor, stderr, stderrIsTTY)
			outcome, err = runInteractiveTextMode(context.Background(), *backendName, lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, interactiveWriter, stderr, useColor, *hideInteractiveTaken, *showInteractivePartials, auditLogger)
			if err != nil {
				return err
			}
			break
		}
		outcome, err = runDeterministicTextMode(context.Background(), *backendName, lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, writer, stderr, auditLogger)
		if err != nil {
			return err
		}
	case "jsonl":
		allResults, filteredResults, diagnostics, usageTotals, underfills, stop, err := processCandidates(context.Background(), lookup, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, nil, nil, func(event progressEvent) error {
			return writeAuditRecord(auditLogger, *backendName, lookup.ZoneNames(), event.Result, event.ReportEmitted, false)
		})
		if err != nil {
			return err
		}
		if err := output.WriteJSONL(writer, filteredResults); err != nil {
			return err
		}
		outcome = runOutcome{
			Summary:             report.Summarize(allResults, filteredResults),
			Diagnostics:         diagnostics,
			GeneratedAccepted:   len(allResults) - len(initialCandidates),
			GenerationRequested: trimmedGeneratePrompt != "",
			UsageTotals:         usageTotals,
			Underfills:          underfills,
			Stop:                stop,
		}
	default:
		return fmt.Errorf("unsupported -format %q: want text or jsonl", *format)
	}
	return writeRunSummary(runSummaryFile, buildRunSummary(*backendName, lookup.ZoneNames(), filterMode, *format, interactiveUsed, trimmedGeneratePrompt, cfg, outcome))
}

type runOutcome struct {
	Summary             report.Summary
	Diagnostics         candidates.GenerationDiagnostics
	GeneratedAccepted   int
	GenerationRequested bool
	UsageTotals         openai.UsageTotals
	Underfills          openai.UnderfillTotals
	Stop                *openai.StopSnapshot
	AvailableHits       int
}

func runDeterministicTextMode(ctx context.Context, backendName string, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, statusWriter io.Writer, auditLogger *audit.Logger) (runOutcome, error) {
	allResults, filteredResults, diagnostics, usageTotals, underfills, stop, err := processCandidates(ctx, lookup, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, nil, makeGenerationNotifier(statusWriter), func(event progressEvent) error {
		return writeAuditRecord(auditLogger, backendName, lookup.ZoneNames(), event.Result, event.ReportEmitted, false)
	})
	if err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationDiagnostics(statusWriter, diagnostics); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationUsage(statusWriter, cfg.OpenAI.Model, usageTotals, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationUnderfills(statusWriter, underfills, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationStop(statusWriter, stop, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}
	summary := report.Summarize(allResults, filteredResults)
	if err := output.WriteText(resultWriter, filteredResults, summary); err != nil {
		return runOutcome{}, err
	}
	return runOutcome{
		Summary:             summary,
		Diagnostics:         diagnostics,
		GeneratedAccepted:   len(allResults) - len(initialCandidates),
		GenerationRequested: generatePrompt != "",
		UsageTotals:         usageTotals,
		Underfills:          underfills,
		Stop:                stop,
		AvailableHits:       countAvailableHits(allResults),
	}, nil
}

func runInteractiveTextMode(ctx context.Context, backendName string, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, progressWriter io.Writer, color, hideTaken, showPartials bool, auditLogger *audit.Logger) (runOutcome, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	totalPlanned := len(initialCandidates)
	if generatePrompt != "" {
		totalPlanned += cfg.Generate.Count
	}
	console := termui.NewConsole(progressWriter, lookup.ZoneNames(), initialCandidates, color, hideTaken, showPartials)
	console.SetInterrupt(cancel)
	if err := console.Start(totalPlanned, filterMode); err != nil {
		return runOutcome{}, err
	}
	defer console.Close()

	allResults, emittedResults, diagnostics, usageTotals, underfills, stop, err := processCandidates(ctx, lookup, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, func(event progressEvent) error {
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
		return console.UpdateStatus(renderGenerationEvent(event))
	}, nil)
	if err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationDiagnosticsToConsole(console, diagnostics); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationUsageToConsole(console, cfg.OpenAI.Model, usageTotals, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationUnderfillsToConsole(console, underfills, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}
	if err := writeGenerationStopToConsole(console, stop, generatePrompt != ""); err != nil {
		return runOutcome{}, err
	}

	summary := report.Summarize(allResults, emittedResults)
	if err := console.Finish(summary); err != nil {
		return runOutcome{}, err
	}
	if resultWriter == nil {
		return runOutcome{
			Summary:             summary,
			Diagnostics:         diagnostics,
			GeneratedAccepted:   len(allResults) - len(initialCandidates),
			GenerationRequested: generatePrompt != "",
			UsageTotals:         usageTotals,
			Underfills:          underfills,
			Stop:                stop,
			AvailableHits:       countAvailableHits(allResults),
		}, nil
	}
	if err := output.WriteTextSummary(resultWriter, summary); err != nil {
		return runOutcome{}, err
	}
	return runOutcome{
		Summary:             summary,
		Diagnostics:         diagnostics,
		GeneratedAccepted:   len(allResults) - len(initialCandidates),
		GenerationRequested: generatePrompt != "",
		UsageTotals:         usageTotals,
		Underfills:          underfills,
		Stop:                stop,
		AvailableHits:       countAvailableHits(allResults),
	}, nil
}

type progressEvent struct {
	Index         int
	Candidate     string
	Result        match.CandidateResult
	Emitted       bool
	ReportEmitted bool
}

func processCandidates(ctx context.Context, lookup backend.Lookup, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, onProgress func(progressEvent) error, onGenerationEvent func(openai.Event) error, onAudit func(progressEvent) error) ([]match.CandidateResult, []match.CandidateResult, candidates.GenerationDiagnostics, openai.UsageTotals, openai.UnderfillTotals, *openai.StopSnapshot, error) {
	allResults := make([]match.CandidateResult, 0, len(initialCandidates))
	emittedResults := make([]match.CandidateResult, 0, len(initialCandidates))
	var diagnostics candidates.GenerationDiagnostics
	var usageTotals openai.UsageTotals
	var underfills openai.UnderfillTotals
	var stop *openai.StopSnapshot
	processedCount := 0
	availableHits := 0
	strongHits := 0

	processBatch := func(candidates []string) (int, int, error) {
		availableDelta := 0
		strongDelta := 0
		for _, candidate := range candidates {
			processedCount++
			result, err := match.Classify(ctx, lookup, candidate)
			if err != nil {
				return 0, 0, err
			}
			allResults = append(allResults, result)
			emitted := report.ShouldEmit(result, filterMode)
			if emitted {
				emittedResults = append(emittedResults, result)
			}
			if result.AbsentInAll {
				strongDelta++
				strongHits++
			}
			if isAvailableHit(result) {
				availableDelta++
				availableHits++
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
					return 0, 0, err
				}
			}
			if onProgress != nil {
				if err := onProgress(event); err != nil {
					return 0, 0, err
				}
			}
		}
		return availableDelta, strongDelta, nil
	}

	if _, _, err := processBatch(initialCandidates); err != nil {
		return nil, nil, diagnostics, usageTotals, underfills, stop, err
	}

	if generatePrompt == "" {
		return allResults, emittedResults, diagnostics, usageTotals, underfills, stop, nil
	}

	stopController, err := openai.NewStopController(cfg.OpenAI.Model, openai.StopConditions{
		MaxAccepted:         cfg.Generate.Count,
		MaxCostUSD:          cfg.Generate.MaxCostUSD,
		TargetAvailableHits: cfg.Generate.TargetAvailableHits,
		TargetStrongHits:    cfg.Generate.TargetStrongHits,
		MaxStallBatches:     cfg.Generate.MaxStallBatches,
	}, availableHits, strongHits)
	if err != nil {
		return nil, nil, diagnostics, usageTotals, underfills, stop, err
	}
	if decision := stopController.InitialDecision(); decision != nil && decision.Reason != openai.StopReasonCountReached {
		stop = decision
		return allResults, emittedResults, diagnostics, usageTotals, underfills, stop, nil
	}

	fulfiller := openai.NewFulfiller(generator, cfg.Generate)
	wrappedGenerationEvent := func(event openai.Event) error {
		if event.Type == openai.EventBatchRequest || event.Type == openai.EventBatchResult || event.Type == openai.EventRetry {
			event.Stop = stopController.Snapshot()
		}
		if (event.Type == openai.EventComplete || event.Type == openai.EventFailed) && stop != nil {
			event.Stop = *stop
		}
		if onGenerationEvent != nil {
			return onGenerationEvent(event)
		}
		return nil
	}
	usageTotals, underfills, stop, err = fulfiller.Fulfill(ctx, generatePrompt, cfg.Generate.Count, func(batch openai.BatchResult, limit int) (candidates.BatchReport, error) {
		report := collector.AddGeneratedReportLimited(batch.Stems, limit, candidates.GeneratedPolicy{
			AvoidSubstrings: cfg.Generate.AvoidSubstrings,
			AvoidPrefixes:   cfg.Generate.AvoidPrefixes,
			AvoidSuffixes:   cfg.Generate.AvoidSuffixes,
			MinLength:       cfg.Generate.MinLength,
			MinScore:        cfg.Generate.MinScore,
			PhoneticQuality: cfg.Generate.PhoneticQuality,
			QualityProfile:  cfg.Generate.QualityProfile,
		})
		diagnostics.MergeBatch(report)
		availableDelta, strongDelta, err := processBatch(report.Accepted)
		if err != nil {
			return candidates.BatchReport{}, err
		}
		if decision := stopController.ObserveBatch(len(report.Accepted), availableDelta, strongDelta, batch.Usage); decision != nil {
			stop = decision
			return report, &openai.StopError{Snapshot: *decision}
		}
		return report, nil
	}, wrappedGenerationEvent)
	if err != nil {
		return nil, nil, diagnostics, usageTotals, underfills, stop, err
	}
	if stop == nil && usageTotals.HasUsage() {
		snapshot := stopController.Snapshot()
		stop = &snapshot
	}

	return allResults, emittedResults, diagnostics, usageTotals, underfills, stop, nil
}

func writeGenerationUnderfills(w io.Writer, underfills openai.UnderfillTotals, generationRequested bool) error {
	if w == nil || !generationRequested || (underfills.Batches == 0 && underfills.Stems == 0) {
		return nil
	}
	for _, line := range renderGenerationUnderfillLines(underfills) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationUnderfillsToConsole(console *termui.Console, underfills openai.UnderfillTotals, generationRequested bool) error {
	if console == nil || !generationRequested || (underfills.Batches == 0 && underfills.Stems == 0) {
		return nil
	}
	for _, line := range renderGenerationUnderfillLines(underfills) {
		if err := console.Note(line); err != nil {
			return err
		}
	}
	return nil
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

func writeGenerationDiagnostics(w io.Writer, diagnostics candidates.GenerationDiagnostics) error {
	if w == nil {
		return nil
	}
	for _, line := range diagnostics.Lines() {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationDiagnosticsToConsole(console *termui.Console, diagnostics candidates.GenerationDiagnostics) error {
	if console == nil {
		return nil
	}
	for _, line := range diagnostics.Lines() {
		if err := console.Note(line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationUsage(w io.Writer, model string, totals openai.UsageTotals, generationRequested bool) error {
	if w == nil || !generationRequested {
		return nil
	}
	for _, line := range renderGenerationUsageLines(model, totals) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationUsageToConsole(console *termui.Console, model string, totals openai.UsageTotals, generationRequested bool) error {
	if console == nil || !generationRequested {
		return nil
	}
	for _, line := range renderGenerationUsageLines(model, totals) {
		if err := console.Note(line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationStop(w io.Writer, stop *openai.StopSnapshot, generationRequested bool) error {
	if w == nil || !generationRequested || stop == nil || stop.Reason == "" {
		return nil
	}
	for _, line := range renderGenerationStopLines(stop) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerationStopToConsole(console *termui.Console, stop *openai.StopSnapshot, generationRequested bool) error {
	if console == nil || !generationRequested || stop == nil || stop.Reason == "" {
		return nil
	}
	for _, line := range renderGenerationStopLines(stop) {
		if err := console.Note(line); err != nil {
			return err
		}
	}
	return nil
}

func renderGenerationUsageLines(model string, totals openai.UsageTotals) []string {
	lines := []string{"generation usage"}
	if model != "" {
		lines = append(lines, fmt.Sprintf("  model: %s", model))
	}
	if !totals.HasUsage() {
		lines = append(lines, "  usage: unavailable")
		return lines
	}
	lines = append(lines,
		fmt.Sprintf("  input_tokens: %d", totals.InputTokens),
		fmt.Sprintf("  output_tokens: %d", totals.OutputTokens),
		fmt.Sprintf("  cached_input_tokens: %d", totals.CachedInputTokens),
	)
	if totals.PricingAvailable {
		lines = append(lines, fmt.Sprintf("  estimated_cost_usd: %s", openai.FormatCostUSD(totals.EstimatedCostUSD)))
	} else {
		lines = append(lines, "  estimated_cost_usd: pricing unavailable")
	}
	return lines
}

func renderGenerationUnderfillLines(underfills openai.UnderfillTotals) []string {
	return []string{
		"generation underfill",
		fmt.Sprintf("  underfilled_batches: %d", underfills.Batches),
		fmt.Sprintf("  underfilled_stems: %d", underfills.Stems),
	}
}

func renderGenerationStopLines(stop *openai.StopSnapshot) []string {
	if stop == nil || stop.Reason == "" {
		return nil
	}
	lines := []string{
		"generation stop",
		fmt.Sprintf("  reason: %s", openai.StopReasonLabel(stop.Reason)),
	}
	if stop.TargetStrongHits > 0 {
		lines = append(lines, fmt.Sprintf("  strong_hits: %d/%d", stop.StrongHits, stop.TargetStrongHits))
	}
	if stop.TargetAvailableHits > 0 {
		lines = append(lines, fmt.Sprintf("  available_hits: %d/%d", stop.AvailableHits, stop.TargetAvailableHits))
	}
	if stop.MaxStallBatches > 0 {
		lines = append(lines, fmt.Sprintf("  stall_batches: %d/%d", stop.StallBatches, stop.MaxStallBatches))
	}
	if stop.MaxCostUSD > 0 {
		if stop.PricingAvailable {
			lines = append(lines, fmt.Sprintf("  estimated_cost_usd: %s/%.2f", openai.FormatCostUSD(stop.EstimatedCostUSD), stop.MaxCostUSD))
		} else {
			lines = append(lines, "  estimated_cost_usd: pricing unavailable")
		}
	}
	return lines
}

func buildRunSummary(backendName string, requestedZones []string, filterMode report.FilterMode, format string, interactive bool, generatePrompt string, cfg config.Config, outcome runOutcome) runsummary.Artifact {
	artifact := runsummary.Artifact{
		Backend:           backendName,
		RequestedZones:    append([]string(nil), requestedZones...),
		FilterMode:        string(filterMode),
		Interactive:       interactive,
		Format:            format,
		TotalCheckedStems: outcome.Summary.TotalCandidates,
		EmittedResults:    outcome.Summary.EmittedResults,
		StrongHits:        outcome.Summary.AbsentInAll,
		PresentInAny:      outcome.Summary.PresentInAny,
		Diagnostics:       runsummary.NewDiagnostics(outcome.Diagnostics),
	}
	if outcome.GenerationRequested {
		artifact.Generation = &runsummary.Generation{
			Model:                   cfg.OpenAI.Model,
			Prompt:                  generatePrompt,
			Style:                   cfg.Generate.Style,
			GenerateCount:           cfg.Generate.Count,
			BatchSize:               cfg.Generate.BatchSize,
			AdaptiveRefill:          cfg.Generate.AdaptiveRefill,
			MinBatchSize:            cfg.Generate.MinBatchSize,
			FinalEffectiveBatchSize: outcome.Underfills.FinalEffectiveBatchSize,
			MaxAttempts:             cfg.Generate.MaxAttemptsPerBatch,
			RetryCount:              cfg.Generate.RetryCount,
			QualityProfile:          cfg.Generate.QualityProfile,
			PhoneticQuality:         cfg.Generate.PhoneticQuality,
			MinLength:               cfg.Generate.MinLength,
			MinScore:                cfg.Generate.MinScore,
			AvoidSubstrings:         append([]string(nil), cfg.Generate.AvoidSubstrings...),
			AvoidPrefixes:           append([]string(nil), cfg.Generate.AvoidPrefixes...),
			AvoidSuffixes:           append([]string(nil), cfg.Generate.AvoidSuffixes...),
			MaxCostUSD:              cfg.Generate.MaxCostUSD,
			TargetAvailableHits:     cfg.Generate.TargetAvailableHits,
			TargetStrongHits:        cfg.Generate.TargetStrongHits,
			MaxStallBatches:         cfg.Generate.MaxStallBatches,
			AcceptedCount:           outcome.GeneratedAccepted,
			AvailableHits:           outcome.AvailableHits,
			UnderfilledBatches:      outcome.Underfills.Batches,
			UnderfilledStems:        outcome.Underfills.Stems,
			StopReason:              string(stopReason(outcome.Stop)),
			InputTokens:             outcome.UsageTotals.InputTokens,
			OutputTokens:            outcome.UsageTotals.OutputTokens,
			CachedInputTokens:       outcome.UsageTotals.CachedInputTokens,
			PricingAvailable:        outcome.UsageTotals.PricingAvailable,
			EstimatedCostUSD:        outcome.UsageTotals.EstimatedCostUSD,
		}
	}
	return artifact
}

func writeRunSummary(file *os.File, artifact runsummary.Artifact) error {
	if file == nil {
		return nil
	}
	return runsummary.Write(file, artifact)
}

func isAvailableHit(result match.CandidateResult) bool {
	for _, zone := range result.Zones {
		if !zone.Present {
			return true
		}
	}
	return false
}

func countAvailableHits(results []match.CandidateResult) int {
	count := 0
	for _, result := range results {
		if isAvailableHit(result) {
			count++
		}
	}
	return count
}

func renderGenerationEvent(event openai.Event) string {
	switch event.Type {
	case openai.EventBatchRequest:
		line := fmt.Sprintf("generation: batch %d attempt %d requesting %d stems", event.Batch, event.Attempt, event.Requested)
		line += renderAdaptiveBatchSize(event)
		line += renderLiveProgress(event)
		return line
	case openai.EventBatchResult:
		if event.Err != nil && openai.IsQuality(event.Err) {
			line := fmt.Sprintf("generation: batch %d attempt %d produced unusable output, need %d more", event.Batch, event.Attempt, event.RemainingBatch)
			line += renderAdaptiveBatchSize(event)
			line += renderLiveProgress(event)
			return line
		}
		line := fmt.Sprintf("generation: batch %d attempt %d accepted %d, invalid %d, banned %d, quality_rejected %d, duplicates %d, need %d more", event.Batch, event.Attempt, event.Accepted, event.Invalid, event.Banned, event.QualityRejected, event.Duplicates, event.RemainingBatch)
		if event.Underfilled > 0 {
			line += fmt.Sprintf(" | underfilled %d", event.Underfilled)
		}
		line += renderAdaptiveBatchSize(event)
		line += renderLiveProgress(event)
		return line
	case openai.EventRetry:
		line := fmt.Sprintf("generation: retrying batch %d attempt %d (%d/%d) after transient error", event.Batch, event.Attempt, event.Retry, event.RetryCount)
		line += renderAdaptiveBatchSize(event)
		line += renderLiveProgress(event)
		return line
	case openai.EventComplete:
		line := fmt.Sprintf("generation: complete, accepted %d stems", event.Accepted)
		line += renderLiveProgress(event)
		if event.Stop.Reason != "" {
			line += fmt.Sprintf(" | stop %s", openai.StopReasonLabel(event.Stop.Reason))
		}
		return line
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

func renderAdaptiveBatchSize(event openai.Event) string {
	if event.EffectiveBatchSize <= 0 {
		return ""
	}
	if event.BaseBatchSize > 0 && event.BaseBatchSize != event.EffectiveBatchSize {
		return fmt.Sprintf(" | batch_size %d->%d", event.BaseBatchSize, event.EffectiveBatchSize)
	}
	if event.BaseBatchSize > 0 {
		return fmt.Sprintf(" | effective_batch %d", event.EffectiveBatchSize)
	}
	return ""
}

func renderLiveProgress(event openai.Event) string {
	parts := make([]string, 0, 4)
	if stopPart := renderStopProgress(event.Stop); stopPart != "" {
		parts = append(parts, stopPart)
	}
	if costPart := renderTotalCostProgress(event); costPart != "" {
		parts = append(parts, costPart)
	}
	if event.Usage != nil {
		if event.LastEstimate.PricingAvailable {
			parts = append(parts, fmt.Sprintf("last %s", openai.FormatCostUSD(event.LastEstimate.CostUSD)))
		} else if event.Stop.MaxCostUSD == 0 && !event.Stop.PricingAvailable {
			parts = append(parts, "pricing unavailable")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func renderStopProgress(stop openai.StopSnapshot) string {
	parts := make([]string, 0, 3)
	if stop.TargetAvailableHits > 0 {
		parts = append(parts, fmt.Sprintf("available %d/%d", stop.AvailableHits, stop.TargetAvailableHits))
	}
	if stop.TargetStrongHits > 0 {
		parts = append(parts, fmt.Sprintf("strong %d/%d", stop.StrongHits, stop.TargetStrongHits))
	}
	if stop.MaxStallBatches > 0 {
		parts = append(parts, fmt.Sprintf("stall %d/%d", stop.StallBatches, stop.MaxStallBatches))
	}
	return strings.Join(parts, " | ")
}

func renderTotalCostProgress(event openai.Event) string {
	label := "cost"
	if event.Type == openai.EventBatchResult || event.Type == openai.EventComplete {
		label = "total"
	}
	switch {
	case event.Stop.MaxCostUSD > 0 && event.Stop.PricingAvailable:
		return fmt.Sprintf("%s %s/%.2f", label, openai.FormatCostUSD(event.Stop.EstimatedCostUSD), event.Stop.MaxCostUSD)
	case event.Stop.MaxCostUSD > 0 && !event.Stop.PricingAvailable:
		return "cost pricing unavailable"
	case event.Stop.PricingAvailable:
		return fmt.Sprintf("%s %s", label, openai.FormatCostUSD(event.Stop.EstimatedCostUSD))
	case event.Totals.PricingAvailable:
		return fmt.Sprintf("%s %s", label, openai.FormatCostUSD(event.Totals.EstimatedCostUSD))
	default:
		return ""
	}
}

func stopReason(stop *openai.StopSnapshot) openai.StopReason {
	if stop == nil {
		return ""
	}
	return stop.Reason
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
