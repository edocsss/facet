# facet docs Command — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `facet docs [topic]` command that prints embedded markdown documentation to stdout, enabling AI agents to learn facet's config format and capabilities via progressive disclosure.

**Architecture:** New `internal/docs` package with `go:embed` topic files and a thin `cmd/docs.go` Cobra adapter. No dependencies on `app.App`, no config loading, no state. The command is self-contained static output.

**Tech Stack:** Go 1.23, `embed` stdlib package, Cobra, testify for assertions.

---

## File Structure

```
cmd/
  docs.go                    # Cobra command — thin adapter

internal/docs/
  docs.go                    # go:embed, topic registry, Overview(), Render()
  docs_test.go               # unit tests for the docs package
  topics/
    quickstart.md            # setup from scratch
    config.md                # YAML format and file structure
    variables.md             # variable substitution
    packages.md              # package entries
    deploy.md                # config file deployment
    ai.md                    # AI agent configuration
    merge.md                 # layer merge rules
    commands.md              # CLI reference
    examples.md              # complete working examples
```

---

### Task 1: Create `internal/docs` Package with Registry and Embed

**Files:**
- Create: `internal/docs/docs.go`
- Create: `internal/docs/topics/quickstart.md` (placeholder: `# Quickstart`)
- Create: `internal/docs/docs_test.go`

- [ ] **Step 1: Create a placeholder topic file so `go:embed` has something to embed**

Create `internal/docs/topics/quickstart.md`:

```markdown
# Quickstart
```

- [ ] **Step 2: Write the failing test for `Topics()`**

Create `internal/docs/docs_test.go`:

```go
package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopics_ReturnsNonEmptyList(t *testing.T) {
	topics := Topics()
	assert.NotEmpty(t, topics, "Topics() should return at least one topic")
}

func TestTopics_EachHasNameAndDescription(t *testing.T) {
	for _, topic := range Topics() {
		assert.NotEmpty(t, topic.Name, "topic name must not be empty")
		assert.NotEmpty(t, topic.Description, "topic %q description must not be empty", topic.Name)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestTopics`
Expected: compilation error — `Topics` not defined.

- [ ] **Step 4: Write the `internal/docs/docs.go` implementation**

```go
package docs

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed topics/*.md
var topicFS embed.FS

// Topic holds a topic's name and description for the index.
type Topic struct {
	Name        string
	Description string
}

var registry = []Topic{
	{"quickstart", "Set up facet from scratch (start here)"},
	{"config", "YAML config format and file structure"},
	{"variables", "Variable substitution syntax and rules"},
	{"packages", "Package installation entries"},
	{"deploy", "Config file deployment (symlink vs template)"},
	{"ai", "AI agent configuration (permissions, MCPs, skills)"},
	{"merge", "How base, profile, and .local layers combine"},
	{"commands", "CLI commands and flags reference"},
	{"examples", "Complete working config examples"},
}

// Topics returns the ordered list of available topics.
func Topics() []Topic {
	return registry
}

// Render returns the markdown content for a given topic name.
// Returns an error if the topic does not exist.
func Render(topic string) (string, error) {
	for _, t := range registry {
		if t.Name == topic {
			data, err := topicFS.ReadFile("topics/" + topic + ".md")
			if err != nil {
				return "", fmt.Errorf("failed to read topic %q: %w", topic, err)
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("unknown topic %q — run \"facet docs\" to see available topics", topic)
}

// Overview returns the text printed by `facet docs` with no argument.
func Overview() string {
	var b strings.Builder
	b.WriteString("# facet\n\n")
	b.WriteString("facet manages developer environment setup across machines. You describe packages,\n")
	b.WriteString("config files, and AI tool configuration in YAML profiles, and facet makes it real.\n\n")
	b.WriteString("## Usage\n\n")
	b.WriteString("  facet docs <topic>\n\n")
	b.WriteString("## Topics\n\n")

	// Find max name length for alignment
	maxLen := 0
	for _, t := range registry {
		if len(t.Name) > maxLen {
			maxLen = len(t.Name)
		}
	}

	for _, t := range registry {
		padding := strings.Repeat(" ", maxLen-len(t.Name))
		fmt.Fprintf(&b, "  %s%s   %s\n", t.Name, padding, t.Description)
	}

	b.WriteString("\nRun \"facet docs <topic>\" to read a specific topic.\n")
	return b.String()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestTopics`
