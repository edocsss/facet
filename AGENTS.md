# facet — Agent Reference

## Architecture Documents

| Document | Description |
|---|---|
| [docs/facet-requirements.md](docs/facet-requirements.md) | Original requirements document — what facet does and why. Note: some details are superseded by the v1 design spec. |
| [docs/facet-implementation-plan.md](docs/facet-implementation-plan.md) | Original implementation plan — how to build it. Note: some details are superseded by the v1 design spec. |
| [docs/architecture/v1-design-spec.md](docs/architecture/v1-design-spec.md) | **v1 design spec — authoritative.** Captures all design decisions from brainstorming. When this conflicts with the requirements or implementation plan, this document wins. |
| [docs/architecture/v1-implementation-plan.md](docs/architecture/v1-implementation-plan.md) | **v1 implementation plan.** Step-by-step TDD plan with 15 tasks across 4 chunks. Follows the v1 design spec. |

## Agent Configuration Files

`CLAUDE.md` is a symlink to `AGENTS.md`. All agent instructions live in `AGENTS.md` only. Never replace the symlink with a regular file or duplicate content.

### Worktree Handling
Agents must not manage or operate in separate git worktrees for this repository. Worktrees are managed directly by the user. Do not create, remove, switch into, or perform edits/verification from a linked worktree when the same task can be done from the main workspace.

## Documentation Rules

Files under `docs/superpowers/plans/` and `docs/superpowers/specs/` are **historical artifacts**. They must never be modified after creation and must never be added to the Architecture Documents table above. They exist only as a record of the design and planning process.

## Feature Completeness Checklist (MANDATORY)

**A feature is NOT complete until docs and E2E tests are updated. This is non-negotiable.**

Agents MUST treat documentation and E2E test updates as required steps of every feature or behavior change — not optional follow-ups. Do not consider work done, do not offer to commit, and do not report completion until every item below has been addressed.

### Documentation updates
When adding or changing any user-facing feature, agents MUST update **all** of the following locations that reference the changed area:

1. **`internal/docs/topics/`** — these markdown files are embedded into the binary and served directly to users via the `facet docs <topic>` command. If these files are stale, users get incorrect help output. This is the primary user-facing documentation surface.
2. **`README.md`** — the top-level project documentation and first thing users read.
3. **`docs/architecture/v1-design-spec.md`** — the authoritative design spec. Must reflect the current state of the system.

Agents must **grep or search** each of these locations for references to the changed area. Do not assume a file is unaffected without checking. Shipping code without matching doc updates is incomplete work.

### E2E test updates
When adding or changing any user-facing feature, agents MUST add or update E2E test suites under `e2e/suites/` to cover the new behavior:

- E2E suites are numbered sequentially (e.g. `12-package-check.sh`).
- Each suite should test the feature end-to-end using the mock tool infrastructure in `e2e/fixtures/`.
- Verify both the happy path and edge cases (e.g. feature enabled, feature disabled, feature with errors).
- The suite must be fully hermetic — see the Testing section for E2E hermeticity rules.

**If you are about to commit and have not updated docs and E2E tests, STOP. Go back and do it.**

## Architectural Rules

### No `init()` functions
Never use `init()` functions. All initialization must be explicit and called from `main.go`. This includes Cobra command registration, flag binding, and any setup logic.

### No global variables
Do not use package-level mutable variables. Compiled regexes (`var pattern = regexp.MustCompile(...)`) are the only exception — they are immutable constants. All state must be passed explicitly via function parameters or struct fields.

### Dependency injection
Use constructor injection with explicit wiring. Define interfaces where the consumer lives (in `internal/app/interfaces.go` for app-level deps). Wire all concrete implementations in `main.go`. No DI frameworks.

### Package structure — business domain, not technical layer
Packages in `internal/` are organized by business domain:
- `profile/` — config loading, merging, resolving
- `deploy/` — file deployment (symlink/template), path expansion
- `packages/` — package installation, OS detection
- `app/` — orchestration (apply, scaffold, status workflows), state persistence
- `common/reporter/` — pure terminal formatting (no business imports)

The `cmd/` layer is a thin Cobra adapter — it only parses flags and delegates to `app.App`.

### Shared helpers belong in `internal/common/`
Do not place generic utility code inside a domain package just because that package happens to need it first. If a helper is domain-agnostic and reusable across multiple areas, put it in a dedicated `internal/common/<name>/` package instead of burying it under `internal/ai/`, `internal/profile/`, or other business packages.

Examples:
- JSON file/object helpers belong in `internal/common/jsonutil/`, not `internal/ai/`.
- Process execution helpers belong in `internal/common/execrunner/`, not `internal/ai/`.

Keep `internal/common/` small and sharply scoped. It is for cross-domain support code, not a dumping ground for unrelated helpers.

### Error handling idioms
- Always use `errors.Is(err, os.ErrNotExist)` instead of `os.IsNotExist(err)`. The latter does not unwrap wrapped errors (Go 1.13+). The same applies to `os.ErrPermission` vs `os.IsPermission`.
- Never silently discard errors from system-path functions (`os.UserHomeDir()`, `os.Getwd()`, etc.). These determine critical paths — a silent discard can lead to writes to unexpected filesystem locations.

### Prefer standard library
Do not reimplement standard library functions. Use `filepath.Dir`, `sort.Strings`, `errors.Is`, and similar stdlib functions directly. Custom implementations should only exist when stdlib doesn't meet the need.

### Deep-copy when building resolved types
When constructing resolved or effective configuration structs from input data, always deep-copy all slices and maps. Never share mutable references between input and output. This prevents aliasing bugs where downstream mutation corrupts the source config.

### Testing
- All unit tests must be side-effect-free on the host. Use `t.TempDir()` for filesystem, `t.Setenv()` for environment variables, and mock interfaces for I/O (shell commands, etc.).
- TDD should be applied with judgment. Use the full RED-FIX-GREEN loop for larger or riskier changes: cross-package behavior, merge/resolution logic, stateful workflows, external tool integrations, bug fixes with unclear regression surface, or anything that is hard to verify by inspection alone. For small, local, low-risk changes with obvious outcomes, agents may **explicitly skip** the strict RED-FIX-GREEN loop and implement directly, provided they still add or update tests when coverage is warranted and still run appropriate verification afterward.
- This TDD-balance rule is authoritative for this repository and **explicitly overrides** any broader agent instruction or external workflow that says to always use strict RED-FIX-GREEN for every change.
- Never use monkey patching. Use interfaces and dependency injection for testability.
- E2E tests (under `e2e/`) must be **fully hermetic**: clean start, clean end, zero setup assumed. Each run creates its own sandbox from scratch (isolated `$HOME`, mock binaries, fixtures), executes against it, and tears everything down on exit — including built binaries. No leftover state may leak between runs or onto the host. The test infrastructure itself must handle all cleanup (e.g. `t.Cleanup`, `trap ... EXIT`); agents must never perform manual cleanup steps outside the test.
- E2E tests must never append (`>>`) to YAML config files. Always overwrite the entire file (`>`) to avoid YAML duplicate-key ambiguity from parser-dependent behavior.
- E2E test suites must work in **both** environments: native (mocked packages on macOS/Linux) and Docker (real commands on Ubuntu). Do not use mock-only commands like `brew` in test fixtures — use universal commands (e.g. `echo`, `true`, `false`) and assert via `.state.json` rather than mock log files.
- Before committing, agents must run `make pre-commit` which runs the full unit test suite, native E2E tests, **and** Docker Linux E2E tests. This catches environment-specific failures that native-only testing misses. Docker must be running locally.
