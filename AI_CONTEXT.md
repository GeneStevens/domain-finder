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
- `internal/candidates`: candidate loading, normalization, merge, and dedupe
- `internal/match`: stable candidate result model and classification
- `internal/report`: filter modes and summary stats
- `internal/output`: text and JSONL durable rendering
- `internal/termui`: transient stderr activity-line rendering
- `testdata/small`: tiny deterministic fixtures used by tests
- `testdata/slices`: reserved for small realistic slices, never giant CZDS files

## Domain normalization policy

- Stored domains are normalized FQDNs: lowercase and trailing-dot stripped.
- Candidate ingestion normalizes CLI, file, and stdin inputs the same way.
- Candidates must be full FQDNs.
- Relative labels like `example` are rejected in this phase.

## Candidates layer

- Sources:
  - repeated `-candidate` flags
  - optional `-candidate-file`
  - optional `-candidate-stdin`
- File/stdin format:
  - plain text
  - one candidate per line
  - blank lines ignored
  - `#` comment lines ignored
- Merge order:
  - CLI candidates first
  - file candidates second
  - stdin candidates third
- Dedupe:
  - preserve first-seen order across all sources
- Error handling:
  - invalid or empty normalized candidates return an error

## Result model

- `candidate`: normalized FQDN candidate string
- `zones`: ordered `zone/present` results in deterministic zone-name order
- `present_in_any`: true if found in at least one loaded zone
- `absent_in_all`: true if not found in any loaded zone

## Report, output, and terminal UX layers

- Report filters:
  - `all`
  - `absent-in-all`
- Durable text output renders emitted results first, then a deterministic summary.
- JSONL output emits one stable JSON object per emitted candidate result.
- Summary is intentionally text-only and is omitted from JSONL output.
- `internal/termui` renders a single reusable transient activity line on `stderr`.
- Text mode uses live stderr progress; JSONL mode disables it completely.
- Durable results always remain on `stdout` or the configured `-out` file.

## Current CLI capabilities

- Repeated `-zone name=path` flags load explicitly named zones.
- Repeated `-candidate fqdn` flags add explicit candidates.
- `-candidate-file <path>` loads candidates from a text file.
- `-candidate-stdin` loads candidates from stdin.
- `-format text|jsonl` selects a human-readable or machine-readable output mode.
- `-filter all|absent-in-all` controls which results are emitted.
- `-out <path>` writes durable output to a file instead of stdout.

## Testing rule

Do not add tests that depend on full production zone files. Use only small
fixtures or tiny realistic slices under `testdata/`.

## Repo workflow policy

- Persistent Codex operating policy now lives in the repo root `AGENTS.md`.
- Future task packets should follow its git-status, validation, commit-scoping,
  commit-message, and final-reporting rules by default.

## Deferred work

- No concurrency yet.
- No full-screen TUI.
- No advanced terminal UI beyond a single stderr activity line.
- No registrar checks or probabilistic availability logic.
- No filename-based zone inference.
- No large-file optimization beyond streaming reads.