Expected: PASS

- [ ] **Step 6: Write tests for `Render()` and `Overview()`**

Add to `internal/docs/docs_test.go`:

```go
func TestRender_ValidTopic(t *testing.T) {
	content, err := Render("quickstart")
	assert.NoError(t, err)
	assert.Contains(t, content, "# Quickstart")
}

func TestRender_InvalidTopic(t *testing.T) {
	_, err := Render("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown topic")
}

func TestRender_AllRegisteredTopicsHaveFiles(t *testing.T) {
	for _, topic := range Topics() {
		content, err := Render(topic.Name)
		assert.NoError(t, err, "topic %q should have an embedded file", topic.Name)
		assert.NotEmpty(t, content, "topic %q should have non-empty content", topic.Name)
	}
}

func TestOverview_ContainsAllTopicNames(t *testing.T) {
	overview := Overview()
	for _, topic := range Topics() {
		assert.Contains(t, overview, topic.Name, "overview should list topic %q", topic.Name)
	}
}

func TestOverview_ContainsUsageInstructions(t *testing.T) {
	overview := Overview()
	assert.Contains(t, overview, "facet docs <topic>")
	assert.Contains(t, overview, "Run \"facet docs <topic>\" to read a specific topic.")
}
```

- [ ] **Step 7: Run all tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v`
Expected: `TestRender_AllRegisteredTopicsHaveFiles` will FAIL because only `quickstart.md` exists. Create the remaining placeholder files.

- [ ] **Step 8: Create remaining placeholder topic files**

Create each file with a single `# Title` line so the embed and tests pass. These will be filled with real content in later tasks.

`internal/docs/topics/config.md`: `# Config`
`internal/docs/topics/variables.md`: `# Variables`
`internal/docs/topics/packages.md`: `# Packages`
`internal/docs/topics/deploy.md`: `# Deploy`
`internal/docs/topics/ai.md`: `# AI Configuration`
`internal/docs/topics/merge.md`: `# Merge Rules`
`internal/docs/topics/commands.md`: `# Commands`
`internal/docs/topics/examples.md`: `# Examples`

- [ ] **Step 9: Run all tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v`
Expected: ALL PASS

- [ ] **Step 10: Commit**

```bash
git add internal/docs/ && git commit -m "feat: add internal/docs package with topic registry and embed"
```

---

### Task 2: Add Cobra Command in `cmd/docs.go`

**Files:**
- Create: `cmd/docs.go`
- Modify: `cmd/root.go:25-27` (add `rootCmd.AddCommand(newDocsCmd())`)

- [ ] **Step 1: Create `cmd/docs.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"facet/internal/docs"

	"github.com/spf13/cobra"
)

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs [topic]",
		Short: "Show documentation for AI agents",
		Long:  "Prints markdown documentation about facet's configuration format and capabilities.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Fprint(os.Stdout, docs.Overview())
				return nil
			}
			content, err := docs.Render(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, content)
			return nil
		},
	}
}
```

- [ ] **Step 2: Register the command in `cmd/root.go`**

Add after the existing `AddCommand` calls (after line 27):

```go
rootCmd.AddCommand(newDocsCmd())
```

Note: `newDocsCmd()` does NOT take `application`, `configDir`, or `stateDir` parameters — it has no business logic dependencies.

- [ ] **Step 3: Build and verify manually**

Run: `cd /Users/edocsss/aec/src/facet && go build -o /tmp/facet-test . && /tmp/facet-test docs && echo "---" && /tmp/facet-test docs quickstart && echo "---" && /tmp/facet-test docs nonexistent; rm /tmp/facet-test`

Expected:
- First call prints the overview with topic list
- Second call prints "# Quickstart"
- Third call prints an error about unknown topic

- [ ] **Step 4: Run the full test suite to make sure nothing broke**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... 2>&1 | tail -20`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/docs.go cmd/root.go && git commit -m "feat: add facet docs cobra command"
```

---

### Task 3: Write `quickstart.md` Topic

**Files:**
- Modify: `internal/docs/topics/quickstart.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/quickstart.md` with:

```markdown
# Quickstart

