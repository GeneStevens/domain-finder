# AI Context

## Project goal

Build a high-scale CLI that searches ICANN CZDS zone files for potentially
available domains.

## Current architecture

- `cmd/domainfinder/main.go`: thin CLI entrypoint
- `AGENTS.md`: persistent repo-level Codex workflow and git policy
- `internal/app`: argument parsing and top-level lookup orchestration
- `internal/zonefile`: zone opening, gzip detection, streaming reader, parser
- `internal/index`: single-zone exact-match index, multi-zone lookup, loader
- `internal/candidates`: stem loading, normalization, merge, and dedupe
- `internal/config`: YAML config loading and precedence resolution
- `internal/match`: stable stem result model and per-zone classification
- `internal/openai`: batched OpenAI stem generation
- `internal/report`: filter modes and summary stats
- `internal/output`: deterministic durable text and JSONL rendering
- `internal/termui`: interactive stderr console rendering
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
  - `domainfinder.yaml`
  - `domainfinder.local.yaml`
- Precedence:
  - CLI flags
  - environment variables
  - local YAML
  - base YAML
  - built-in defaults
- API key policy:
  - prefer `OPENAI_API_KEY`
  - allow `domainfinder.local.yaml` fallback for local-only use
  - do not put API keys in `domainfinder.yaml`
- Generation model:
  - `-generate` enables OpenAI stem generation
  - generation happens in batches
  - each batch is normalized, deduped, and processed before the next batch
  - generated outputs must be stems only, not FQDNs

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
- `internal/termui` owns the streaming interactive console on `stderr`.
- Interactive console prints:
  - a small startup header
  - a reusable active stem line
  - durable scrolling emitted stem rows
  - a compact final completion line
- Durable results still go to `stdout` or `-out`.
- JSONL bypasses `termui` entirely.

## Current CLI capabilities

- Repeated `-zone name=path` flags load explicitly named zones.
- Repeated `-candidate stem` flags add explicit stems.
- `-candidate-file <path>` loads stems from a text file.
- `-candidate-stdin` loads stems from stdin.
- `-generate <prompt>` requests OpenAI-generated stems.
- `-generate-count`, `-generate-batch-size`, and `-generate-model` override generation config.
- `-format text|jsonl` selects a human-readable or machine-readable output mode.
- `-filter all|absent-in-all` controls which results are emitted.
- `-out <path>` writes durable output to a file instead of stdout.
- `-interactive` forces interactive text console mode.
- `-no-interactive` forces deterministic fallback text mode.

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
