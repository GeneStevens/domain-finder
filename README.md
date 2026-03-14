# domain-finder

`domain-finder` is a Go CLI project for scanning ICANN CZDS zone files and
building toward high-scale domain availability checks.

## Current status

The repository currently uses stem-based matching across loaded zones:

- a thin CLI entrypoint at `cmd/domain-finder`
- `internal/zonefile` for opening files and detecting gzip by content
- streaming line-by-line zone reading
- `internal/index` for exact-match named-zone indexing and deterministic lookup
- `internal/backend` for backend-neutral file and PostgreSQL exact-match lookups
- `internal/candidates` for stem loading, normalization, merge, and dedupe
- `internal/config` for YAML config loading with CLI/env/local/base/default precedence
- `internal/match` for stable per-stem classification across loaded zones
- `internal/openai` for batch stem generation through the OpenAI API
- `internal/report` for filtering and summary statistics
- `internal/output` for deterministic durable text and JSONL rendering
- `internal/termui` for compact interactive table rendering on `stderr`
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

## Backends

- `-backend file` keeps the existing zone-file lookup behavior
- `-backend postgres` checks exact stem presence against PostgreSQL Domain Miner data
- The result model stays stem-based and backend-neutral

### Zone syntax by backend

- File backend:
  - `-zone com=path/to/com.zone`
  - `-zone net=path/to/net.zone`
- PostgreSQL backend:
  - `-zone com`
  - `-zone net`

### PostgreSQL exact-match semantics

- Query shape uses `SELECT EXISTS (...)`
- Exact match keys:
  - `zone_file = <zone>`
  - `name = <stem>`
- Assumed schema:
  - table `dm.zone_records`
  - columns `zone_file`, `name`

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
- `PG_DSN` can provide the PostgreSQL connection string
- `domain-finder.local.yaml` may contain a local fallback `openai.api_key`
- `domain-finder.yaml` must not contain API keys
- `domain-finder.local.yaml` is ignored by git

Committed example config lives at [`domain-finder.yaml.example`](/Users/gene/src/domain-finder/domain-finder.yaml.example).

## Generation workflow

- `-generate "prompt text"` requests OpenAI-generated stems
- `-generate-count` sets the total requested stem count
- `-generate-batch-size` sets the per-request batch size
- `-generate-model` overrides the configured OpenAI model
- `-generate-style` adds reusable style guidance such as `invented SaaS` or `developer tool`
- `-generate-max-length` prefers stems with no more than `N` letters
- `-generate-max-syllables` prefers shorter, simpler-sounding stems
- `-generate-prefix` prefers stems that start with specific text
- `-generate-suffix` prefers stems that end with specific text
- `-generate-dry-run` prints the fully resolved generation contract and exits before any OpenAI call
- `-generate-dry-run-format text|json` chooses human-readable or machine-readable inspection output
- `-audit-log <path>` writes one audit JSONL record per checked stem
- `generate.max_attempts` bounds how many attempts each batch gets to satisfy its target
- `generate.retry_count` bounds transient API retries inside one attempt
- Generated values are treated as stems, not FQDNs
- Generated batches are normalized and deduped through `internal/candidates`
- Manual, file, stdin, and generated stems can all be used together
- Matching still composes `<stem>.<zone>` internally

## Prompt constraints vs validation

- Prompt constraints steer the OpenAI request; they do not guarantee compliance
- `internal/openai` now owns a dedicated prompt builder for the generation contract
- Generated values still pass through the normal stem validation and dedupe pipeline
- Invalid outputs such as FQDNs, spaces, punctuation, duplicates, or empty strings are still rejected after generation
- `-generate-dry-run` uses the same resolved config and prompt builder, but does not require an API key and does not touch the network

## Generation dry run

- `-generate-dry-run` is an inspection mode for prompt tuning
- It prints the resolved model, generation counts, retry policy, theme, style, structural constraints, and the final prompt-builder output
- `-generate-dry-run-format text` keeps the current readable inspection block
- `-generate-dry-run-format json` emits a stable JSON contract for diffing, archiving, and tooling
- It exits before backend loading, OpenAI client creation, or any network call
- It is intended for prompt-contract inspection, not candidate lookup

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

## Audit log

- `-audit-log <path>` creates or truncates a JSONL file for the run
- The audit log is separate from:
  - the interactive stderr tape
  - deterministic text output
  - JSONL result output