Set up facet from scratch in 4 steps.

## 1. Initialize the config repo

```bash
mkdir ~/dotfiles && cd ~/dotfiles
facet init
```

This creates:

```
~/dotfiles/
├── facet.yaml       # metadata marker
├── base.yaml        # shared config (edit this)
├── profiles/        # per-machine profiles
└── configs/         # config files to deploy
```

It also creates `~/.facet/.local.yaml` if it doesn't exist.

## 2. Edit `.local.yaml`

The file at `~/.facet/.local.yaml` holds machine-specific secrets. It must exist — facet
will fail without it. Add any secret variables here:

```yaml
vars:
  github_token: ghp_xxxxxxxxxxxx
```

If you have no secrets yet, leave it as an empty file or with an empty `vars:` block.

## 3. Create a profile

Create `profiles/work.yaml`:

```yaml
extends: base

vars:
  git_email: you@company.com

packages:
  - name: docker
    install: brew install docker

configs:
  ~/.gitconfig: configs/work/.gitconfig
```

The `extends: base` line is required — it must be exactly `base`.

## 4. Apply

```bash
facet apply work
```

This merges base.yaml + your profile + .local.yaml, installs packages, deploys
config files, and configures AI tools.

Run `facet status` to see what was applied.

## Next steps

Run `facet docs <topic>` to learn more:

- `facet docs config` — full YAML format reference
- `facet docs ai` — AI agent configuration
- `facet docs examples` — complete working configs
```

- [ ] **Step 2: Run tests to make sure the file embeds correctly**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_ValidTopic`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/quickstart.md && git commit -m "docs: write quickstart topic for facet docs"
```

---

### Task 4: Write `config.md` Topic

**Files:**
- Modify: `internal/docs/topics/config.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/config.md` with:

```markdown
# Config Format

## Directory layout

facet separates the **config repo** (git-managed, portable) from the **state directory**
(machine-local, never committed).

### Config repo (wherever you cloned it)

```
~/dotfiles/
├── facet.yaml              # metadata marker (required)
├── base.yaml               # shared config (required)
├── profiles/
│   ├── work.yaml
│   └── personal.yaml
└── configs/
    ├── .gitconfig           # may contain ${facet:...} variables
    ├── .zshrc
    └── work/
        └── .gitconfig
```

Detected via `facet.yaml` in the current directory, or `--config-dir` / `-c`.

### State directory (`~/.facet/` by default)

```
~/.facet/
├── .state.json             # written by facet apply
└── .local.yaml             # machine-specific secrets (MUST exist)
```

Override with `--state-dir` / `-s`.

## facet.yaml

Metadata marker. Must exist in the config repo root.

```yaml
min_version: "0.1.0"
```

## base.yaml

Shared config that all profiles extend. Same schema as profiles, minus `extends`.

```yaml
vars:
  git_name: Sarah Chen

packages:
  - name: git
    install: brew install git
  - name: ripgrep
    install: brew install ripgrep

configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.zshrc: configs/.zshrc
```

## Profile files (`profiles/<name>.yaml`)

```yaml
extends: base              # required, must be exactly "base"

vars:
  git_email: sarah@work.com

packages:
  - name: docker
    install: brew install docker

configs:
  ~/.gitconfig: configs/work/.gitconfig   # overrides base
  ~/.npmrc: configs/work/.npmrc           # adds new config
```

`extends` must be `"base"` — no other value is allowed. Multi-level extends is not
supported.

## .local.yaml

Same schema as base.yaml. Lives in the state directory. **Must exist** — facet exits
with a fatal error if it's missing. Create an empty file if you have no secrets.

```yaml
vars:
  acme_db_url: postgres://user:secret@localhost:5432/acme
```

## Field reference

All three file types (base, profile, .local) share this schema:

