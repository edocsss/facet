# Design: Verbose Apply Timing

**Date:** 2026-05-16
**Status:** Approved

## Problem

`facet apply --verbose` shows stage progress, but it does not show where time is
spent. When an apply is slow, users can see the broad stage currently running but
cannot tell whether the delay came from profile resolution, remote `extends`,
config deployment, package checks, package installs, scripts, AI permissions,
skill installs, skill verification, MCP registration, or state persistence.

This makes performance work guess-heavy. It also makes normal troubleshooting
harder because slow items inside a stage are invisible until the final report.

## Goals

- Add complete timing visibility for the `facet apply` pipeline.
- Emit timing output only when `--verbose` or `-v` is set.
- Keep normal non-verbose output unchanged.
- Cover both high-level steps and item-level operations.
- Avoid printing secrets or expanded environment values.
- Make the timing data useful before any future concurrency or skip work.

## Non-Goals

- No concurrency changes.
- No apply skip logic.
- No state file format changes.
- No new machine-readable tracing format.
- No command output streaming changes.

## Output Model

Verbose progress lines gain outcome and duration. Long-running grouped stages get
a start line and a completion line.

Example:

```text
facet apply work ... start
Loading metadata ... ok 2ms
Loading profile ... ok 3ms
Resolving extends ... ok 412ms
Merging layers ... ok 1ms
Resolving variables ... ok 4ms

Deploying configs ... start
  -> ~/.gitconfig ... ok 7ms
  -> ~/.zshrc ... ok 5ms
Deploying configs ... done 12ms

Installing packages ... start
  -> ripgrep check ... ok 18ms
  -> node check ... failed 21ms
  -> node install ... ok 14.2s
Installing packages ... done 14.3s

Applying AI configuration ... start
  -> permissions claude-code ... ok 4ms
  -> skills install owner/repo [claude-code,codex] ... ok 8.2s
  -> skills verify owner/repo ... ok 12ms
  -> mcp register playwright claude-code ... ok 1.1s
Applying AI configuration ... done 9.5s

Writing state ... ok 2ms
facet apply work ... done 23.1s
```

Failures include duration and a short error line:

```text
  -> skills install owner/repo [cursor] ... failed 8.2s
     error: skills install: exit status 1
```

The exact duration formatting is:

- sub-second durations: milliseconds, e.g. `12ms`
- one second or longer: one decimal place, e.g. `8.2s`

## Coverage

Timing covers the whole apply pipeline:

| Area | Timed operations |
|---|---|
| Setup | load `facet.yaml`, load profile, validate profile, annotate profile |
| Resolution | resolve `extends`, load `.local.yaml`, merge layers, validate merged config, resolve variables |
| Dry run | dry-run rendering when `--dry-run` is selected |
| State preflight | create state dir, canary write, read previous state |
| Unapply | unapply decision, each config removal, AI unapply sections |
| Configs | source resolution, target expansion, each config deployment, rollback if needed |
| Scripts | each `pre_apply` and `post_apply` script by script name |
| Packages | each package OS decision, check command, install command, result |
| AI | effective config resolution, permissions per agent, skill orphan detection, each skill remove, each skill install, skill lock verification, MCP orphan removal, MCP registration per provider and agent |
| State finalization | carry-forward handling for skipped stages, final state write |
| Reporting | final report rendering |

## Privacy Rules

Verbose timing must not expose secrets.

- Do not print MCP environment values.
- If MCP env detail is useful, print only env key names.
- Do not print expanded variable values.
- Do not print full script or package command strings in new timing lines.
- Log scripts by `name`.
- Log packages by `name` plus operation label (`check`, `install`, `skip`).
- Log skills by `source`, requested skill names or `all`, and target agents.
- Log MCPs by `name` and target agent/provider.

Existing non-timing output behavior is unchanged.

## Architecture

Add a small verbose timing API behind the existing reporter boundary. The App and
AI orchestrator can call it unconditionally; the concrete reporter suppresses it
when verbose mode is off.

Suggested API shape:

