# domain-finder

`domain-finder` is a Go CLI project for scanning ICANN CZDS zone files and
building toward high-scale domain availability checks.

## Current status

The repository currently uses stem-based matching across loaded zones:

- a thin CLI entrypoint at `cmd/domain-finder`
- `internal/zonefile` for opening files and detecting gzip by content
- streaming line-by-line zone reading
- `internal/index` for exact-match named-zone indexing and deterministic lookup
- `internal/candidates` for stem loading, normalization, merge, and dedupe
- `internal/config` for YAML config loading with CLI/env/local/base/default precedence
- `internal/match` for stable per-stem classification across loaded zones
- `internal/openai` for batch stem generation through the OpenAI API
- `internal/report` for filtering and summary statistics
- `internal/output` for deterministic durable text and JSONL rendering
- `internal/termui` for lightweight interactive terminal rendering on `stderr`
- a CLI workflow that loads named zones, ingests stems from flags, files,
  and/or stdin, composes `<stem>.<zone>` internally, and reports per-zone
  presence or absence for each stem

Tests intentionally use small deterministic fixtures. They do not depend on
full `.com` or `.net` CZDS zone files.

## Candidate / search model

- Candidate inputs are stems such as `example`, `missing`, or `my-brand`
- Loaded zones determine which FQDNs are tested
- For loaded zones `com` and `net`, candidate `example` checks:
  - `example.com`
  - `example.net`
- Zone indexes still store normalized FQDN owner names from the zone files

## Candidate file and stdin format

- Plain text, one stem per line
- Blank lines are ignored
- Lines beginning with `#` are ignored as comments
- Remaining lines are treated as raw candidate stems

## Candidate merge and dedupe behavior

- Repeated `-candidate` flags are read first
- `-candidate-file` entries are read second
- `-candidate-stdin` entries are read third
- Stems are normalized and deduplicated while preserving first-seen order
- Invalid stems are rejected with a clear error

## YAML config and OpenAI generation

- Optional config files:
  - `domain-finder.yaml`
  - `domain-finder.local.yaml`
- Config precedence:
  1. CLI flags
  2. environment variables
  3. `domain-finder.local.yaml`
  4. `domain-finder.yaml`
  5. built-in defaults
- `OPENAI_API_KEY` is the primary secret source
- `domain-finder.local.yaml` may contain a local fallback `openai.api_key`
- `domain-finder.yaml` must not contain API keys
- `domain-finder.local.yaml` is ignored by git

Committed example config lives at [`domain-finder.yaml.example`](/Users/gene/src/domain-finder/domain-finder.yaml.example).

## Generation workflow

- `-generate "prompt text"` requests OpenAI-generated stems
- `-generate-count` sets the total requested stem count
- `-generate-batch-size` sets the per-request batch size
- `-generate-model` overrides the configured OpenAI model
- `generate.max_attempts` bounds how many attempts each batch gets to satisfy its target
- `generate.retry_count` bounds transient API retries inside one attempt
- Generated values are treated as stems, not FQDNs
- Generated batches are normalized and deduped through `internal/candidates`
- Manual, file, stdin, and generated stems can all be used together
- Matching still composes `<stem>.<zone>` internally

## Hardened generation behavior

- Each requested batch now has a bounded fulfillment policy:
  - request the batch target
  - normalize and dedupe through the existing stem pipeline
  - if too few usable new stems survive, try again for the remainder
  - stop with a clear error when the attempt budget is exhausted
- Transient OpenAI failures such as rate limits or server errors are retried a bounded number of times
- Poor model output such as duplicates, FQDNs, punctuation, empty values, or noisy text is treated as degraded batch quality rather than silently corrupting the candidate pipeline
- Interactive and text-mode generation runs emit concise stderr status lines showing batch requests, accepted/rejected counts, retries, and completion/failure
- JSONL mode stays machine-readable and does not emit live generation progress

## Interactive vs fallback text mode

- Interactive console is enabled only for `text` mode when `stderr` is a TTY
- `-interactive` forces the interactive console on
- `-no-interactive` forces the deterministic fallback report path
- `jsonl` mode never uses the interactive console

## Interactive console behavior

- Prints a small startup header showing loaded zones, candidate count, and filter
- Shows one reusable active stem line while checking
- Prints durable scrolling rows only for emitted stems
- Clears the active line cleanly on completion and prints a compact final status

## stdout / stderr / file behavior

- Interactive text mode:
  - streaming console on `stderr`
  - durable emitted stem results and summary still go to `stdout`, or to `-out`
- Non-interactive text mode:
  - deterministic text report on `stdout`, or in `-out`
  - no interactive terminal rendering
- JSONL mode:
  - deterministic JSON Lines on `stdout`, or in `-out`
  - no interactive terminal rendering

## Manual examples

Stem-based CLI input:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -no-interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

Stem-based candidate-file input:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -interactive \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate-file testdata/small/candidates.txt
```

Stem-based stdin input:

```sh
printf 'missing\nexample\n' | \
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -no-interactive \
  -candidate-stdin \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice
```

Interactive console with stems:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

YAML-configured generation with manual stems:

```sh
cp domain-finder.yaml.example domain-finder.yaml
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate missing \
  -generate "short invented SaaS brand stems" \
  -generate-count 6 \
  -generate-batch-size 3
```

Typical operator feedback during generation:

- `generation: batch 1 attempt 1 requesting 3 stems`
- `generation: batch 1 attempt 1 accepted 2, invalid 1, duplicates 0, need 1 more`
- `generation: retrying batch 1 attempt 2 (1/2) after transient error`
- `generation: complete, accepted 6 stems`

This still reports only exact presence or absence in loaded zone files. It is
not a registrar availability check. OpenAI generation produces candidate stems
only; it does not check registrar or DNS availability.
