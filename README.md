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
- `-generate-adaptive-refill` shrinks effective batch size after repeated underfilled batches
- `-generate-min-batch-size` sets the minimum effective batch size for adaptive refill
- `-generate-model` overrides the configured OpenAI model
- `-generate-style` adds reusable style guidance such as `invented SaaS` or `developer tool`
- `-generate-quality-profile industrial|off` applies a generated-only quality filter after validation and lexical bans
- `-generate-max-length` prefers stems with no more than `N` letters
- `-generate-max-syllables` prefers shorter, simpler-sounding stems
- `-generate-prefix` prefers stems that start with specific text
- `-generate-suffix` prefers stems that end with specific text
- `-generate-avoid-substrings` hard-bans low-value lexical families from generated stems
- `-generate-avoid-prefixes` hard-bans generated stems that start with certain prefixes
- `-generate-avoid-suffixes` hard-bans generated stems that end with certain suffixes
- `-generate-max-cost-usd` stops generation once cumulative estimated spend reaches the configured USD cap
- `-generate-target-strong-hits` stops generation once enough all-zone strong hits have been found
- `-generate-max-stall-batches` stops generation after too many consecutive no-progress batches
- `-generate-dry-run` prints the fully resolved generation contract and exits before any OpenAI call
- `-generate-dry-run-format text|json` chooses human-readable or machine-readable inspection output
- `-audit-log <path>` writes one audit JSONL record per checked stem
- `-run-summary <path>` writes one machine-readable JSON summary object for the run
- `generate.max_attempts` bounds how many attempts each batch gets to satisfy its target
- `generate.retry_count` bounds transient API retries inside one attempt
- Generated values are treated as stems, not FQDNs
- Generated batches are normalized and deduped through `internal/candidates`
- Manual, file, stdin, and generated stems can all be used together
- Matching still composes `<stem>.<zone>` internally
- When the OpenAI response includes `usage`, text-mode generation runs show compact token and estimated-cost telemetry

## Prompt constraints vs validation

- Prompt constraints steer the OpenAI request; they do not guarantee compliance
- `internal/openai` now owns a dedicated prompt builder for the generation contract
- Generated values still pass through the normal stem validation and dedupe pipeline
- Invalid outputs such as FQDNs, spaces, punctuation, duplicates, or empty strings are still rejected after generation
- `avoid_substrings` is stronger than prompt guidance alone:
  - it is rendered into the prompt contract as an explicit negative rule
  - generated stems containing banned substrings are also hard-rejected after generation
- `avoid_prefixes` and `avoid_suffixes` extend that same generated-only hard-rejection policy:
  - both are rendered into the prompt contract as explicit negative rules
  - generated stems starting with banned prefixes or ending with banned suffixes are rejected before lookup
- `generate.quality_profile` is a generated-only taste filter:
  - `industrial` now more aggressively favors stronger, harder-edged infrastructure-like name shapes
  - compact 5-7 letter forms, denser consonant structure, stronger consonant anchors, and harder endings score positively
  - soft pharma/startup-mush patterns such as soft open endings, mushy CV alternation, and weak consonant weight are rejected more aggressively
  - manual CLI, file, and stdin stems are not filtered by this profile
- generated runs also use a lightweight family-diversity guard:
  - accepted stems are limited per crude family signature so one naming basin does not dominate the run
  - this is deterministic, explainable, and generated-only
  - family rejections appear in diagnostics as `family_rejected`
- `-generate-dry-run` uses the same resolved config and prompt builder, but does not require an API key and does not touch the network

## Budget- and goal-driven stop conditions

- Generation can now stop on any configured stop condition:
  - accepted-count target
  - estimated cost cap
  - strong-hit target
  - stall limit
- The default fallback still uses `-generate-count` as the accepted-count stop condition
- If additional stop controls are configured, the run ends when any configured condition is reached first
- `-generate-max-cost-usd` uses cumulative estimated spend from OpenAI `usage` plus the repo pricing table
- Cost-cap runs require known pricing for the selected model
  - if pricing is unavailable, the run fails clearly instead of silently ignoring the cap
- `-generate-target-strong-hits` tracks the strongest current result class:
  - stems absent across all requested zones
  - the same `all ✓` semantics shown in the interactive table
- `-generate-max-stall-batches` uses a simple, explicit stall definition:
  - consecutive batches with zero newly accepted generated stems
  - and zero increase in strong all-zone hits
- Batch status lines now show compact progress such as:
  - `strong 3/25`
  - `stall 2/8`
  - `cost $0.18/1.00`
- At the end of a generation run, text mode prints a compact `generation stop` block explaining which condition ended the run
- The run-summary artifact also records:
  - configured stop-condition settings
  - actual stop reason at run end

## Adaptive refill policy