| Field | Type | Description |
|-------|------|-------------|
| `extends` | string | Profiles only. Must be `"base"`. |
| `vars` | map[string]any | Key-value pairs for variable substitution. Supports nested maps. |
| `packages` | list of PackageEntry | Packages to install. See `facet docs packages`. |
| `configs` | map[string]string | Target path → source path. See `facet docs deploy`. |
| `ai` | AIConfig | AI agent configuration. See `facet docs ai`. |

All fields are optional except `extends` in profiles.
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/config.md && git commit -m "docs: write config topic for facet docs"
```

---

### Task 5: Write `variables.md` Topic

**Files:**
- Modify: `internal/docs/topics/variables.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/variables.md` with:

```markdown
# Variables

## Syntax

```
${facet:var_name}
```

Variables are defined in the `vars:` block of any config layer (base, profile, .local).

## Nested variables

Vars support nested maps with dot notation:

```yaml
vars:
  git:
    name: Sarah Chen
    email: sarah@work.com
  aws:
    region: us-east-1
```

Reference as `${facet:git.email}`, `${facet:aws.region}`. Arbitrary depth is allowed.

## Where variables are resolved

- Config file contents (templates)
- Package `install` commands
- Config source paths (right side of `configs:`)
- MCP entry fields (`command`, `args`, `env` values)

## Where variables are NOT resolved

- Config target paths (left side of `configs:`) — these use environment variable
  expansion (`~`, `$HOME`, `$VAR`) instead. See `facet docs deploy`.
- Variable values themselves — no recursive resolution.

## Undefined variables

Any `${facet:var_name}` referencing an undefined variable is a **fatal error**. The
error message names the variable and suggests where to define it.

```
undefined variable: ${facet:db_url} — define it in .local.yaml or your profile's vars
```

## Rules

- No recursive resolution. If a var's value contains `${facet:...}`, it stays literal.
- Variables merge across layers: base → profile → .local. Later layers win per leaf key.
  See `facet docs merge` for full rules.
- Keep secrets in `.local.yaml` (gitignored).
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/variables.md && git commit -m "docs: write variables topic for facet docs"
```

---

### Task 6: Write `packages.md` Topic

**Files:**
- Modify: `internal/docs/topics/packages.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/packages.md` with:

```markdown
# Packages

## Format

Every package requires `name` and `install`. No shorthand syntax.

```yaml
packages:
  # Same command on all OSes
  - name: ripgrep
    install: brew install ripgrep

  # Per-OS commands
  - name: lazydocker
    install:
      macos: brew install lazydocker
      linux: go install github.com/jesseduffield/lazydocker@latest

  # Single OS only — skipped with a warning on other OSes
  - name: xcode-tools
    install:
      macos: xcode-select --install
```

## Behavior

- Install commands run **every time** `facet apply` is called. They must be idempotent.
  Most package managers handle this natively (e.g., `brew install` is a no-op if
  already installed).
- Failed installs are **non-fatal** — facet logs the failure and continues with other
  packages and config steps.
- Per-OS entries: if the current OS has no matching key, the package is skipped with
  a warning.
- Install commands can contain `${facet:...}` variables. They are resolved before
  execution.

## Rules

- facet is **additive only** — it never uninstalls packages.
- No version field. Put the version in your install command if needed:
  `install: npm install -g typescript@5.0`
- No automatic package manager detection. You provide the full install command.
- Packages from base and profile are **unioned by name**. If both define the same
  package name, the later layer's install command wins. See `facet docs merge`.
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/packages.md && git commit -m "docs: write packages topic for facet docs"
```

---

### Task 7: Write `deploy.md` Topic

**Files:**
- Modify: `internal/docs/topics/deploy.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/deploy.md` with:

```markdown
# Config File Deployment

## The configs block

Maps target paths (where the file goes) to source paths (where it lives in your repo):

```yaml
configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.config/nvim: configs/nvim
  ~/.zshrc: configs/.zshrc
```

## Target path expansion

Target paths support environment variable expansion:

- `~` expands to `$HOME`
- `$VAR_NAME` or `${VAR_NAME}` expands from OS environment variables

This is **environment variable expansion**, not `${facet:...}` substitution. The two
systems are separate. After expansion, the target path must be absolute.

## Source path constraints

