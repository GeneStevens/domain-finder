# domainfinder Codex Policy

## Task Packet Model

- Treat each user request as one bounded task packet.
- Keep changes scoped to the packet's objective and acceptance criteria.
- Prefer small, atomic commits that cleanly represent one completed packet.
- Do not bundle unrelated cleanup or speculative refactors into the same commit.

## Required Git Start Behavior

- Run `git status --short` at the start of every task.
- If unexpected uncommitted changes are present, report them before proceeding.
- Never discard, overwrite, or revert unrelated user changes.
- Work around unrelated changes rather than reshaping them unless the user explicitly asks.

## Required Validation Before Commit

- Run the formatting, tests, and other validation required by the current task packet.
- Do not commit if validation fails, if acceptance criteria are not met, or if the work is incomplete.
- If blocked, leave changes uncommitted and explain the blocker clearly in the final response.

## Commit Scoping

- Stage only files relevant to the current task packet.
- Make at most one bounded commit for a successful packet unless the user explicitly asks for a different strategy.
- Keep the commit atomic so it can be reviewed, reverted, or cherry-picked cleanly.

## Commit Message Style

- Use: `domainfinder: <short imperative summary>`
- Messages must be concise, human-readable, and useful in `git log`.
- Messages should help both human reviewers and future AI reviewers understand the change quickly.

Examples:

- `domainfinder: add repo codex git workflow policy`
- `domainfinder: support candidate ingestion from stdin`
- `domainfinder: add absent-in-all report filtering`

## Required Git End Behavior

- Run `git status --short` before staging so the final change set is explicit.
- Stage only the files for the task packet.
- Commit only when the task has passed required validation and satisfies acceptance criteria.

## Final Response Requirements

- State whether a commit was created.
- If committed, include the commit hash and exact commit message.
- If not committed, explain why not.
- Mention any noteworthy git status context that affected the task.
