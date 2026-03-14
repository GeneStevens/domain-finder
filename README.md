# domain-finder

`domain-finder` is a Go CLI project for scanning ICANN CZDS zone files and
building toward high-scale domain availability checks.

## Current status

The repository currently provides a first streaming interactive text console:

- a thin CLI entrypoint at `cmd/domainfinder`
- `internal/zonefile` for opening files and detecting gzip by content
- streaming line-by-line zone reading
- `internal/index` for exact-match named-zone indexing and deterministic lookup
- `internal/candidates` for candidate loading, normalization, merge, and dedupe
- `internal/match` for stable per-candidate classification results
- `internal/report` for filtering and summary statistics
- `internal/output` for deterministic text and JSON Lines rendering
- `internal/termui` for lightweight interactive terminal rendering on `stderr`
- a CLI workflow that loads named zones, ingests candidates from flags, files,
  and/or stdin, classifies candidates, applies report policy, and writes
  durable results either to `stdout` or to a file

Tests intentionally use small deterministic fixtures. They do not depend on
full `.com` or `.net` CZDS zone files.

## Interactive vs fallback text mode

- Interactive console is enabled only for `text` mode when `stderr` is a TTY
- `-interactive` forces the interactive console on
- `-no-interactive` forces the deterministic fallback report path
- `jsonl` mode never uses the interactive console

## Interactive console behavior

- Prints a small startup header showing loaded zones, candidate count, and filter
- Shows one reusable active candidate line while checking
- Prints durable scrolling rows only for emitted candidates
- Clears the active line cleanly on completion and prints a compact final status

## stdout / stderr / file behavior

- Interactive text mode:
  - streaming console on `stderr`
  - durable emitted results and summary still go to `stdout`, or to `-out`
- Non-interactive text mode:
  - deterministic text report on `stdout`, or in `-out`
  - no interactive terminal rendering
- JSONL mode:
  - deterministic JSON Lines on `stdout`, or in `-out`
  - no interactive terminal rendering

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

## Manual interactive examples

Interactive text mode:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example.net \
  -candidate missing.net
```

Filtered interactive text mode:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -interactive \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

Non-interactive fallback:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -no-interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example.net \
  -candidate missing.net
```

Interactive mode with `-out`:

```sh
printf 'missing.net\n' | \
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domainfinder \
  -interactive \
  -candidate-stdin \
  -filter absent-in-all \
  -out results.txt \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

JSONL unchanged:

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

This still reports only exact presence or absence in loaded zone files. It is
not a registrar availability check.