- `-generate-adaptive-refill` is an opt-in sparse-search policy for longer generation runs
- When enabled:
  - generation starts at the configured `batch_size`
  - repeated underfilled batches shrink the effective batch size for later batches
  - the first version uses a simple one-way shrink for the run
  - after 2 consecutive underfilled batches, the effective batch size is cut in half
  - it will not shrink below `-generate-min-batch-size`
- Recovery is intentionally simple in v1:
  - batch size does not grow back during the same run
- Adaptive refill only changes request sizing
  - it does not change acceptance logic
  - it does not change lookup semantics
  - it does not change stop-condition semantics
- When enabled, status lines show the active effective batch size:
  - `effective_batch 8`
  - `batch_size 8->4`
- The run summary records:
  - whether adaptive refill was enabled
  - the configured minimum batch size
  - the final effective batch size

## Generation dry run

- `-generate-dry-run` is an inspection mode for prompt tuning
- It prints the resolved model, generation counts, retry policy, quality profile, theme, style, structural constraints, and the final prompt-builder output
- It also prints the resolved stop-condition policy such as cost cap, strong-hit target, and stall limit
- It also prints adaptive-refill policy such as whether it is enabled and the minimum batch size
- `-generate-dry-run-format text` keeps the current readable inspection block
- `-generate-dry-run-format json` emits a stable JSON contract for diffing, archiving, and tooling
- It exits before backend loading, OpenAI client creation, or any network call
- It is intended for prompt-contract inspection, not candidate lookup

## Hardened generation behavior

- Each requested batch now has a bounded fulfillment policy:
  - request the batch target
  - normalize and dedupe through the existing stem pipeline
  - if too few usable new stems survive, try again for the remainder
  - if refill attempts are exhausted, keep any accepted stems, record the batch as underfilled, and continue to the next batch
- Transient OpenAI failures such as rate limits or server errors are retried a bounded number of times
- Poor model output such as duplicates, FQDNs, punctuation, empty values, or noisy text is treated as degraded batch quality rather than silently corrupting the candidate pipeline
- Generated stems containing banned substrings are rejected before lookup and counted as unusable batch output
- Generated stems hitting banned prefixes or banned suffixes are also rejected before lookup
- Generated stems can also be rejected by the configured quality profile before lookup, and those rejections are counted separately in generation progress
- Interactive and text-mode generation runs emit concise stderr status lines showing batch requests, accepted/rejected counts, retries, and completion/failure
- Underfilled batches are now diagnostic, not fatal:
  - batch status lines can append `underfilled N`
  - the run keeps going until an actual stop condition ends it
- At the end of a generation run, text-mode runs also print a compact `generation diagnostics` block summarizing dominant rejection categories across the whole run
- Those same runs also print a compact `generation underfill` block when any batches finished short
- The same text-mode runs now also print a compact `generation usage` block with:
  - model
  - input/output token totals
  - cached input token totals when available
  - estimated cost from the repo's built-in pricing table
- Batch status lines include compact last-call and cumulative estimated cost when pricing is known
- If the API omits `usage`, the run continues and the usage summary reports `usage: unavailable`
- If the configured model is not in the built-in pricing table, token totals are still shown when available but cost is reported as `pricing unavailable`
- JSONL mode stays machine-readable and does not emit live generation progress

## Generation diagnostics summary

- After a real generation run in text mode, `domain-finder` prints a compact run-level diagnostics block on `stderr`
- This summary aggregates generated-stem rejection signals across the run, including:
  - `banned_substring`
  - `banned_prefix`
  - `banned_suffix`
  - `quality.<reason>`
  - `family_rejected`
  - `invalid`
  - `duplicates`
- Quality reasons reuse the same explainable categories used by the generated quality filter, such as `quality.pharma_like_suffix` or `quality.soft_open_ending`
- The goal is operator tuning:
  - identify which failure families are dominating
  - adjust prompts, lexical bans, or the active quality profile accordingly
- This diagnostics summary is separate from:
  - deterministic text result output
  - JSONL result output
  - the audit log

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

## Run summary artifact

- `-run-summary <path>` creates or truncates one JSON file for the run
- The run summary is separate from:
  - the per-stem audit log
  - the interactive stderr tape
  - deterministic text or JSONL result output
- It captures run-level context and outcomes, including:
  - backend
  - requested zones
  - filter mode
  - whether interactive mode was used
  - final checked/emitted/strong-hit counts
  - generation settings when generation was used
  - configured stop-condition settings and final stop reason when generation was used
  - adaptive-refill settings and final effective batch size when generation was used
  - underfilled-batch totals when generation was used
  - generation token totals and estimated cost when usage data was available
  - aggregated generation diagnostics and rejection categories
- Use it when you want one stable artifact per run for diffing, archiving, or comparing prompt/profile changes over time