Source paths are relative to the config directory. Paths that escape the config
directory (via `../` or absolute paths) are rejected with a fatal error.

Source paths can contain `${facet:...}` variables — they are resolved before use.

## Deploy strategy (auto-detected)

| Source type | Contains `${facet:` | Strategy |
|-------------|---------------------|----------|
| File | No | Symlink: target → source |
| File | Yes | Template: substitute variables, write rendered file |
| Directory | N/A | Symlink: target → source |

You never specify the strategy — facet detects it by scanning file content.

## Symlink behavior

1. Symlink exists, points to correct source → no-op
2. Symlink exists, wrong source → removed and recreated
3. Regular file exists → prompt to replace (with `--force`: replace without asking)
4. Target doesn't exist → create symlink, parent directories created automatically

## Template behavior

Templated files are rewritten on every `facet apply` (variables may have changed).

## Profile switching

When switching from profile A to profile B, configs in A but not B are removed
(symlinks deleted, templated files deleted).
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/deploy.md && git commit -m "docs: write deploy topic for facet docs"
```

---

### Task 8: Write `ai.md` Topic

**Files:**
- Modify: `internal/docs/topics/ai.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/ai.md` with:

```markdown
# AI Agent Configuration

## Overview

The `ai:` block configures AI coding agents. facet writes each agent's native config
files on `facet apply`.

## Supported agents

Valid agent names: `claude-code`, `cursor`, `codex`

```yaml
ai:
  agents:
    - claude-code
    - cursor
    - codex
```

Agents not installed on the machine are skipped with a warning.

## Permissions

Allow/deny lists applied to AI agents:

```yaml
ai:
  agents:
    - claude-code
  permissions:
    allow:
      - Bash(npm test)
      - Bash(npm run build)
      - Read(~/code)
    deny:
      - Bash(rm -rf *)
      - Bash(sudo *)
```

Permission format: `Type(pattern)` where Type is `Bash`, `Read`, or `Write`.
Patterns can use `*` as a wildcard.

## Skills

Install skill packages for agents:

```yaml
ai:
  agents:
    - claude-code
  skills:
    - source: "@anthropic/claude-code-skills"
      skills:
        - code-review
        - testing
    - source: "@my-org/custom-skills"
      skills:
        - deploy-helper
      agents:
        - claude-code          # only install for claude-code
```

Each skill entry has:
- `source` (required): NPM package or local path containing the skills
- `skills` (required): list of skill names to install from that source
- `agents` (optional): restrict to specific agents. If omitted, installs for all
  agents in the `ai.agents` list.

## MCP servers

Configure Model Context Protocol servers:

```yaml
ai:
  agents:
    - claude-code
    - cursor
  mcps:
    - name: filesystem
      command: npx
      args: ["-y", "@anthropic/mcp-filesystem"]
      env:
        ALLOWED_DIRS: ~/code
    - name: postgres
      command: npx
      args: ["-y", "@anthropic/mcp-postgres"]
      env:
        DATABASE_URL: ${facet:db_url}
      agents:
        - claude-code          # only configure for claude-code
```

Each MCP entry has:
- `name` (required): identifier for the server
- `command` (required): command to run the server
- `args` (optional): command arguments
- `env` (optional): environment variables. Values support `${facet:...}` substitution.
- `agents` (optional): restrict to specific agents. If omitted, configures for all
  agents in the `ai.agents` list.

## Per-agent overrides

Override permissions for specific agents:

```yaml
ai:
  agents:
    - claude-code
    - cursor
  permissions:
    allow:
      - Bash(npm test)
  overrides:
    claude-code:
      permissions:
        allow:
          - Bash(npm test)
          - Bash(npm run deploy)
```

When an override is present for an agent, its `permissions` block **replaces** the
top-level permissions for that agent (it does not merge).

## Complete example

