# AI Context

## Project goal

Build a high-scale CLI that searches ICANN CZDS zone files for potentially
available domains.

## Current architecture

- `cmd/domain-finder/main.go`: thin CLI entrypoint
- `AGENTS.md`: persistent repo-level Codex workflow and git policy
- `internal/app`: argument parsing and top-level lookup orchestration
- `internal/zonefile`: zone opening, gzip detection, streaming reader, parser
- `internal/index`: single-zone exact-match index, multi-zone lookup, loader
- `internal/backend`: backend-neutral lookup interface plus file and PostgreSQL backends
- `internal/candidates`: stem loading, normalization, merge, and dedupe
- `internal/candidates`: also owns run-level generated rejection diagnostics aggregation
- `internal/config`: YAML config loading and precedence resolution
- `internal/genquality`: generated-stem quality profiles and explainable rule-based scoring
- `internal/match`: stable stem result model and per-zone classification
- `internal/openai`: batched OpenAI stem generation
- `internal/report`: filter modes and summary stats
- `internal/output`: deterministic durable text and JSONL rendering
- `internal/audit`: JSONL audit logging for one record per checked stem
- `internal/termui`: compact interactive stderr table rendering
- `testdata/small`: tiny deterministic fixtures used by tests
- `testdata/slices`: reserved for small realistic slices, never giant CZDS files

## Candidate model

- Candidate inputs are stems, not full FQDNs.
- Loaded zones determine the TLDs checked for each stem.
- Matching composes `<stem>.<zone>` internally.
- Example:
  - candidate `example`
  - zones `com`, `net`
  - lookups `example.com`, `example.net`

## Backends

- File backend:
  - expects `-zone zone=path`
  - composes `<stem>.<zone>` internally for exact file-backed checks
- PostgreSQL backend:
  - expects plain `-zone zone-name`
  - checks `dm.zone_records` with exact `zone_file` + `name`
- `internal/match` now depends on the backend-neutral lookup interface.

## Candidates layer

- Sources:
  - repeated `-candidate` flags
  - optional `-candidate-file`
  - optional `-candidate-stdin`
- File/stdin format:
  - plain text
  - one stem per line
  - blank lines ignored
  - `#` comment lines ignored
- Merge order:
  - CLI stems first
  - file stems second
  - stdin stems third
- Dedupe:
  - preserve first-seen order across all sources
- Error handling:
  - invalid stems return an error
- Current scope:
  - single-label stems only
- Generated stems:
  - flow through `internal/candidates.Collector`
  - reuse the same normalization and dedupe policy as manual stems

## Config and generation

- Optional config files:
  - `domain-finder.yaml`
  - `domain-finder.local.yaml`
- Precedence:
  - CLI flags
  - environment variables
  - local YAML
  - base YAML
  - built-in defaults
- API key policy:
  - prefer `OPENAI_API_KEY`
  - allow `domain-finder.local.yaml` fallback for local-only use
  - do not put API keys in `domain-finder.yaml`
- PostgreSQL config:
  - `postgres.dsn` in YAML
  - `PG_DSN` in environment
  - `-pg-dsn` as the CLI override
- Generation model:
  - `-generate` enables OpenAI stem generation
  - `-generate-dry-run` inspects the resolved generation contract without creating a client or calling the network
  - `-generate-dry-run-format text|json` selects the inspection renderer
  - generation happens in batches
  - each batch is normalized, deduped, and processed before the next batch
  - `internal/openai` owns a dedicated prompt builder for the generation contract
  - prompt constraints can steer length, syllables, prefix, suffix, style, banned substrings, and the generated quality profile
  - generated outputs must be stems only, not FQDNs
  - each batch has bounded fulfillment attempts
  - transient API failures have bounded retries inside one attempt
  - degraded model output is rejected without contaminating the candidate set
  - prompt guidance is not validation; generated stems still pass through the normal candidate validation gate
  - `avoid_substrings` is also hard-enforced after generation, before lookup
  - generated stems can also be rejected by a generated-only quality profile, currently `industrial`
  - text-mode generation runs now print a compact end-of-run diagnostics block summarizing dominant rejection categories
  - dry-run uses the same config-resolution and prompt-builder path as a real run

## Result model

- `candidate`: normalized stem string
- `zones`: ordered `zone/present` results in deterministic zone-name order
- `present_in_any`: true if the stem exists in at least one loaded zone
- `absent_in_all`: true if the stem is absent from all loaded zones

## Report, output, and terminal UX layers

- Report filters:
  - `all`
  - `absent-in-all`
- `internal/output` owns deterministic fallback/file rendering.
- `internal/audit` owns durable machine-readable per-stem run logging.
- `internal/termui` owns the compact streaming interactive table on `stderr`.
- Interactive console prints:
  - a small startup header
  - a reusable `checking:` stem line
  - one-line durable emitted stem rows
  - an `available_zones` column that explicitly lists which requested zones are available
  - a compact `result` column:
    - `all ✓` for strongest all-zone hits
    - `partial` for mixed availability
  - `taken` when no requested zones are available
  - optional ANSI emphasis for strongest all-zone hits
  - optional taken-row suppression for the interactive tape only
  - a compact final completion line
- Interactive mode keeps the compact human-facing table on `stderr`; deterministic detailed output is preserved for non-interactive mode and `-out` files.
- Audit logging is separate from both interactive and deterministic output paths, and records all checked stems whether or not they were visibly shown.
- JSONL bypasses `termui` entirely.

## Current CLI capabilities

- Repeated `-zone name=path` flags load explicitly named zones.
- `-backend file|postgres` selects the lookup backend.
- File backend expects `-zone zone=path`.
- Postgres backend expects `-zone zone-name`.
- Repeated `-candidate stem` flags add explicit stems.
- `-candidate-file <path>` loads stems from a text file.
- `-candidate-stdin` loads stems from stdin.
- `-generate <prompt>` requests OpenAI-generated stems.
- `-generate-dry-run` prints the resolved prompt contract and exits without an API call.
- `-generate-dry-run-format text|json` selects human-readable or machine-readable contract inspection.
- `-generate-count`, `-generate-batch-size`, and `-generate-model` override generation config.
- `-generate-style`, `-generate-quality-profile`, `-generate-max-length`, `-generate-max-syllables`, `-generate-prefix`, `-generate-suffix`, and `-generate-avoid-substrings` steer prompt construction.
- `generate.max_attempts` and `generate.retry_count` harden generation behavior from YAML/env config.
- `-format text|jsonl` selects a human-readable or machine-readable output mode.
- `-filter all|absent-in-all` controls which results are emitted.
- `-out <path>` writes durable output to a file instead of stdout.
- `-interactive` forces interactive text console mode.
- `-no-interactive` forces deterministic fallback text mode.
- `-interactive-hide-taken` suppresses durable `taken` rows only in interactive mode.
- `-audit-log <path>` writes one JSONL audit record per checked stem.
- `-color` / `-no-color` control interactive ANSI styling.

## Testing rule

Do not add tests that depend on full production zone files. Use only small
fixtures or tiny realistic slices under `testdata/`.

## Repo workflow policy

- Persistent Codex operating policy now lives in the repo root `AGENTS.md`.
- Future task packets should follow its git-status, validation, commit-scoping,
  commit-message, and final-reporting rules by default.

## Deferred work

- No concurrency yet.
- No full-screen TUI framework.
- No advanced terminal UI beyond the streaming stderr console.
- No concurrency in generation or matching.
- No registrar checks or probabilistic availability logic.
- No filename-based zone inference.
- No large-file optimization beyond streaming reads.
