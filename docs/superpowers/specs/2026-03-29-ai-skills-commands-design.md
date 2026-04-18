# AI Skills Commands Design Spec

**Date:** 2026-03-29
**Status:** Approved

## Summary

Add `facet ai skills check` and `facet ai skills update` commands as thin passthroughs to the `npx skills` CLI. Also document that the existing `source` field in skill entries supports private repos, SSH URLs, and local paths.

## Motivation

The `npx skills` CLI supports checking for available updates (`npx skills check`) and updating installed skills to their latest versions (`npx skills update`). Facet currently wraps `npx skills add` and `npx skills remove` during `facet apply`, but provides no way to check for or apply skill updates outside of a full apply cycle.

Additionally, the `source` field in skill entries already supports multiple formats (GitHub shorthand, SSH URLs, local paths, HTTPS URLs) because facet passes it verbatim to `npx skills add`. This is not documented.

## Design

### New CLI Commands

```
facet ai skills check    # passthrough to: npx skills check
facet ai skills update   # passthrough to: npx skills update
```

**Command hierarchy:** `facet` -> `ai` (group, help only) -> `skills` (group, help only) -> `check` | `update` (leaf commands).

Running `facet ai` or `facet ai skills` alone prints help/usage text. Only the leaf commands perform actions.

Both commands are **thin passthroughs** — they shell out to `npx skills check` / `npx skills update` and stream output directly to stdout/stderr. No filtering, no state manipulation. Facet's `.state.json` is not read or written by these commands.

### Interface Changes

Extend the existing `SkillsManager` interface with two new methods:

```go
type SkillsManager interface {
    Install(source string, skills []string, agents []string) error
    Remove(skills []string, agents []string) error
    Check() error   // new
    Update() error  // new
}
```

`NPXSkillsManager` implements `Check()` by running `npx skills check` and `Update()` by running `npx skills update`. Unlike `Install`/`Remove` (which use `CommandRunner.Run()`), these methods need to stream output directly to the terminal so the user sees results in real time. This may require a new `CommandRunner` method (e.g. `RunInteractive()`) or direct use of `os/exec` with `Stdout`/`Stderr` wired to `os.Stdout`/`os.Stderr`.

### Dependency Flow

```
cmd/ai_skills_check.go  -> app.App.AISkillsCheck() -> SkillsManager.Check()  -> npx skills check
cmd/ai_skills_update.go -> app.App.AISkillsUpdate() -> SkillsManager.Update() -> npx skills update
```

### cmd Layer

New files in `cmd/`:

| File | Purpose |
|------|---------|
| `ai.go` | Registers `ai` parent command (help only) |
| `ai_skills.go` | Registers `skills` subcommand under `ai` (help only) |
| `ai_skills_check.go` | Leaf command, delegates to `app.App.AISkillsCheck()` |
| `ai_skills_update.go` | Leaf command, delegates to `app.App.AISkillsUpdate()` |

### app Layer

New file `internal/app/ai_skills.go` with two methods on `App`:

- `AISkillsCheck() error` — calls `SkillsManager.Check()`
- `AISkillsUpdate() error` — calls `SkillsManager.Update()`

These methods validate that `SkillsManager` is available (npx is installed) and delegate directly.

### Scoping

These commands operate on **all globally installed skills** (full passthrough to `npx skills`), not just skills managed by facet. This keeps the implementation simple and avoids divergence between what facet tracks and what the skills CLI knows about.

## Documentation Updates

### `internal/docs/topics/ai.md`

1. **Skills source formats** — add a subsection documenting that `source` supports:
   - GitHub shorthand: `owner/repo`
   - SSH URLs: `git@github.com:org/private-repo.git`
   - Local paths: `./my-local-skills` or `/absolute/path`
   - HTTPS URLs: `https://github.com/org/repo`
   - Private repos work via system-level git auth (SSH keys, `gh auth login`)

2. **Skills management commands** — add a subsection documenting:
   - `facet ai skills check` — check for available skill updates
   - `facet ai skills update` — update all skills to latest versions

### `internal/docs/topics/commands.md`

Add the new `facet ai skills check` and `facet ai skills update` commands to the CLI reference.

### `README.md`

Add a brief mention of AI configuration and the skills commands.

### `docs/architecture/v1-design-spec.md`

The v1 design spec explicitly defers AI features. No update needed — the AI feature set is documented in `ai.md`.

## E2E Testing

New suite `e2e/suites/13-ai-skills-commands.sh` covering:

1. **`facet ai skills check`** — mock npx binary, assert `npx skills check` is invoked
2. **`facet ai skills update`** — mock npx binary, assert `npx skills update` is invoked
3. **Error case** — npx not found or command fails, verify facet reports error cleanly
4. **Help output** — `facet ai`, `facet ai skills`, and leaf commands with `--help` produce usage text

Tests use mock binaries to intercept `npx` calls, matching the existing E2E pattern from `10-ai-config.sh`.

## Out of Scope

- Plugin management for Claude Code or Codex
- Filtering check/update to only facet-managed skills
- Changes to `facet status`
- Changes to `facet apply` flow
- Auto-update or scheduled update checks