```yaml
ai:
  agents:
    - claude-code
    - cursor
  permissions:
    allow:
      - Bash(npm test)
      - Bash(npm run build)
      - Read(~/code)
    deny:
      - Bash(rm -rf *)
      - Bash(sudo *)
  skills:
    - source: "@anthropic/claude-code-skills"
      skills:
        - code-review
  mcps:
    - name: filesystem
      command: npx
      args: ["-y", "@anthropic/mcp-filesystem"]
      env:
        ALLOWED_DIRS: ~/code
  overrides:
    claude-code:
      permissions:
        allow:
          - Bash(npm test)
          - Bash(npm run deploy)
          - Read(~/code)
```
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/ai.md && git commit -m "docs: write AI configuration topic for facet docs"
```

---

### Task 9: Write `merge.md` Topic

**Files:**
- Modify: `internal/docs/topics/merge.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/merge.md` with:

```markdown
# Merge Rules

## Layer order

Three layers merge in order. Later layers win on conflicts.

```
1. base.yaml            — shared foundation
2. profiles/<name>.yaml — per-machine overrides
3. .local.yaml          — machine-specific secrets
```

## vars — deep merge, last writer wins per leaf

```yaml
# base.yaml
vars:
  git:
    name: Sarah
    editor: nvim

# profile
vars:
  git:
    email: sarah@work.com

# result
vars:
  git:
    name: Sarah        # from base
    editor: nvim       # from base
    email: sarah@work.com  # from profile
```

**Type conflict is fatal.** If base defines `git` as a map and a profile defines `git`
as a string (or vice versa), facet exits with an error.

## packages — union by name, last writer wins

```yaml
# base.yaml
packages:
  - name: git
    install: brew install git

# profile (same name, different command)
packages:
  - name: git
    install: sudo apt-get install -y git

# result: profile's install command wins for "git"
```

Packages from all layers are unioned. If the same package name appears in multiple
layers, the later layer's entry replaces the earlier one entirely.

## configs — shallow merge, same target = later wins

```yaml
# base.yaml
configs:
  ~/.gitconfig: configs/.gitconfig

# profile
configs:
  ~/.gitconfig: configs/work/.gitconfig   # overrides base
  ~/.npmrc: configs/work/.npmrc           # new entry

# result
configs:
  ~/.gitconfig: configs/work/.gitconfig   # profile won
  ~/.npmrc: configs/work/.npmrc           # added by profile
```

## ai.agents — last writer wins

If a later layer defines `ai.agents`, it replaces the list entirely.

## ai.permissions — last writer wins

If a later layer defines `ai.permissions`, its permissions block replaces the earlier
one entirely.

## ai.skills — union by (source, skill name)

Skills from all layers are unioned by the combination of source and skill name. If
the same (source, skill) pair appears in multiple layers, the later layer's entry
wins (including its agent scoping).

## ai.mcps — union by name

MCP entries from all layers are unioned by name. Same name = later layer replaces
the entire entry.

## ai.overrides — deep merge by agent name

Override entries merge by agent name. For the same agent, the later layer's override
replaces the earlier one's permissions.
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/merge.md && git commit -m "docs: write merge rules topic for facet docs"
```

---

### Task 10: Write `commands.md` Topic

**Files:**
- Modify: `internal/docs/topics/commands.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/commands.md` with:

```markdown
# Commands

## Global flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config-dir` | `-c` | Current directory | Path to facet config repo |
| `--state-dir` | `-s` | `~/.facet/` | Path to machine-local state directory |

## facet init

Create a new config repo in the current directory and initialize the state directory.

```bash
facet init
```

Creates: `facet.yaml`, `base.yaml`, `profiles/`, `configs/` in the current directory.
Creates `.local.yaml` in the state directory if it doesn't exist.

Fails if `facet.yaml` already exists.

## facet apply \<profile\>

Apply a configuration profile.

```bash
facet apply work
facet apply work --dry-run
facet apply work --force
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview what would happen without making changes |
| `--force` | Unapply current state + apply, skip prompts for conflicting files |
| `--skip-failure` | Warn on config deploy failure instead of rolling back |

**What it does:**
1. Loads facet.yaml, base.yaml, profile, and .local.yaml
2. Merges the three layers
3. Resolves `${facet:...}` variables
4. Installs packages
5. Deploys config files (symlink or template)
6. Configures AI tools
7. Writes `.state.json`

## facet status

Show the current applied state.

```bash
facet status
```

Reads `.state.json` and displays: active profile, packages, configs, and validity
checks (symlinks still valid, files still exist).

## facet docs [topic]

Show documentation.

```bash
facet docs              # overview + topic list
facet docs config       # specific topic
```

See `facet docs` for the full topic list.

## Exit codes

- **0**: success
- **1**: fatal error (bad YAML, missing profile, undefined variable, missing .local.yaml)
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/commands.md && git commit -m "docs: write commands topic for facet docs"
```

