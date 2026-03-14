package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/genestevens/domain-finder/internal/candidates"
	"github.com/genestevens/domain-finder/internal/config"
	"github.com/genestevens/domain-finder/internal/index"
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

// Run executes the CLI entrypoint.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("domain-finder", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var zones zoneFlag
	var cliCandidates candidateFlag
	format := fs.String("format", "text", "output format: text | jsonl")
	filterValue := fs.String("filter", "all", "result filter: all | absent-in-all")
	outPath := fs.String("out", "", "write output to this file instead of stdout")
	candidateFile := fs.String("candidate-file", "", "read candidates from a text file")
	candidateStdin := fs.Bool("candidate-stdin", false, "read candidates from stdin")
	generatePrompt := fs.String("generate", "", "generate candidate stems from this prompt")
	generateCount := fs.Int("generate-count", 0, "total number of stems to generate")
	generateBatchSize := fs.Int("generate-batch-size", 0, "number of stems requested per generation batch")
	generateModel := fs.String("generate-model", "", "OpenAI model for stem generation")
	forceInteractive := fs.Bool("interactive", false, "force interactive text console")
	noInteractive := fs.Bool("no-interactive", false, "disable interactive text console")
	fs.Var(&zones, "zone", "named zone file in the form zone=path (repeatable)")
	fs.Var(&cliCandidates, "candidate", "candidate stem/label to query across loaded zones (repeatable)")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: domain-finder -zone com=path/to/com.zone -candidate example [more flags]\n\n")
		fmt.Fprintf(stderr, "Loads named zone files, checks candidate stems across all loaded zones, and can generate new stems with OpenAI.\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(zones) == 0 {
		fs.Usage()
		return fmt.Errorf("at least one -zone flag is required")
	}
	if len(cliCandidates) == 0 && *candidateFile == "" && !*candidateStdin && strings.TrimSpace(*generatePrompt) == "" {
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

	multi, err := index.LoadMulti(zones)
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

	var cfg config.Config
	var generator openai.StemGenerator
	trimmedGeneratePrompt := strings.TrimSpace(*generatePrompt)
	if trimmedGeneratePrompt != "" {
		workingDir, err := getWorkingDir()
		if err != nil {
			return fmt.Errorf("determine working directory: %w", err)
		}
		cfg, err = loadConfig(workingDir, os.LookupEnv, config.CLIOverrides{
			OpenAIModel:       strings.TrimSpace(*generateModel),
			GenerateCount:     *generateCount,
			GenerateBatchSize: *generateBatchSize,
		})
		if err != nil {
			return err
		}
		generator, err = newStemGenerator(cfg)
		if err != nil {
			return err
		}
	}

	switch *format {
	case "text":
		if termui.ShouldUseInteractive(*format, *forceInteractive, *noInteractive, stderr, stderrIsTTY) {
			return runInteractiveTextMode(context.Background(), multi, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, writer, stderr)
		}
		return runDeterministicTextMode(context.Background(), multi, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, writer, stderr)
	case "jsonl":
		allResults, filteredResults, err := processCandidates(context.Background(), multi, initialCandidates, collector, generator, trimmedGeneratePrompt, cfg, filterMode, nil, nil)
		if err != nil {
			return err
		}
		_ = allResults
		return output.WriteJSONL(writer, filteredResults)
	default:
		return fmt.Errorf("unsupported -format %q: want text or jsonl", *format)
	}
}

func runDeterministicTextMode(ctx context.Context, multi *index.Multi, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, statusWriter io.Writer) error {
	allResults, filteredResults, err := processCandidates(ctx, multi, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, nil, makeGenerationNotifier(statusWriter))
	if err != nil {
		return err
	}
	summary := report.Summarize(allResults, filteredResults)
	return output.WriteText(resultWriter, filteredResults, summary)
}

func runInteractiveTextMode(ctx context.Context, multi *index.Multi, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, resultWriter, progressWriter io.Writer) error {
	totalPlanned := len(initialCandidates)
	if generatePrompt != "" {
		totalPlanned += cfg.Generate.Count
	}
	console := termui.NewConsole(progressWriter, multi.ZoneNames(), initialCandidates)
	if err := console.Start(totalPlanned, filterMode); err != nil {
		return err
	}

	allResults, emittedResults, err := processCandidates(ctx, multi, initialCandidates, collector, generator, generatePrompt, cfg, filterMode, func(event progressEvent) error {
		if err := console.UpdateActive(event.Index, totalPlanned, event.Candidate); err != nil {
			return err
		}
		if !event.Emitted {
			return nil
		}
		if err := console.EmitRow(event.Result); err != nil {
			return err
		}
		return output.WriteTextResult(resultWriter, event.Result)
	}, func(event openai.Event) error {
		return console.Note(renderGenerationEvent(event))
	})
	if err != nil {
		return err
	}

	summary := report.Summarize(allResults, emittedResults)
	if err := console.Finish(summary); err != nil {
		return err
	}
	return output.WriteTextSummary(resultWriter, summary)
}

type progressEvent struct {
	Index     int
	Candidate string
	Result    match.CandidateResult
	Emitted   bool
}

func processCandidates(ctx context.Context, multi *index.Multi, initialCandidates []string, collector *candidates.Collector, generator openai.StemGenerator, generatePrompt string, cfg config.Config, filterMode report.FilterMode, onProgress func(progressEvent) error, onGenerationEvent func(openai.Event) error) ([]match.CandidateResult, []match.CandidateResult, error) {
	allResults := make([]match.CandidateResult, 0, len(initialCandidates))
	emittedResults := make([]match.CandidateResult, 0, len(initialCandidates))
	processedCount := 0

	processBatch := func(candidates []string) error {
		for _, candidate := range candidates {
			processedCount++
			result := match.Classify(multi, candidate)
			allResults = append(allResults, result)
			emitted := report.ShouldEmit(result, filterMode)
			if emitted {
				emittedResults = append(emittedResults, result)
			}
			if onProgress != nil {
				if err := onProgress(progressEvent{
					Index:     processedCount,
					Candidate: candidate,
					Result:    result,
					Emitted:   emitted,
				}); err != nil {
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
		report := collector.AddAllReportLimited(rawBatch, limit)
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
		return fmt.Sprintf("generation: batch %d attempt %d accepted %d, invalid %d, duplicates %d, need %d more", event.Batch, event.Attempt, event.Accepted, event.Invalid, event.Duplicates, event.RemainingBatch)
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
