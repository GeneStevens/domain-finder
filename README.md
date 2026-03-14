# domain-finder

`domain-finder` is a Go CLI project for scanning ICANN CZDS zone files and
building toward high-scale domain availability checks.

## Current status

The repository currently provides the Phase 3b exact-match reporting foundation:

- a thin CLI entrypoint at `cmd/domainfinder`
- `internal/zonefile` for opening files and detecting gzip by content
- streaming line-by-line zone reading
- `internal/index` for exact-match named-zone indexing and deterministic lookup
- `internal/candidates` for candidate loading, normalization, merge, and dedupe
- `internal/match` for stable per-candidate classification results
- `internal/report` for filtering and summary statistics
- `internal/output` for text and JSON Lines rendering
- a minimal CLI workflow that loads named zones, ingests candidates from flags,
  candidate files, and/or stdin, classifies candidates, applies report policy,
  and writes either to stdout or to a file
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

## CLI examples

Stdin only:

```sh
printf '# comment\nmissing.net\nEXAMPLE.NET.\n' | \
go run ./cmd/domainfinder \
  -candidate-stdin \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice
```

CLI plus stdin:

```sh
printf 'missing.net\nstdin-only.net\n' | \
go run ./cmd/domainfinder \
  -candidate-stdin \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate EXAMPLE.NET. \
  -candidate cli-only.com
```

Candidate file plus stdin:

```sh
printf 'stdin-only.net\nexample.com\n' | \
go run ./cmd/domainfinder \
  -candidate-stdin \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

CLI plus candidate file plus stdin:

```sh
printf 'missing.net\nstdin-only.net\ncli-only.com\n' | \
go run ./cmd/domainfinder \
  -candidate-stdin \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate EXAMPLE.NET. \
  -candidate cli-only.com \
  -candidate-file testdata/small/candidates.txt
```

## Filter modes

- `all`: emit every classified candidate result
- `absent-in-all`: emit only candidates not found in any loaded zone

## Output behavior

- `text`: human-readable candidate summaries plus a deterministic summary block
- `jsonl`: one JSON object per emitted candidate with fields `candidate`,
  `zones`, `present_in_any`, and `absent_in_all`
- Summary output is text-only; JSONL intentionally omits summary records to stay
  clean for downstream tooling

This still reports only exact presence or absence in loaded zone files. It is
not a registrar availability check.