---

### Task 11: Write `examples.md` Topic

**Files:**
- Modify: `internal/docs/topics/examples.md`

- [ ] **Step 1: Write the content**

Replace the placeholder in `internal/docs/topics/examples.md` with:

```markdown
# Examples

A complete working configuration with packages, config files, variables, and AI setup.

## base.yaml

```yaml
vars:
  git_name: Sarah Chen

packages:
  - name: git
    install: brew install git
  - name: ripgrep
    install: brew install ripgrep
  - name: fd
    install:
      macos: brew install fd
      linux: sudo apt-get install -y fd-find

configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.zshrc: configs/.zshrc

ai:
  agents:
    - claude-code
    - cursor
  permissions:
    allow:
      - Bash(npm test)
      - Bash(npm run build)
    deny:
      - Bash(rm -rf *)
      - Bash(sudo *)
  mcps:
    - name: filesystem
      command: npx
      args: ["-y", "@anthropic/mcp-filesystem"]
      env:
        ALLOWED_DIRS: ~/code
```

## profiles/work.yaml

```yaml
extends: base

vars:
  git_email: sarah@acme.com

packages:
  - name: docker
    install: brew install docker
  - name: awscli
    install: brew install awscli

configs:
  ~/.gitconfig: configs/work/.gitconfig
  ~/.npmrc: configs/work/.npmrc

ai:
  mcps:
    - name: postgres
      command: npx
      args: ["-y", "@anthropic/mcp-postgres"]
      env:
        DATABASE_URL: ${facet:db_url}
      agents:
        - claude-code
  skills:
    - source: "@acme/dev-skills"
      skills:
        - deploy-helper
        - code-review
```

## ~/.facet/.local.yaml

```yaml
vars:
  db_url: postgres://sarah:secret@localhost:5432/acme
  github_token: ghp_xxxxxxxxxxxx
```

## What happens on `facet apply work`

**Merged vars:**
- `git_name`: Sarah Chen (from base)
- `git_email`: sarah@acme.com (from profile)
- `db_url`: postgres://sarah:secret@localhost:5432/acme (from .local)
- `github_token`: ghp_xxxxxxxxxxxx (from .local)

**Packages installed** (5): git, ripgrep, fd, docker, awscli

**Configs deployed:**
- `~/.gitconfig` → configs/work/.gitconfig (profile overrode base)
- `~/.zshrc` → configs/.zshrc (from base)
- `~/.npmrc` → configs/work/.npmrc (added by profile)

**AI configured for:** claude-code, cursor
- Permissions: 2 allow, 2 deny (applied to both agents)
- MCPs: filesystem (both agents), postgres (claude-code only)
- Skills: deploy-helper + code-review from @acme/dev-skills (both agents)
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v -run TestRender_AllRegisteredTopicsHaveFiles`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/docs/topics/examples.md && git commit -m "docs: write examples topic for facet docs"
```

---

### Task 12: Final Verification

**Files:** None (read-only verification)

- [ ] **Step 1: Run the full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... 2>&1 | tail -30`
Expected: ALL PASS

- [ ] **Step 2: Build and test all topics via CLI**

Run:
```bash
cd /Users/edocsss/aec/src/facet && go build -o /tmp/facet-test .
for topic in quickstart config variables packages deploy ai merge commands examples; do
  echo "=== $topic ===" && /tmp/facet-test docs $topic | head -3 && echo
done
/tmp/facet-test docs
/tmp/facet-test docs nonexistent 2>&1; echo "exit: $?"
rm /tmp/facet-test
```

Expected: Each topic prints its heading. Overview prints the topic list. Invalid topic prints an error with exit code 1.

- [ ] **Step 3: Commit if any cleanup was needed, otherwise done**