```go
type ProgressTimer interface {
    ProgressStart(label string) func(outcome string, err error)
    ProgressStep(label string, fn func() error) error
}
```

The final implementation can choose different names, but it must provide these
capabilities:

- print a start line for grouped operations
- print an end line with outcome and elapsed duration
- print item-level duration lines
- print a short error line when an operation fails
- no-op cleanly when verbose is disabled

The timing helper belongs at the reporting boundary, likely
`internal/common/reporter`, because it is terminal-formatting behavior. Domain
packages should provide labels and outcomes, not ANSI details.

## Apply Integration

`internal/app/apply.go` wraps each apply step in timing calls. The current
control flow remains the source of truth:

1. Parse stages.
2. Load and resolve config.
3. Handle dry run.
4. Prepare state writes.
5. Read previous state.
6. Unapply previous state when needed.
7. Run selected stages.
8. Carry forward skipped stage state.
9. Write final state.
10. Print final report.

Existing `Progress` messages can either be converted into timed messages or
kept as start lines for grouped operations. The implementation should avoid
duplicating both an old progress line and a new timing line for the same event.

## Package Integration

Package timing needs item-level visibility inside `InstallAll`, because package
checks and installs currently happen inside the package installer.

The package installer should accept a progress/timing dependency or callback via
constructor injection. It should report:

- package skipped because no OS command exists
- package check started and finished
- package install started and finished
- package result status

The installer must remain side-effect-free in unit tests through its existing
mock runner pattern.

## AI Integration

The AI orchestrator should emit timings for its internal units:

- permissions removal for dropped agents
- permissions apply per current agent
- skill orphan detection per source/agent when it reads installed skills
- skill remove calls
- skill install calls
- skill lock verification after install
- MCP orphan removal per agent
- MCP registration per MCP and agent

The skills manager may also emit command-level timings around `npx` calls, but
there should not be duplicate timing lines for the same command. Prefer the
orchestrator for user-facing operation labels and the skills manager only if the
lower-level command timing is otherwise unavailable.

## Error Handling

Timing must not change apply behavior.

- Fatal errors remain fatal.
- Non-fatal package and AI errors remain non-fatal.
- Rollback behavior remains unchanged.
- If timing output itself fails to write, behavior follows the existing reporter
  behavior.

Each timed operation reports the same outcome the existing code already uses.
For non-fatal failures, the operation line says `failed` and the apply continues.

## Testing

### Unit tests

- Reporter timing is silent when verbose is disabled.
- Reporter timing prints start, success, failure, duration, and short error lines
  when verbose is enabled.
- Duration formatting uses milliseconds below one second and seconds at one
  second or longer.
- `Apply` emits timing lines for all major steps under verbose mode.
- `Apply` emits no timing lines without verbose mode.
- Config deploy timing covers success and rollback paths.
- Script timing covers successful and failing scripts.
- Package timing covers OS skip, check success, check failure plus install
  success, and install failure.
- AI timing covers permissions, skill install, skill verification, skill remove,
  MCP registration, and non-fatal AI failures.
- Secret values from MCP env and resolved vars do not appear in verbose output.

### E2E tests

Update or extend the verbose E2E suite to assert:

- non-verbose apply does not include timing lines
- `facet apply work --verbose` includes total apply timing
- verbose output includes config, package, script, and AI timing lines
- a failing non-fatal package or AI operation includes `failed <duration>`
- secret env values are absent from verbose output

## Documentation Updates

Implementation must update all user-facing docs that describe `--verbose`:

- `internal/docs/topics/commands.md`
- `internal/docs/topics/ai.md` if AI timing examples are mentioned
- `README.md`
- `docs/architecture/v1-design-spec.md`

The docs should describe `--verbose` as progress plus timing diagnostics for the
full apply pipeline.

## Open Decisions Resolved

- Timing is complete across `facet apply`, not AI-only.
- Timing is enabled only by `--verbose` / `-v`.
- This design does not introduce concurrency.
- This design does not introduce convergence skipping.
