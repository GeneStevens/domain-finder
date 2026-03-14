# domain-finder

`domain-finder` is a Go CLI project for scanning ICANN CZDS zone files and
building toward high-scale domain availability checks.

## Current status

The repository currently provides the first terminal-progress text UX layer:

- a thin CLI entrypoint at `cmd/domainfinder`
- `internal/zonefile` for opening files and detecting gzip by content
- streaming line-by-line zone reading
- `internal/index` for exact-match named-zone indexing and deterministic lookup
- `internal/candidates` for candidate loading, normalization, merge, and dedupe
- `internal/match` for stable per-candidate classification results
- `internal/report` for filtering and summary statistics
- `internal/output` for text and JSON Lines rendering
- `internal/termui` for lightweight transient activity rendering on stderr
- a CLI workflow that loads named zones, ingests candidates from flags, files,
  and/or stdin, classifies candidates, applies report policy, and writes
  durable results either to stdout or to a file
- tiny fixture-based tests under `testdata/` for both plain and gzip inputs

Tests intentionally use small deterministic fixtures. They do not depend on
full `.com` or `.net` CZDS zone files.

## Normalization policy

- Indexed values are normalized FQDN owner names exactly as parsed from zone
  records: lowercase, no trailing dot.
- Candidate ingestion applies the same normalization policy to CLI, file, and
  stdin inputs.
- Candidates must be full domain names such as `example.com` or `example.net`.
- Relative labels such as `example` are rejected in this phase.

## Candidate file and stdin format

- Plain text, one candidate per line
- Blank lines are ignored
- Lines beginning with `#` are ignored as comments
- Remaining lines are treated as raw candidate strings

## Candidate merge and dedupe behavior

- Repeated `-candidate` flags are read first
- `-candidate-file` entries are read second
- `-candidate-stdin` entries are read third
- Candidates are normalized and deduplicated while preserving first-seen order
- Invalid candidates are rejected with a clear error

## Live text-mode progress

- Text mode now shows a transient reusable activity line on `stderr`
- The activity line reports candidate index, candidate name, and whether it was
  emitted or skipped by the current filter
- Durable emitted text results are still written to `stdout`, or to `-out` if
  specified
- JSONL mode disables live progress entirely so machine-readable output stays
  clean

## stdout vs stderr behavior

- Text mode without `-out`:
  - transient progress on `stderr`
  - durable emitted results and summary on `stdout`
- Text mode with `-out`:
  - transient progress on `stderr`
  - durable emitted results and summary in the output file
- JSONL mode:
  - durable JSONL records on `stdout` or in the output file
  - no live progress on `stderr`

## CLI examples

Text mode with live progress:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example.net \
  -candidate missing.net
```

Text mode with `-filter absent-in-all`:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

Text mode with `-out`:

```sh
printf 'missing.net\n' | \
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -candidate-stdin \
  -filter absent-in-all \
  -out results.txt \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

JSONL mode with no live progress:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -format jsonl \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate missing.net \
  -candidate example.net
```

## Filter modes

- `all`: emit every classified candidate result
- `absent-in-all`: emit only candidates not found in any loaded zone

## Output behavior

- `text`: durable per-candidate text results followed by a deterministic summary
- `jsonl`: one JSON object per emitted candidate with fields `candidate`,
  `zones`, `present_in_any`, and `absent_in_all`
- Summary output is text-only; JSONL intentionally omits summary records to stay
  clean for downstream tooling

This still reports only exact presence or absence in loaded zone files. It is
not a registrar availability check.
