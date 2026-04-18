# facet docs — Design Spec

## Summary

Add a `facet docs [topic]` command that prints markdown documentation to stdout. The primary audience is AI agents that need to understand facet's config format and capabilities to help users set up their environment independently.

## Design Decisions

### Output target
Stdout markdown. No file writing, no side effects.

### Static only
The command prints the same documentation regardless of whether a config repo is present. No config loading, no state reading. Agents can read YAML files directly for context-aware information.

### Progressive disclosure via topics
Instead of one massive output, documentation is split into focused topics. `facet docs` (no argument) prints a short overview with usage instructions and the topic list. `facet docs <topic>` prints that topic's content.

### Implementation approach
`go:embed` — each topic is a `.md` file under `internal/docs/topics/`, embedded at compile time. Real markdown files that are easy to review and edit, with zero runtime I/O.

---

## Command Interface

```
facet docs [topic]
```

- **No argument:** prints overview (what facet is, how to use `facet docs <topic>`, list of topics with descriptions).
- **One argument:** prints the content of that topic's embedded markdown file.
- **Invalid topic:** prints error: `Unknown topic "<name>". Run "facet docs" to see available topics.`

No flags. Topic is a positional argument.

---

## Topics

| Topic | Description | Key content |
|-------|-------------|-------------|
| `quickstart` | Set up facet from scratch | `facet init` → create `.local.yaml` → create profile → `facet apply`. Step-by-step, minimal explanation. |
| `config` | YAML config format and file structure | Directory layout (config repo vs state dir), `facet.yaml`, `base.yaml`, `profiles/<name>.yaml`, `.local.yaml`. Full field reference with types. `extends: base` is required and is the only valid value. `.local.yaml` must exist (fatal if missing). |
| `variables` | Variable substitution syntax and rules | `${facet:var_name}` syntax. Nested dot notation (`${facet:git.email}`). Resolved in: file contents, install commands, config source paths. NOT resolved in: config target paths (those use env var expansion). Undefined = fatal error. No recursive resolution. |
| `packages` | Package installation entries | `name` (required) + `install` (required). Simple string command vs per-OS map. No plain-string shorthand. Packages run every apply — must be idempotent. Failed installs are non-fatal. No uninstallation. |
| `deploy` | Config file deployment | Target path expansion (`~`, `$HOME`, `$VAR`). Source paths relative to config dir, cannot escape it. Auto-detected strategy: no `${facet:` → symlink, contains `${facet:` → template, directory → symlink. Parent dirs created automatically. |
| `ai` | AI agent configuration | `agents` list (valid names: `claude-code`, `cursor`, `codex`). `permissions` (allow/deny lists). `skills` (source, skills list, optional agent scoping). `mcps` (name, command, args, env, optional agent scoping). `overrides` (per-agent permission overrides). Full YAML examples matching the implemented schema. |
| `merge` | How layers combine | 3 layers: base → profile → .local. Per-field rules: vars (deep merge, last writer wins per leaf), packages (union by name, last writer wins), configs (shallow merge, same target = later wins), AI permissions (union, deny is append-only), MCPs (union by name). Type conflicts = fatal. |
| `commands` | CLI commands and flags reference | `facet init`, `facet apply <profile>` (flags: `--force`, `--dry-run`, `--skip-failure`), `facet status`, `facet docs [topic]`. Global flags: `--config-dir` / `-c`, `--state-dir` / `-s`. Exit codes: 0 success, 1 fatal error. |
| `examples` | Complete working config examples | A full base.yaml + profiles/work.yaml + .local.yaml showing packages, configs with variables, and AI configuration. Shows expected merge result. |

---

## Overview Output

When `facet docs` is run with no argument, it prints:

```markdown
# facet

facet manages developer environment setup across machines. You describe packages,
config files, and AI tool configuration in YAML profiles, and facet makes it real.

## Usage

  facet docs <topic>

## Topics

  quickstart   Set up facet from scratch (start here)
  config       YAML config format and file structure
  variables    Variable substitution syntax and rules
  packages     Package installation entries
  deploy       Config file deployment (symlink vs template)
  ai           AI agent configuration (permissions, MCPs, skills)
  merge        How base, profile, and .local layers combine
  commands     CLI commands and flags reference
  examples     Complete working config examples

Run "facet docs <topic>" to read a specific topic.
```

---

## File Layout

```
cmd/
  docs.go                 # Cobra command — thin adapter, delegates to internal/docs

internal/docs/
  docs.go                 # go:embed directives, topic registry, Overview/Render functions
  docs_test.go            # all topics load, no empty files, overview lists match registry
  topics/
    quickstart.md
    config.md
    variables.md
    packages.md
    deploy.md
    ai.md
    merge.md
    commands.md
    examples.md
```

---

## Package API

```go
package docs

import "embed"

//go:embed topics/*.md
var topicFS embed.FS

type Topic struct {
    Name        string
    Description string
}

// Topics returns the ordered list of available topics.
func Topics() []Topic

// Render returns the markdown content for a given topic name.
// Returns an error if the topic does not exist.
func Render(topic string) (string, error)

// Overview returns the text printed by `facet docs` with no argument.
// Built dynamically from Topics() so the list can't drift from the registry.
func Overview() string
```

---

## Cobra Command

`cmd/docs.go` registers the `docs` command on the root command. It does NOT receive `*app.App` — this command has no business logic dependencies.

- `Args: cobra.MaximumNArgs(1)`
- No args → `fmt.Print(docs.Overview())`
- One arg → `content, err := docs.Render(arg)` → print or error

The command is registered in `cmd/root.go` alongside `apply`, `init`, and `status`. It does not use `--config-dir` or `--state-dir` (no config loading). No changes to `main.go` wiring or the `App` struct.

---

## Testing

- **Unit tests for `internal/docs`:**
  - Every topic in the registry has a corresponding non-empty `.md` file
  - `Render` returns content for all valid topics
  - `Render` returns an error for unknown topics
  - `Overview()` contains every topic name from `Topics()`

- **No E2E tests needed** — this is pure static output with no side effects, no config loading, no state.

---

## What This Does NOT Do

- No context-aware output (no config loading, no profile inspection)
- No pager integration (agents don't use pagers)
- No `--format` flag (always markdown)
- No man page generation
- No `--all` flag to dump everything (agents should use progressive disclosure)