## Token and cost telemetry

- Token telemetry is grounded in actual OpenAI API `usage` fields when the response includes them
- The tool tracks:
  - input tokens
  - output tokens
  - cached input tokens from `usage.prompt_tokens_details.cached_tokens` when present
- Cost is an estimate, not a bill:
  - it uses the repo's explicit model pricing table
  - unknown model pricing is not guessed
  - current pricing assumptions should be updated when the official pricing page changes
- This telemetry appears in:
  - compact generation status lines during the run
  - the end-of-run `generation usage` block
  - the JSON run-summary artifact

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

Manual run with a machine-readable run summary:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -no-interactive \
  -run-summary run-summary.json \
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
  -generate-quality-profile industrial \
  -generate-style "developer tool" \
  -generate-count 8 \
  -generate-batch-size 4 \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io
```

Constrained generation with hard lexical bans:

```sh
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -generate "short product name stems" \
  -generate-quality-profile industrial \
  -generate-style "developer tool" \
  -generate-count 8 \
  -generate-batch-size 4 \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-suffix io \
  -generate-avoid-substrings "dev,code,stack,cloud,sync,ops,grid,craft,build,tool,lab,forge,flow" \
  -generate-avoid-prefixes "dev,neo" \
  -generate-avoid-suffixes "io,ia,ora,iva,ara"
```

Budget-shaped exploratory generation:

```sh
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -generate "industrial infrastructure names" \
  -generate-quality-profile industrial \
  -generate-count 200 \
  -generate-max-cost-usd 1.00 \
  -generate-target-strong-hits 25 \
  -generate-max-stall-batches 8
```

Adaptive refill for sparse late-run search:

```sh
export OPENAI_API_KEY=your-key-here
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -backend file \
  -interactive \
  -zone com=testdata/small/com.zone \
  -zone net=testdata/small/net.zone.slice \
  -generate "industrial infrastructure names" \
  -generate-count 200 \
  -generate-adaptive-refill \
  -generate-min-batch-size 2 \
  -generate-max-stall-batches 8
```

Dry-run prompt inspection without spending API calls:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -generate "short product name stems" \
  -generate-dry-run \
  -generate-quality-profile industrial \
  -generate-style "developer tool" \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io \
  -generate-adaptive-refill \
  -generate-min-batch-size 2 \
  -generate-max-cost-usd 1.00 \
  -generate-target-strong-hits 25 \
  -generate-max-stall-batches 8
```

Machine-readable dry-run contract:

```sh
env GOCACHE=/tmp/domain-finder-gocache \
go run ./cmd/domain-finder \
  -generate "short product name stems" \
  -generate-dry-run \
  -generate-dry-run-format json \
  -generate-quality-profile industrial \
  -generate-style "developer tool" \
  -generate-max-length 12 \
  -generate-max-syllables 3 \
  -generate-prefix dev \
  -generate-suffix io \
  -generate-max-cost-usd 1.00 \
  -generate-target-strong-hits 25 \
  -generate-max-stall-batches 8
```

Typical operator feedback during generation:

- `generation: batch 1 attempt 1 requesting 3 stems`
- `generation: batch 1 attempt 1 accepted 2, invalid 1, banned 0, quality_rejected 1, duplicates 0, need 1 more | strong 1/25 | stall 0/8 | cost $0.03/1.00`
- `generation: batch 9 attempt 1 requesting 2 stems | batch_size 8->2`
- `generation diagnostics`
- `  banned_prefix: 3`
- `  banned_suffix: 4`
- `  quality.pharma_like_suffix: 4`
- `  family_rejected: 2`
- `  duplicates: 2`
- `generation: retrying batch 1 attempt 2 (1/2) after transient error`
- `generation: complete, accepted 6 stems | total $0.18 | stop strong-hit target reached`
- `generation stop`
- `  reason: strong-hit target reached`
- `  strong_hits: 25/25`
- `  stall_batches: 0/8`
- `  estimated_cost_usd: $0.18/1.00`

Example generation tuning in YAML:

```yaml
generate:
  count: 20
  batch_size: 10
  adaptive_refill: true
  min_batch_size: 2
  max_attempts: 3
  retry_count: 2
  max_cost_usd: 1.00
  target_strong_hits: 25
  max_stall_batches: 8
  quality_profile: industrial
  max_length: 10
  max_syllables: 3
  prefix: ""
  suffix: ""
  style: industrial infrastructure naming
  avoid_substrings: dev,code,stack,cloud,sync,ops,grid,craft,build,tool,lab,forge,flow
  avoid_prefixes: dev,neo
  avoid_suffixes: io,ia,ora,iva,ara
```

The run summary JSON complements the audit log:

- audit log: one JSONL record per checked stem
- run summary: one JSON object per run

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