- It records every checked stem, including stems that were:
  - filtered out of the interactive table
  - suppressed by `-interactive-hide-taken`
- Each record includes:
  - `stem`
  - `backend`
  - `requested_zones`
  - per-zone `available` results
  - `state` (`all`, `partial`, `taken`)
  - `report_emitted`
  - `interactive_emitted`
- This is the durable machine-readable truth of what was checked during the run

## Interactive vs fallback text mode

- Interactive console is enabled only for `text` mode when `stderr` is a TTY
- `-interactive` forces the interactive console on
- `-no-interactive` forces the deterministic fallback report path
- `-interactive-hide-taken` suppresses durable `taken` rows in the interactive compact table only
- `-color` forces ANSI styling in interactive mode
- `-no-color` disables ANSI styling in interactive mode
- `jsonl` mode never uses the interactive console

## Interactive console behavior

- Prints a small startup header showing loaded zones, candidate count, and filter
- Shows one reusable ephemeral `checking: ...` line while checking
- Prints exactly one durable line per emitted stem
- Uses a compact table layout tuned for scanability
- Header columns are:
  - `stem`: the stem being evaluated
  - `available_zones`: which requested zones are currently available for that stem
  - `result`: compact status semantics
- Durable rows use explicit status language:
  - `all ✓` means all requested zones are available for that stem
  - `partial` means only some requested zones are available
  - `taken` means none of the requested zones are available
- Strongest all-zone hits stay visually strongest with the success marker and optional ANSI styling
- `-interactive-hide-taken` only suppresses durable `taken` rows on the interactive tape
- It does not change matching, filtering, non-interactive text output, or JSONL output
- Clears the active line cleanly on completion and prints a compact final status

## stdout / stderr / file behavior

- Interactive text mode:
  - compact streaming table on `stderr`
  - no detailed durable result blocks on `stdout`
  - if `-out` is used, the full deterministic report still goes to the file
  - if `-audit-log` is used, every checked stem is still recorded in JSONL even if the row is not shown on the interactive tape
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
  -backend file \
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
  -backend file \
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
  -backend file \
  -no-interactive \
  -candidate-stdin \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice
```

Interactive console with stems:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

Interactive console with taken rows suppressed:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -interactive-hide-taken \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

Interactive mode with audit logging:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -interactive-hide-taken \
  -audit-log run.jsonl \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

Non-interactive mode with audit logging:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -no-interactive \
  -audit-log run.jsonl \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate example \
  -candidate missing
```

Interactive console with strong-hit styling:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -color \
  -filter absent-in-all \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate missing
```

YAML-configured generation with manual stems:

```sh
cp domain-finder.yaml.example domain-finder.yaml
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -candidate missing \
  -generate "short invented SaaS brand stems" \
  -generate-count 6 \
  -generate-batch-size 3
```

Constrained generation with prompt builder guidance:

```sh
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -generate "short product name stems" \
  -generate-style "developer tool" \
  -generate-count 8 \
  -generate-batch-size 4 \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io
```

Dry-run prompt inspection without spending API calls:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -generate "short product name stems" \
  -generate-dry-run \
  -generate-style "developer tool" \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io
```

Machine-readable dry-run contract:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -generate "short product name stems" \
  -generate-dry-run \
  -generate-dry-run-format json \
  -generate-style "developer tool" \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io
```

Typical operator feedback during generation:

- `generation: batch 1 attempt 1 requesting 3 stems`
- `generation: batch 1 attempt 1 accepted 2, invalid 1, duplicates 0, need 1 more`
- `generation: retrying batch 1 attempt 2 (1/2) after transient error`
- `generation: complete, accepted 6 stems`

Example generation tuning in YAML:

```yaml
generate:
  count: 20
  batch_size: 10
  max_attempts: 3
  retry_count: 2
  max_length: 12
  max_syllables: 3
  prefix: dev
  suffix: io
  style: invented SaaS
```

PostgreSQL backend example:

```sh
export PG_DSN='postgres://user:pass@localhost:5432/domainminer'
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend postgres \
  -no-interactive \
  -zone com \
  -zone net \
  -candidate example \
  -candidate missing
```

This still reports only exact presence or absence in loaded zone files. It is
not a registrar availability check. OpenAI generation produces candidate stems
only; it does not check registrar or DNS availability.
