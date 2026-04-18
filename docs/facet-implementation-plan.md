# facet — Implementation Plan

This is a step-by-step implementation plan for building the `facet` CLI tool. Each phase is self-contained, testable, and builds on the previous one. Written to be used as a spec for Claude Code.

---

## Tech Stack

- **Language:** Go 1.22+
- **YAML parsing:** `gopkg.in/yaml.v3`
- **CLI framework:** `cobra` (github.com/spf13/cobra)
- **Template engine:** Go's `text/template` (only `{{.var_name}}` syntax exposed)
- **Testing:** Go standard `testing` package + Docker for E2E
- **Build:** Single static binary via `go build`
- **E2E:** Docker containers (ephemeral, one per test run)

---

## Project Structure

```
facet/
├── cmd/
│   ├── root.go              # cobra root command
│   ├── apply.go             # facet apply [profile]
│   ├── status.go            # facet status
│   ├── diff.go              # facet diff [profile]
│   ├── init_cmd.go          # facet init [--from repo]
│   └── doctor.go            # facet doctor
├── internal/
│   ├── config/
│   │   ├── types.go         # YAML struct definitions
│   │   ├── loader.go        # Load and parse YAML files
│   │   ├── merger.go        # Merge base + profile + .local
│   │   └── resolver.go      # Resolve {{vars}} in merged config
│   ├── packages/
│   │   ├── manager.go       # Package manager interface
│   │   ├── brew.go          # Homebrew backend
│   │   ├── apt.go           # apt backend
│   │   └── detect.go        # OS/package manager detection
│   ├── configs/
│   │   ├── deployer.go      # Symlink or template+copy logic
│   │   └── dotd.go          # .facet.d/ directory management
│   ├── ai/
│   │   ├── adapter.go       # AI tool adapter interface
│   │   ├── claude.go        # Claude Code adapter
│   │   ├── cursor.go        # Cursor adapter
│   │   ├── codex.go         # Codex adapter
│   │   └── permissions.go   # Permission merging logic
│   └── reporter/
│       └── reporter.go      # Terminal output formatting
├── e2e/
│   ├── Dockerfile.ubuntu     # Ubuntu 24.04 test image
│   ├── Dockerfile.macos      # macOS cross-compile + verify image
│   ├── e2e_test.go           # Go test file that orchestrates Docker
│   ├── run.sh                # Standalone runner (no Go required)
│   ├── fixtures/
│   │   ├── setup-sarah.sh    # Populates Sarah's real-world config
│   │   └── mock-tools.sh     # Fake claude/cursor/codex binaries
│   └── suites/
│       ├── 01-init.sh        # Test: facet init
│       ├── 02-apply-basic.sh # Test: apply simple profile
│       ├── 03-configs.sh     # Test: symlink, template, .facet.d
│       ├── 04-ai-tools.sh    # Test: MCP + permissions output
│       ├── 05-profile-switch.sh  # Test: switch profiles
│       ├── 06-status-diff.sh # Test: status and diff commands
│       ├── 07-doctor.sh      # Test: doctor checks
│       ├── 08-idempotent.sh  # Test: apply twice = same result
│       ├── 09-edge-cases.sh  # Test: missing vars, bad YAML, etc.
│       └── 10-packages.sh    # Test: real apt install inside container
├── testdata/
│   ├── basic/               # Test fixture: simple base + profile
│   ├── realworld/           # Test fixture: Sarah's full setup
│   └── edge/                # Test fixture: edge cases
├── Makefile
├── go.mod
├── go.sum
└── main.go
```

---

## Data Types

These are the core YAML structs. Define these first — everything else depends on them.

```go
// internal/config/types.go

// FacetConfig represents a parsed base.yaml, profile, or .local.yaml
type FacetConfig struct {
    Extends     string            `yaml:"extends,omitempty"`
    Vars        map[string]string `yaml:"vars,omitempty"`
    Packages    []PackageEntry    `yaml:"packages,omitempty"`
    PackagesGUI []PackageEntry    `yaml:"packages_gui,omitempty"`
    Configs     map[string]string `yaml:"configs,omitempty"`
    AI          AIConfig          `yaml:"ai,omitempty"`
}

// PackageEntry can be a simple string "ripgrep" or an object with name + install
type PackageEntry struct {
    Name    string            `yaml:"name,omitempty"`
    Version string            `yaml:"version,omitempty"`
    Install interface{}       `yaml:"install,omitempty"` // string or map[string]string for per-OS
}

// AIConfig holds MCP servers, permissions, and per-tool settings
type AIConfig struct {
    MCP         map[string]MCPServer `yaml:"mcp,omitempty"`
    Permissions Permissions          `yaml:"permissions,omitempty"`
    Claude      map[string]any       `yaml:"claude,omitempty"`
    Cursor      map[string]any       `yaml:"cursor,omitempty"`
    Codex       map[string]any       `yaml:"codex,omitempty"`
    Copilot     map[string]any       `yaml:"copilot,omitempty"`
}

// MCPServer is a single MCP server definition
type MCPServer struct {
    Command string            `yaml:"command,omitempty"`
    Args    []string          `yaml:"args,omitempty"`
    URL     string            `yaml:"url,omitempty"`
    Env     map[string]string `yaml:"env,omitempty"`
}

// Permissions holds composable allow/deny lists
type Permissions struct {
    Allow []string `yaml:"allow,omitempty"`
    Deny  []string `yaml:"deny,omitempty"`
}

// ResolvedConfig is the fully merged, variable-resolved config ready to apply
type ResolvedConfig struct {
    Vars        map[string]string
    Packages    []PackageEntry
    PackagesGUI []PackageEntry
    Configs     map[string]string    // target path -> source path (resolved)
    AI          AIConfig
}
```

---

## Phase 1: Config Loading & Merging

**Goal:** Parse YAML files, merge base + profile + .local, resolve vars.
**No side effects.** This phase only reads files and produces a ResolvedConfig.

### 1.1 — YAML Loader

File: `internal/config/loader.go`

- `LoadConfig(path string) (*FacetConfig, error)` — parse a single YAML file
- Handle the `PackageEntry` union type (string or object) with a custom `UnmarshalYAML`
- Return clear errors: "failed to parse base.yaml: line 12: ..."

Test with:
```
testdata/basic/base.yaml
testdata/basic/profiles/work.yaml
testdata/basic/.local.yaml
```

### 1.2 — Config Merger

File: `internal/config/merger.go`

- `Merge(base, profile, local *FacetConfig) *FacetConfig`
- Merge rules:
  - `vars`: shallow merge, later layers overwrite same keys
  - `packages` / `packages_gui`: union by package name, later layer wins on conflict (same name, different version)
  - `configs`: shallow merge of target->source map, later layer overwrites same target key
  - `ai.mcp`: union by server name, later layer overwrites same name
  - `ai.permissions.allow`: union (deduplicated)
  - `ai.permissions.deny`: union (deduplicated), and track which layer each deny came from — denies from earlier layers cannot be removed
  - `ai.claude`, `ai.cursor`, etc.: shallow merge per tool, later layer overwrites same keys

Test: merge the basic fixture and assert the resolved config has the expected packages, configs, permissions, etc.

### 1.3 — Variable Resolver

File: `internal/config/resolver.go`

- `Resolve(config *FacetConfig) (*ResolvedConfig, error)`
- Walk all string values in the merged config
- Replace `{{var_name}}` with the value from `config.Vars`
- If a var is referenced but not defined, return an error: `unresolved variable: {{acme_db_url}} — define it in .local.yaml or your profile's vars`
- Do NOT resolve vars inside the `configs` source paths (those are file paths, not templates — the template resolution happens when deploying the file in Phase 2)

Test: create a config with `{{git_email}}` in an MCP env value, resolve it, check the output.

### 1.4 — Profile Resolution Entry Point

File: `internal/config/loader.go` (extend)

- `LoadResolved(facetDir string, profileName string) (*ResolvedConfig, error)`
- Steps:
  1. Load `base.yaml`
  2. Load `profiles/{profileName}.yaml`
  3. Verify `extends: base` (only value allowed for now)
  4. Load `.local.yaml` (if exists, optional)
  5. Merge all three
  6. Resolve vars
  7. Return ResolvedConfig

### Phase 1 Tests

```
TestLoadConfig_BasicYAML
TestLoadConfig_PackageEntry_StringAndObject
TestMerge_PackagesUnion
TestMerge_ConfigsOverride
TestMerge_MCPUnion
TestMerge_PermissionsAllow_Union
TestMerge_PermissionsDeny_Immutable
TestResolve_VarsSubstituted
TestResolve_UndefinedVar_Error
TestLoadResolved_FullPipeline
```

### Phase 1 Done When

- `go test ./internal/config/...` passes
- Can load the Sarah real-world fixture and get a correct ResolvedConfig for each of her 3 profiles

---

## Phase 2: Config Deployment

**Goal:** Take a ResolvedConfig and deploy dotfiles to the filesystem.

### 2.1 — File Deployer

File: `internal/configs/deployer.go`

- `Deploy(configs map[string]string, vars map[string]string, facetDir string, homeDir string) ([]DeployResult, error)`
- For each target -> source pair:
  1. Resolve source path relative to facetDir
  2. Read the source file
  3. If source contains `{{` — render with vars using `text/template`, write to `homeDir/target`
  4. If source does NOT contain `{{` — create symlink from `homeDir/target` to source
  5. If source is a directory — always symlink
  6. Create parent directories as needed (`mkdir -p`)
  7. If target already exists and is not managed by facet — warn but don't overwrite (unless it's already a symlink pointing to our configs dir)
- Return a list of `DeployResult{Target, Strategy (symlink|template), Status (created|updated|skipped)}`

### 2.2 — .facet.d/ Manager

File: `internal/configs/dotd.go`

- `RebuildDotD(configs map[string]string, facetDir string, homeDir string) ([]DeployResult, error)`
- Steps:
  1. Identify all config entries where target starts with `.facet.d/`
  2. Remove all existing files in `homeDir/.facet.d/` (clean rebuild)
  3. Create `homeDir/.facet.d/` if it doesn't exist
  4. Symlink each snippet from the configs map
  5. Sort entries by target name (so 01-xxx loads before 10-yyy)
- This is called by the main Deploy function, which separates .facet.d/ entries from regular configs

### 2.3 — Template Engine Wrapper

File: `internal/configs/deployer.go` (private function)

- `renderTemplate(content string, vars map[string]string) (string, error)`
- Use `text/template` but with custom delimiters: `{{` and `}}`
- Expose vars as `.var_name` — e.g., `{{.git_email}}`
- On missing var during render, return error with the var name and file path
- No logic, no loops, no conditionals — just variable substitution. If someone tries `{{if ...}}`, that's fine (Go templates support it), but we don't document or encourage it.

### Phase 2 Tests

```
TestDeploy_Symlink_NoVars
TestDeploy_Template_WithVars
TestDeploy_Directory_AlwaysSymlink
TestDeploy_CreateParentDirs
TestDeploy_ExistingFile_Warn
TestRebuildDotD_CleanAndRepopulate
TestRebuildDotD_CorrectOrder
TestRenderTemplate_BasicVars
TestRenderTemplate_MissingVar_Error
```

### Phase 2 Done When

- Can deploy the basic fixture to a temp directory
- `.facet.d/` is correctly rebuilt with numbered snippets in order
- Template files render correctly, static files are symlinked

---

## Phase 3: Package Installation

**Goal:** Detect OS, map package names, install missing packages.

### 3.1 — OS Detection

File: `internal/packages/detect.go`

- `DetectOS() OSInfo` — returns `{OS: "macos"|"linux", Distro: "ubuntu"|"arch"|"fedora"|"", Arch: "arm64"|"amd64"}`
- Detection method: `runtime.GOOS` + read `/etc/os-release` on Linux
- `DetectPackageManager(os OSInfo) PackageManager` — returns the right backend

### 3.2 — Package Manager Interface

File: `internal/packages/manager.go`

```go
type PackageManager interface {
    Name() string
    IsInstalled(pkg string) (bool, error)
    Install(pkg PackageEntry) error
    InstallGUI(pkg PackageEntry) error
    MapName(pkg string) string  // canonical name -> OS-specific name
}
```

### 3.3 — Homebrew Backend (MVP)

File: `internal/packages/brew.go`

- `IsInstalled`: run `brew list --formula | grep -q {name}` (and `brew list --cask` for GUI)
- `Install`: run `brew install {name}`
- `InstallGUI`: run `brew install --cask {name}`
- `MapName`: lookup table for known differences (e.g., `fd` -> `fd` on brew, `fd-find` on apt)
- Handle `PackageEntry` with custom install commands — if `Install` field is set, run that instead
- Handle versioned packages like `node@22`

### 3.4 — apt Backend

File: `internal/packages/apt.go`

- Same interface as brew but using `dpkg -l | grep`, `sudo apt install -y`
- Name mapping table for known packages

### 3.5 — Install Orchestrator

File: `internal/packages/manager.go` (extend)

- `InstallAll(packages []PackageEntry, gui []PackageEntry, pm PackageManager) ([]InstallResult, error)`
- For each package: check if installed, skip if yes, install if no
- Collect results: `{Name, Status: installed|skipped|failed, Error}`
- Don't fail on first error — try all, report all

### Phase 3 Tests

```
TestDetectOS
TestBrewIsInstalled (mock exec)
TestBrewInstall (mock exec)
TestBrewMapName
TestInstallAll_SkipsExisting
TestInstallAll_CustomInstallCommand
TestInstallAll_PerOSInstallCommand
```

### Phase 3 Done When

- On macOS, `facet apply` installs missing Homebrew packages
- Already-installed packages are skipped
- Custom install commands (like `npm install -g ...`) work
- Failed installs don't crash — they're reported

---

## Phase 4: AI Tool Adapters

**Goal:** Write MCP servers, permissions, and settings to each AI tool's native config format.

### 4.1 — Adapter Interface

File: `internal/ai/adapter.go`

```go
type Adapter interface {
    Name() string
    IsInstalled() bool
    Apply(mcp map[string]MCPServer, perms Permissions, settings map[string]any) error
}

func ApplyAll(ai AIConfig) []AdapterResult {
    adapters := []Adapter{
        &ClaudeAdapter{},
        &CursorAdapter{},
        &CodexAdapter{},
    }
    var results []AdapterResult
    for _, a := range adapters {
        if !a.IsInstalled() {
            results = append(results, AdapterResult{Name: a.Name(), Skipped: true})
            continue
        }
        settings := getToolSettings(ai, a.Name())
        err := a.Apply(ai.MCP, ai.Permissions, settings)
        results = append(results, AdapterResult{Name: a.Name(), Error: err})
    }
    return results
}
```

### 4.2 — Claude Code Adapter

File: `internal/ai/claude.go`

Writes to two files:

**~/.claude.json** — MCP servers:
```json
{
  "mcpServers": {
    "<n>": {
      "type": "stdio",
      "command": "<command>",
      "args": ["<args>"],
      "env": {"<key>": "<value>"}
    }
  }
}
```
- If server has `url` instead of `command`, use `"type": "url"` and `"url"` field

**~/.claude/settings.json** — permissions + settings:
```json
{
  "permissions": {
    "allow": ["..."],
    "deny": ["..."]
  }
}
```
- Merge in any extra keys from `ai.claude` (like `model`)

Detection: `IsInstalled()` checks if `claude` binary is in PATH.

Read existing files before writing. Preserve keys that facet doesn't manage (don't clobber user's manual additions). Use a `"_managed_by": "facet"` marker or a facet-specific section to track what we own.

Strategy for not clobbering:
- Read existing file if present
- For MCP servers: replace all entries (facet owns MCP config entirely when active)
- For settings: merge facet-managed keys, preserve unknown keys
- Write back with consistent JSON formatting (indented, sorted keys)

### 4.3 — Cursor Adapter

File: `internal/ai/cursor.go`

Writes to:

**~/.cursor/mcp.json** — MCP servers:
```json
{
  "mcpServers": {
    "<n>": {
      "command": "<command>",
      "args": ["<args>"],
      "env": {"<key>": "<value>"}
    }
  }
}
```

If `ai.cursor.rules` is set (inline string), write to **~/.cursor/rules/facet.mdc** or the appropriate rules location.

If `ai.cursor.rules_file` is set (path to a file), copy that file to the Cursor rules directory.

Detection: check for `~/.cursor/` directory or `cursor` in PATH.

### 4.4 — Codex Adapter

File: `internal/ai/codex.go`

Writes to **~/.codex/config.toml**:
```toml
model = "o3"
approval_mode = "suggest"

[mcp_servers.<n>]
command = "<command>"
args = ["<args>"]
```

Use a TOML library: `github.com/pelletier/go-toml/v2`

Detection: check for `codex` binary in PATH.

### 4.5 — Permission Merger

File: `internal/ai/permissions.go`

- `MergePermissions(layers ...Permissions) Permissions`
- Allow lists: union, deduplicate
- Deny lists: union, deduplicate
- Validation: if any allow entry matches a deny pattern, emit a warning (but don't block — the deny will take precedence at the tool level)

### Phase 4 Tests

```
TestClaudeAdapter_WriteMCP
TestClaudeAdapter_WritePermissions
TestClaudeAdapter_PreservesUnmanagedKeys
TestClaudeAdapter_NotInstalled_Skip
TestCursorAdapter_WriteMCP
TestCursorAdapter_WriteRules
TestCodexAdapter_WriteTOML
TestMergePermissions_AllowUnion
TestMergePermissions_DenyImmutable
```

### Phase 4 Done When

- Claude Code, Cursor, and Codex configs are written correctly
- Uninstalled tools are skipped with a warning
- Existing non-facet config keys are preserved

---

## Phase 5: CLI Commands

**Goal:** Wire everything together with cobra commands.

### 5.1 — `facet apply [profile]`

File: `cmd/apply.go`

Steps:
1. Find facet directory (`~/.facet/` or `FACET_DIR` env var)
2. Load resolved config for the given profile
3. Install packages (with progress output)
4. Deploy configs + rebuild .facet.d/
5. Apply AI tool configs
6. Write state file (`~/.facet/.state.json`) recording what was applied:
   ```json
   {
     "profile": "acme",
     "applied_at": "2025-03-14T10:30:00Z",
     "packages_installed": ["gh"],
     "configs_deployed": [".gitconfig", ".zshrc", ".facet.d/10-acme-env.sh"],
     "ai_tools": ["claude", "codex"]
   }
   ```
7. Print report using reporter

### 5.2 — `facet status`

File: `cmd/status.go`

- Read `.state.json`
- Show: active profile, when last applied, package count, config count, AI tools configured
- Quick checks: are all expected packages still installed? Do all symlinks still point to the right place?

### 5.3 — `facet diff [profile]`

File: `cmd/diff.go`

- Load resolved config for the given profile (or current profile if not specified)
- Compare against current machine state:
  - Packages: what would be installed/removed
  - Configs: what would change
  - AI tools: what would change in MCP/permissions/settings
- Output as a human-readable diff

### 5.4 — `facet init [--from repo]`

File: `cmd/init_cmd.go`

- Without `--from`: create `~/.facet/` with a starter `base.yaml` and empty `profiles/` and `configs/` directories, plus a `.gitignore` (ignoring `.local.yaml` and `.state.json`), then `git init`
- With `--from`: `git clone <repo> ~/.facet/`

### 5.5 — `facet doctor`

File: `cmd/doctor.go`

Check and report:
- Is `~/.facet/` a git repo?
- Is `.local.yaml` gitignored?
- Are all referenced config source files present?
- Are all {{vars}} resolvable?
- Is the active profile's package manager available?
- Are AI tools installed for tools referenced in config?
- Are there broken symlinks in deployed configs?

### 5.6 — Reporter

File: `internal/reporter/reporter.go`

Consistent terminal output:
```
  ✓ 9 packages up to date
  ↓ Installing: gh (1 new)
  ✓ 4 configs deployed (2 symlinked, 2 templated)
  ✓ ~/.facet.d/: 4 shell snippets
  ✓ claude: 2 MCP servers, 6 allow / 3 deny
  ✓ cursor: 2 MCP servers, rules written
  ⚠ codex: not installed — skipping
```

Use colors if terminal supports it (check `os.Getenv("TERM")`). Fallback to plain text.

### Phase 5 Tests

```
TestApply_FullPipeline (integration test with temp dirs)
TestStatus_ReadsState
TestDiff_ShowsChanges
TestInit_CreatesStructure
TestInit_FromRepo (mock git clone)
TestDoctor_AllChecks
```

### Phase 5 Done When

- All 5 commands work end-to-end
- `facet apply` on a real macOS machine installs packages, deploys configs, writes AI tool configs
- `facet status` accurately reflects the current state

---

## Phase 6: Test Fixtures

Create realistic test data used across all phases.

### testdata/basic/

A minimal setup for unit tests:

```
testdata/basic/
├── base.yaml
├── profiles/
│   └── work.yaml
├── configs/
│   ├── .gitconfig          # has {{.git_email}}
│   ├── .zshrc              # static
│   └── shell/
│       ├── base-aliases.sh
│       └── work-env.sh
└── .local.yaml
```

### testdata/realworld/

Sarah's full setup from the design doc:

```
testdata/realworld/
├── base.yaml
├── profiles/
│   ├── personal.yaml
│   ├── acme.yaml
│   └── acme-server.yaml
├── configs/
│   ├── .zshrc
│   ├── .gitconfig
│   ├── starship.toml
│   ├── nvim/
│   │   └── init.lua
│   ├── shell/
│   │   ├── base-path.sh
│   │   ├── base-aliases.sh
│   │   ├── acme-env.sh
│   │   ├── acme-aliases.sh
│   │   ├── acme-aws.sh
│   │   ├── personal-env.sh
│   │   └── personal-aliases.sh
│   ├── acme/
│   │   ├── .gitconfig
│   │   ├── .npmrc
│   │   └── .aws/
│   │       └── config
│   └── ai/
│       ├── acme-review.md
│       └── acme-cursor-rules.md
└── .local.yaml
```

### testdata/edge/

Edge cases:

- Profile with no `extends` (should error)
- Profile extending non-existent base (should error)
- Config file with `{{undefined_var}}` (should error)
- Empty profile (just `extends: base`, inherits everything)
- Package with per-OS install commands
- MCP server with URL instead of command
- .local.yaml that doesn't exist (should be fine)

---

## Phase 7: Hermetic E2E Testing (Docker)

**Goal:** Fully automated end-to-end tests that run in disposable Docker containers. No manual steps. Fresh filesystem every run. Tests the real binary against a real package manager.

### 7.1 — Design Principles

- **Hermetic via HOME override** — the harness sets `HOME` to a temp directory before running any suite. All file operations (`~/.gitconfig`, `~/.claude.json`, etc.) go to the sandbox. Your real config files are never read or written.
- **One sandbox per suite** — each suite gets its own `$HOME` so suites can't leak state to each other
- **Cleanup on exit** — `trap cleanup EXIT` deletes the sandbox even on failure or Ctrl+C
- **Same suites, both platforms** — identical shell scripts run in Docker (Linux) and natively (macOS). No separate test code per OS.
- **Real packages in Docker, mocked locally** — Docker runs with `FACET_E2E_REAL_PACKAGES=1` (real `apt install`). Native runs use mock `brew`/`apt-get` that log to a file instead of installing.
- **Mock AI tools** — fake `claude`, `cursor`, `codex` binaries that satisfy `IsInstalled()` checks without needing real tools
- **Exit code driven** — each suite exits 0 on pass, non-zero on fail. The harness collects results.
- **Runs from `make e2e` (Docker) or `make e2e-local` (native)** — one command, zero risk to your machine

### 7.2 — Dockerfile

File: `e2e/Dockerfile.ubuntu`

```dockerfile
FROM ubuntu:24.04

# Avoid interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Base system — the minimum a real dev server would have
RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    sudo \
    zsh \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root test user (facet shouldn't need root for configs)
RUN useradd -m -s /bin/zsh testuser \
    && echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Working directories
RUN mkdir -p /opt/facet /opt/e2e

# Copy the facet binary (built on host, for linux)
COPY facet-linux /usr/local/bin/facet
RUN chmod +x /usr/local/bin/facet

# Copy test fixtures, suites, and harness
COPY fixtures/ /opt/e2e/fixtures/
COPY suites/ /opt/e2e/suites/
COPY harness.sh /opt/e2e/harness.sh
RUN chmod +x /opt/e2e/harness.sh /opt/e2e/suites/*.sh /opt/e2e/fixtures/*.sh

# Tell the harness where to find things
ENV SUITE_DIR=/opt/e2e/suites
ENV FIXTURE_DIR=/opt/e2e/fixtures

USER testuser
WORKDIR /home/testuser

ENTRYPOINT ["/opt/e2e/harness.sh"]
```

### 7.3 — Mock AI Tools + Mock Package Manager

File: `e2e/fixtures/mock-tools.sh`

Creates fake binaries for AI tools AND a mock package manager.

**How the mock package manager works:**

Facet calls package managers by name (`brew`, `apt-get`), not by absolute path. The harness puts `$HOME/mock-bin` first in `PATH`. So when facet's Go code runs `exec.Command("brew", "install", "tree")`, the OS resolves `brew` to our fake script at `$HOME/mock-bin/brew` before finding the real `/opt/homebrew/bin/brew`.

The mock handles three operations that facet uses:
1. **`brew install X`** → appends "X" to `$HOME/.mock-packages` (a simple text log)
2. **`brew list`** → reads `$HOME/.mock-packages` (so facet's "is installed?" check works)
3. **`which tree`** → we pre-create stub binaries in `$HOME/mock-bin/` for packages in the test fixture

**Implementation constraint:** facet must call package managers by name, not absolute path. This is the right design anyway — the path differs between Intel macOS (`/usr/local/bin/brew`), Apple Silicon (`/opt/homebrew/bin/brew`), and Linux distros.

On Docker with `FACET_E2E_REAL_PACKAGES=1`, the mock is skipped and real `apt` is used.

```bash
#!/bin/bash
set -euo pipefail

# All paths relative to $HOME (which the harness sets to the sandbox)
mkdir -p "$HOME/mock-bin"

# ── Mock AI tools ──
# Adapters check: is binary in PATH? Does config dir exist?

mkdir -p "$HOME/.claude"
cat > "$HOME/mock-bin/claude" << 'EOF'
#!/bin/bash
echo "mock claude 1.0.0"
EOF
chmod +x "$HOME/mock-bin/claude"

mkdir -p "$HOME/.cursor"
cat > "$HOME/mock-bin/cursor" << 'EOF'
#!/bin/bash
echo "mock cursor"
EOF
chmod +x "$HOME/mock-bin/cursor"

mkdir -p "$HOME/.codex"
cat > "$HOME/mock-bin/codex" << 'EOF'
#!/bin/bash
echo "mock codex"
EOF
chmod +x "$HOME/mock-bin/codex"

# ── Mock package manager ──
# Only installed if FACET_E2E_REAL_PACKAGES is not set.
# The mock logs "installed" packages to $HOME/.mock-packages
# and fakes the "is installed?" check so facet thinks it worked.

if [ "${FACET_E2E_REAL_PACKAGES:-}" != "1" ]; then

    MOCK_PKG_LOG="$HOME/.mock-packages"
    touch "$MOCK_PKG_LOG"

    # Mock brew (for macOS native runs)
    cat > "$HOME/mock-bin/brew" << 'BREWEOF'
#!/bin/bash
MOCK_PKG_LOG="$HOME/.mock-packages"
case "$1" in
    install)
        shift
        # Strip flags like --cask
        for arg in "$@"; do
            [[ "$arg" == --* ]] && continue
            echo "$arg" >> "$MOCK_PKG_LOG"
            echo "mock-brew: installed $arg"
        done
        ;;
    list)
        # Report what we've "installed"
        cat "$MOCK_PKG_LOG" 2>/dev/null
        ;;
    --prefix)
        echo "/opt/homebrew"
        ;;
    *)
        echo "mock-brew: $*"
        ;;
esac
exit 0
BREWEOF
    chmod +x "$HOME/mock-bin/brew"

    # Mock apt-get (for native Linux runs without root)
    cat > "$HOME/mock-bin/apt-get" << 'APTEOF'
#!/bin/bash
MOCK_PKG_LOG="$HOME/.mock-packages"
case "$1" in
    install)
        shift
        for arg in "$@"; do
            [[ "$arg" == -* ]] && continue
            echo "$arg" >> "$MOCK_PKG_LOG"
            echo "mock-apt: installed $arg"
        done
        ;;
    update)
        echo "mock-apt: updated"
        ;;
    *)
        echo "mock-apt: $*"
        ;;
esac
exit 0
APTEOF
    chmod +x "$HOME/mock-bin/apt-get"

    # Mock dpkg for "is installed?" checks
    cat > "$HOME/mock-bin/dpkg" << 'DPKGEOF'
#!/bin/bash
MOCK_PKG_LOG="$HOME/.mock-packages"
if [ "$1" = "-l" ]; then
    # Fake dpkg -l output for packages in our log
    while IFS= read -r pkg; do
        echo "ii  $pkg  1.0.0  amd64  mock package"
    done < "$MOCK_PKG_LOG" 2>/dev/null
fi
exit 0
DPKGEOF
    chmod +x "$HOME/mock-bin/dpkg"

    # Mock which/command for package existence checks
    # The real `which` still works for mock-bin items,
    # but for packages that install binaries, we need to
    # also fake those. We create stubs for common test packages.
    cat > "$HOME/mock-bin/tree" << 'EOF'
#!/bin/bash
echo "mock tree"
EOF
    chmod +x "$HOME/mock-bin/tree"

    echo "[mock-tools] Package manager mocked (FACET_E2E_REAL_PACKAGES != 1)"
else
    echo "[mock-tools] Using real package manager"
fi

# Ensure mock-bin is first in PATH
export PATH="$HOME/mock-bin:$PATH"

echo "[mock-tools] Installed mock AI tools (claude, cursor, codex)"
```

### 7.4 — Test Fixture Setup

File: `e2e/fixtures/setup-sarah.sh`

Populates the full Sarah real-world config inside the sandbox. All paths use `$HOME` which the harness has pointed to a temp directory — this never touches your real home.

```bash
#!/bin/bash
set -euo pipefail

# $HOME is set by the harness to the sandbox directory
FACET_DIR="$HOME/.facet"
mkdir -p "$FACET_DIR"/{profiles,configs/{shell,acme,ai}}

# --- base.yaml ---
cat > "$FACET_DIR/base.yaml" << 'YAML'
packages:
  - git
  - curl
  - jq

configs:
  .zshrc: configs/.zshrc
  .facet.d/01-path.sh: configs/shell/base-path.sh
  .facet.d/02-aliases.sh: configs/shell/base-aliases.sh

ai:
  mcp:
    filesystem:
      command: npx
      args: [-y, "@anthropic/mcp-filesystem"]
  permissions:
    deny:
      - Bash(rm -rf *)
      - Bash(sudo *)
      - Write(~/.ssh)
YAML

# --- profiles/acme.yaml ---
cat > "$FACET_DIR/profiles/acme.yaml" << 'YAML'
extends: base

vars:
  git_name: Sarah Chen
  git_email: sarah@acme-corp.com

packages:
  - tree

configs:
  .gitconfig: configs/acme/.gitconfig
  .npmrc: configs/acme/.npmrc
  .facet.d/10-acme-env.sh: configs/shell/acme-env.sh
  .facet.d/11-acme-aliases.sh: configs/shell/acme-aliases.sh

ai:
  mcp:
    postgres:
      command: npx
      args: [-y, "@anthropic/mcp-postgres"]
      env:
        DATABASE_URL: "{{.acme_db_url}}"
  permissions:
    allow:
      - Bash(pnpm test)
      - Bash(pnpm run build)
      - Read(~/code/acme)
      - Write(~/code/acme)
    deny:
      - Bash(terraform apply)
  claude:
    model: sonnet
  cursor:
    rules: |
      Use TypeScript strict mode.
      Prefer functional React components.
  codex:
    model: o3
    approval_mode: suggest
YAML

# --- profiles/personal.yaml ---
cat > "$FACET_DIR/profiles/personal.yaml" << 'YAML'
extends: base

vars:
  git_name: Sarah Chen
  git_email: sarah@hey.com

configs:
  .gitconfig: configs/.gitconfig
  .facet.d/10-personal-env.sh: configs/shell/personal-env.sh
  .facet.d/11-personal-aliases.sh: configs/shell/personal-aliases.sh

ai:
  mcp:
    memory:
      command: npx
      args: [-y, "@anthropic/mcp-memory"]
  permissions:
    allow:
      - Read(~/code)
      - Write(~/code)
      - Bash(poetry *)
  claude:
    model: opus
YAML

# --- .local.yaml ---
cat > "$FACET_DIR/.local.yaml" << 'YAML'
vars:
  acme_db_url: postgres://user:secret@localhost:5432/acme
YAML

# --- config files ---
cat > "$FACET_DIR/configs/.zshrc" << 'SHELL'
export EDITOR=nvim
alias ll="ls -la"
for f in ~/.facet.d/*.sh(N); do source "$f"; done
SHELL

cat > "$FACET_DIR/configs/.gitconfig" << 'GIT'
[user]
  name = {{.git_name}}
  email = {{.git_email}}
[core]
  editor = nvim
GIT

cat > "$FACET_DIR/configs/acme/.gitconfig" << 'GIT'
[user]
  name = {{.git_name}}
  email = {{.git_email}}
[core]
  editor = cursor --wait
[commit]
  gpgsign = true
GIT

cat > "$FACET_DIR/configs/acme/.npmrc" << 'NPM'
registry=https://npm.acme-corp.com
always-auth=true
NPM

# --- shell snippets ---
echo 'export PATH="$HOME/.local/bin:$PATH"' > "$FACET_DIR/configs/shell/base-path.sh"
echo 'alias gs="git status"' > "$FACET_DIR/configs/shell/base-aliases.sh"
echo 'export COMPANY=acme' > "$FACET_DIR/configs/shell/acme-env.sh"
echo 'alias deploy="pnpm run deploy"' > "$FACET_DIR/configs/shell/acme-aliases.sh"
echo 'export SIDE_PROJECTS=true' > "$FACET_DIR/configs/shell/personal-env.sh"
echo 'alias blog="cd ~/blog"' > "$FACET_DIR/configs/shell/personal-aliases.sh"

# --- AI files ---
echo '# Acme code review skill' > "$FACET_DIR/configs/ai/acme-review.md"
echo 'Use TypeScript strict mode.' > "$FACET_DIR/configs/ai/acme-cursor-rules.md"

# --- git init ---
cd "$FACET_DIR"
git init -q
echo ".local.yaml" > .gitignore
echo ".state.json" >> .gitignore
git add -A
git commit -q -m "initial facet config"

echo "[setup-sarah] Config repo ready at $FACET_DIR"
```

### 7.5 — Test Harness (HOME-isolated)

File: `e2e/harness.sh`

**Critical design: every test run gets a fresh temp HOME.** On macOS this prevents touching your real `~/.gitconfig`, `~/.claude.json`, etc. On Docker it's redundant (container is already isolated) but we do it anyway so the suites are identical on both platforms.

```bash
#!/bin/bash
set -euo pipefail

# ── Create isolated HOME ──
# Every test run gets a fresh directory. Nothing touches the real HOME.
REAL_HOME="$HOME"
export E2E_SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/facet-e2e.XXXXXXXX")
export HOME="$E2E_SANDBOX"

# Resolve suite/fixture locations (may differ between Docker and native)
if [ -d "/opt/e2e" ]; then
    # Docker: suites/fixtures were COPYed to /opt/e2e
    SUITE_DIR="/opt/e2e/suites"
    FIXTURE_DIR="/opt/e2e/fixtures"
else
    # Native (macOS/Linux): relative to this script
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    SUITE_DIR="$SCRIPT_DIR/suites"
    FIXTURE_DIR="$SCRIPT_DIR/fixtures"
fi

export SUITE_DIR FIXTURE_DIR

# Cleanup on exit — always remove the sandbox, even on failure
cleanup() {
    local exit_code=$?
    if [ -n "${E2E_SANDBOX:-}" ] && [ -d "$E2E_SANDBOX" ]; then
        rm -rf "$E2E_SANDBOX"
    fi
    exit $exit_code
}
trap cleanup EXIT

# ── Set up mock tools inside the sandbox ──
bash "$FIXTURE_DIR/mock-tools.sh"
export PATH="$HOME/mock-bin:$PATH"

# Filter suites if specific one requested
if [ $# -gt 0 ]; then
    SUITES=("$@")
else
    SUITES=("$SUITE_DIR"/[0-9]*.sh)
fi

echo "========================================"
echo "  facet E2E test run"
echo "  $(date -Iseconds)"
echo "  Sandbox: $E2E_SANDBOX"
echo "  OS: $(uname -s) $(uname -m)"
echo "  facet: $(facet --version 2>/dev/null || echo 'not found')"
echo "========================================"
echo ""

PASSED=0
FAILED=0
ERRORS=()

for suite in "${SUITES[@]}"; do
    name=$(basename "$suite" .sh)

    # Skip helpers file
    [[ "$name" == "helpers" ]] && continue

    echo "--- [$name] ---"

    # Each suite gets its own clean HOME subdirectory
    # so suites don't leak state to each other
    SUITE_HOME=$(mktemp -d "$E2E_SANDBOX/suite.XXXXXXXX")
    
    if HOME="$SUITE_HOME" PATH="$SUITE_HOME/mock-bin:$PATH" \
       E2E_SANDBOX="$SUITE_HOME" FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$FIXTURE_DIR/mock-tools.sh" >/dev/null 2>&1 \
       && HOME="$SUITE_HOME" PATH="$SUITE_HOME/mock-bin:$PATH" \
       E2E_SANDBOX="$SUITE_HOME" FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$suite" 2>&1; then
        echo "  ✓ PASS"
        ((PASSED++))
    else
        echo "  ✗ FAIL (exit $?)"
        ((FAILED++))
        ERRORS+=("$name")
    fi
    echo ""
done

echo "========================================"
echo "  Results: $PASSED passed, $FAILED failed"
echo "  Sandbox was: $E2E_SANDBOX (cleaned up)"
if [ $FAILED -gt 0 ]; then
    echo "  Failed: ${ERRORS[*]}"
    echo "========================================"
    exit 1
fi
echo "========================================"
exit 0
```

**What this gives you:**
- Each test run creates `/tmp/facet-e2e.XXXXXXXX/` as the sandbox
- Each suite within the run gets its own `suite.XXXXXXXX/` subdirectory as HOME — suites can't leak state to each other
- `trap cleanup EXIT` ensures the sandbox is deleted even on failure, Ctrl+C, etc.
- On macOS: your real `~/.gitconfig`, `~/.claude.json`, `~/.cursor/` are never touched
- On Docker: same code runs identically (just redundant isolation)
- Same harness works in both environments — no separate scripts

### 7.6 — Test Suites

Each suite is a self-contained shell script. It sets up what it needs, runs facet commands, and asserts on the results. Helper functions at the top.

**Common assertions** (sourced by each suite):

File: `e2e/suites/helpers.sh`

```bash
#!/bin/bash

# All paths use $HOME which the harness points to the sandbox.
# Tests never touch the real user home.

assert_file_exists() {
    if [ ! -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to exist"
        exit 1
    fi
}

assert_file_not_exists() {
    if [ -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to NOT exist"
        exit 1
    fi
}

assert_symlink() {
    if [ ! -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to be a symlink"
        exit 1
    fi
}

assert_not_symlink() {
    if [ -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to NOT be a symlink"
        exit 1
    fi
}

assert_file_contains() {
    if ! grep -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to contain '$2'"
        echo "  Actual content:"
        head -20 "$1" 2>/dev/null || echo "  (file not readable)"
        exit 1
    fi
}

assert_file_not_contains() {
    if grep -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to NOT contain '$2'"
        exit 1
    fi
}

assert_json_field() {
    local file="$1" path="$2" expected="$3"
    local actual
    actual=$(jq -r "$path" "$file" 2>/dev/null)
    if [ "$actual" != "$expected" ]; then
        echo "  ASSERT FAIL: $file $path"
        echo "  Expected: $expected"
        echo "  Actual:   $actual"
        exit 1
    fi
}

assert_exit_code() {
    local expected="$1"
    shift
    local actual
    set +e
    "$@" >/dev/null 2>&1
    actual=$?
    set -e
    if [ "$actual" -ne "$expected" ]; then
        echo "  ASSERT FAIL: expected exit code $expected, got $actual"
        echo "  Command: $*"
        exit 1
    fi
}

assert_dir_file_count() {
    local dir="$1" expected="$2"
    local actual
    actual=$(find "$dir" -maxdepth 1 -type f -o -type l | wc -l | tr -d ' ')
    if [ "$actual" -ne "$expected" ]; then
        echo "  ASSERT FAIL: expected $expected files in $dir, got $actual"
        ls -la "$dir" 2>/dev/null
        exit 1
    fi
}

# Helper to source the right fixture scripts
setup_sarah() {
    bash "$FIXTURE_DIR/setup-sarah.sh"
}
```

Each suite sources helpers like this (works in both Docker and native):

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"
```

**Suite 01: init**

File: `e2e/suites/01-init.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Clean slate (HOME is already the sandbox)
rm -rf "$HOME/.facet"

# Test: init creates structure
facet init
assert_file_exists "$HOME/.facet/base.yaml"
assert_file_exists "$HOME/.facet/profiles"
assert_file_exists "$HOME/.facet/configs"
assert_file_exists "$HOME/.facet/.gitignore"
assert_file_contains "$HOME/.facet/.gitignore" ".local.yaml"
assert_file_contains "$HOME/.facet/.gitignore" ".state.json"

# Test: it's a git repo
cd "$HOME/.facet" && git status >/dev/null 2>&1
echo "  init creates valid structure"

# Cleanup for next tests
rm -rf "$HOME/.facet"
```

**Suite 02: apply basic profile**

File: `e2e/suites/02-apply-basic.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Setup Sarah's config (writes into sandbox HOME)
setup_sarah

# Apply acme profile
facet apply acme
echo "  apply exited cleanly"

# Check state file was written
assert_file_exists "$HOME/.facet/.state.json"
assert_json_field "$HOME/.facet/.state.json" '.profile' 'acme'
echo "  state file written"

# Check packages ("tree" should appear in mock install log or be in PATH)
which tree >/dev/null 2>&1
echo "  packages installed"
```

**Suite 03: config deployment**

File: `e2e/suites/03-configs.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah
facet apply acme

# .zshrc should be symlinked (no vars)
assert_symlink "$HOME/.zshrc"
echo "  .zshrc is symlinked"

# .gitconfig should be templated (has {{vars}}) — so it's a regular file, not symlink
assert_not_symlink "$HOME/.gitconfig"
assert_file_contains "$HOME/.gitconfig" "sarah@acme-corp.com"
assert_file_contains "$HOME/.gitconfig" "cursor --wait"
assert_file_not_contains "$HOME/.gitconfig" "{{.git_email}}"
echo "  .gitconfig templated with acme vars"

# .npmrc should be symlinked (no vars)
assert_symlink "$HOME/.npmrc"
assert_file_contains "$HOME/.npmrc" "acme-corp.com"
echo "  .npmrc deployed"

# .facet.d/ should have 4 snippets: 01, 02, 10, 11
assert_file_exists "$HOME/.facet.d"
assert_dir_file_count "$HOME/.facet.d" 4
assert_symlink "$HOME/.facet.d/01-path.sh"
assert_symlink "$HOME/.facet.d/10-acme-env.sh"
assert_symlink "$HOME/.facet.d/11-acme-aliases.sh"
echo "  .facet.d/ has 4 snippets"

# Snippets should have correct content
assert_file_contains "$HOME/.facet.d/10-acme-env.sh" "COMPANY=acme"
assert_file_contains "$HOME/.facet.d/11-acme-aliases.sh" "deploy"
echo "  snippet content correct"
```

**Suite 04: AI tool configs**

File: `e2e/suites/04-ai-tools.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah
facet apply acme

# --- Claude Code ---
assert_file_exists "$HOME/.claude.json"
assert_json_field "$HOME/.claude.json" '.mcpServers.filesystem.command' 'npx'
assert_json_field "$HOME/.claude.json" '.mcpServers.postgres.command' 'npx'
assert_json_field "$HOME/.claude.json" '.mcpServers.postgres.env.DATABASE_URL' \
    'postgres://user:secret@localhost:5432/acme'
echo "  claude MCP servers written"

assert_file_exists "$HOME/.claude/settings.json"
assert_file_contains "$HOME/.claude/settings.json" 'Bash(rm -rf *)'
assert_file_contains "$HOME/.claude/settings.json" 'Bash(sudo *)'
assert_file_contains "$HOME/.claude/settings.json" 'Bash(terraform apply)'
assert_file_contains "$HOME/.claude/settings.json" 'Bash(pnpm test)'
assert_file_contains "$HOME/.claude/settings.json" 'Read(~/code/acme)'
echo "  claude permissions written"

# --- Cursor ---
assert_file_exists "$HOME/.cursor/mcp.json"
assert_json_field "$HOME/.cursor/mcp.json" '.mcpServers.filesystem.command' 'npx'
assert_json_field "$HOME/.cursor/mcp.json" '.mcpServers.postgres.command' 'npx'
echo "  cursor MCP servers written"

# --- Codex ---
assert_file_exists "$HOME/.codex/config.toml"
assert_file_contains "$HOME/.codex/config.toml" 'model = "o3"'
assert_file_contains "$HOME/.codex/config.toml" 'approval_mode = "suggest"'
assert_file_contains "$HOME/.codex/config.toml" '[mcp_servers.filesystem]'
assert_file_contains "$HOME/.codex/config.toml" '[mcp_servers.postgres]'
echo "  codex config written"
```

**Suite 05: profile switching**

File: `e2e/suites/05-profile-switch.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah

# Apply acme first
facet apply acme
assert_file_contains "$HOME/.gitconfig" "sarah@acme-corp.com"
assert_file_contains "$HOME/.facet.d/10-acme-env.sh" "COMPANY=acme"
assert_file_exists "$HOME/.facet.d/11-acme-aliases.sh"
echo "  acme applied"

# Switch to personal
facet apply personal

# .gitconfig should now have personal email
assert_file_contains "$HOME/.gitconfig" "sarah@hey.com"
assert_file_not_contains "$HOME/.gitconfig" "acme-corp"
assert_file_not_contains "$HOME/.gitconfig" "gpgsign"
echo "  .gitconfig switched to personal"

# .facet.d/ should have personal snippets, NOT acme ones
assert_file_not_exists "$HOME/.facet.d/10-acme-env.sh"
assert_file_not_exists "$HOME/.facet.d/11-acme-aliases.sh"
assert_file_exists "$HOME/.facet.d/10-personal-env.sh"
assert_file_exists "$HOME/.facet.d/11-personal-aliases.sh"
assert_file_contains "$HOME/.facet.d/10-personal-env.sh" "SIDE_PROJECTS"
echo "  .facet.d/ switched to personal snippets"

# .npmrc should be gone (personal doesn't define it)
assert_file_not_exists "$HOME/.npmrc"
echo "  .npmrc removed"

# Claude MCP should now have memory, not postgres
assert_json_field "$HOME/.claude.json" '.mcpServers.memory.command' 'npx'
has_postgres=$(jq -r '.mcpServers.postgres // empty' "$HOME/.claude.json")
if [ -n "$has_postgres" ]; then
    echo "  ASSERT FAIL: postgres MCP should not be in personal profile"
    exit 1
fi
echo "  claude MCP switched to personal"

# Permissions: base denies still present, acme-specific deny gone
assert_file_contains "$HOME/.claude/settings.json" 'Bash(rm -rf *)'
assert_file_contains "$HOME/.claude/settings.json" 'Bash(sudo *)'
assert_file_not_contains "$HOME/.claude/settings.json" 'terraform'
echo "  permissions switched correctly"

# State file updated
assert_json_field "$HOME/.facet/.state.json" '.profile' 'personal'
echo "  state file updated"
```

**Suite 06: status and diff**

File: `e2e/suites/06-status-diff.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah
facet apply acme

# Status should show acme profile
facet status | grep -q "acme"
echo "  status shows acme"

# Diff against personal should show changes
facet diff personal | grep -q "sarah@hey.com"
echo "  diff shows email change"
```

**Suite 07: doctor**

File: `e2e/suites/07-doctor.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah
facet apply acme

# Doctor should pass on clean setup
facet doctor
echo "  doctor passes clean"

# Break a symlink — delete the source
rm "$HOME/.facet/configs/shell/acme-env.sh"

# Doctor should report the broken symlink
facet doctor 2>&1 | grep -qi "broken\|missing\|error"
echo "  doctor detects broken symlink"

# Restore
echo 'export COMPANY=acme' > "$HOME/.facet/configs/shell/acme-env.sh"
```

**Suite 08: idempotency**

File: `e2e/suites/08-idempotent.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah

# Apply twice
facet apply acme
FIRST_GITCONFIG=$(cat "$HOME/.gitconfig")
FIRST_CLAUDE=$(cat "$HOME/.claude.json")
FIRST_DOTD=$(ls "$HOME/.facet.d/")

facet apply acme
SECOND_GITCONFIG=$(cat "$HOME/.gitconfig")
SECOND_CLAUDE=$(cat "$HOME/.claude.json")
SECOND_DOTD=$(ls "$HOME/.facet.d/")

# Everything should be identical
if [ "$FIRST_GITCONFIG" != "$SECOND_GITCONFIG" ]; then
    echo "  ASSERT FAIL: .gitconfig changed on second apply"
    exit 1
fi

if [ "$FIRST_CLAUDE" != "$SECOND_CLAUDE" ]; then
    echo "  ASSERT FAIL: .claude.json changed on second apply"
    exit 1
fi

if [ "$FIRST_DOTD" != "$SECOND_DOTD" ]; then
    echo "  ASSERT FAIL: .facet.d/ contents changed on second apply"
    exit 1
fi

echo "  double apply is idempotent"
```

**Suite 09: edge cases**

File: `e2e/suites/09-edge-cases.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# --- Missing profile ---
setup_sarah
assert_exit_code 1 facet apply nonexistent
echo "  missing profile errors"

# --- Undefined variable ---
echo 'secret_thing: {{.undefined_var}}' >> "$HOME/.facet/base.yaml"
assert_exit_code 1 facet apply acme
echo "  undefined var errors"

# Restore
cd "$HOME/.facet" && git checkout -- base.yaml

# --- No .local.yaml is fine ---
rm -f "$HOME/.facet/.local.yaml"
facet apply personal
echo "  no .local.yaml is OK"

# --- Empty profile (just extends) ---
cat > "$HOME/.facet/profiles/minimal.yaml" << 'YAML'
extends: base
YAML
facet apply minimal
assert_symlink "$HOME/.zshrc"
assert_file_exists "$HOME/.facet.d/01-path.sh"
echo "  empty profile inherits base correctly"
```

**Suite 10: package installation**

File: `e2e/suites/10-packages.sh`

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah

if [ "${FACET_E2E_REAL_PACKAGES:-}" = "1" ]; then
    # Real package installation (Docker with apt)
    # Verify tree is not installed yet
    if which tree >/dev/null 2>&1; then
        echo "  SKIP: tree already installed"
    else
        facet apply acme
        which tree >/dev/null 2>&1
        echo "  tree package installed via real apt"
    fi
else
    # Mock package installation (native macOS/Linux)
    facet apply acme

    # Check mock package log recorded the install
    assert_file_exists "$HOME/.mock-packages"
    assert_file_contains "$HOME/.mock-packages" "tree"
    echo "  tree package recorded in mock install log"

    # Mock tree binary should be callable
    which tree >/dev/null 2>&1
    echo "  mock tree binary available in PATH"
fi

# Switching profiles doesn't break
facet apply personal
echo "  switching profiles doesn't break packages"
```

**Suite 11: shell integration verification**

File: `e2e/suites/11-shell-verify.sh`

This suite answers: "does the shell actually work?" Instead of just checking files exist, it spawns a real zsh process in the sandbox and verifies that environment variables, aliases, and the `.facet.d/` sourcing chain all function correctly.

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_sarah

# ── Apply acme and verify the shell loads correctly ──
facet apply acme

# Verify: .zshrc sources .facet.d/ snippets and COMPANY is set
RESULT=$(zsh -c "source \$HOME/.zshrc && echo \$COMPANY" 2>/dev/null)
if [ "$RESULT" != "acme" ]; then
    echo "  ASSERT FAIL: expected COMPANY=acme from shell, got: '$RESULT'"
    exit 1
fi
echo "  zsh sources .facet.d/ → COMPANY=acme"

# Verify: base PATH snippet loaded (check a var or PATH entry)
RESULT=$(zsh -c "source \$HOME/.zshrc && echo \$PATH" 2>/dev/null)
if [[ "$RESULT" != *".local/bin"* ]]; then
    echo "  ASSERT FAIL: expected .local/bin in PATH from base-path.sh"
    exit 1
fi
echo "  zsh sources base-path.sh → .local/bin in PATH"

# Verify: alias from acme-aliases.sh is defined
RESULT=$(zsh -c "source \$HOME/.zshrc && alias deploy" 2>/dev/null)
if [[ "$RESULT" != *"pnpm run deploy"* ]]; then
    echo "  ASSERT FAIL: expected 'deploy' alias from acme-aliases.sh"
    echo "  Got: $RESULT"
    exit 1
fi
echo "  zsh sources acme-aliases.sh → deploy alias defined"

# Verify: base alias also loaded
RESULT=$(zsh -c "source \$HOME/.zshrc && alias gs" 2>/dev/null)
if [[ "$RESULT" != *"git status"* ]]; then
    echo "  ASSERT FAIL: expected 'gs' alias from base-aliases.sh"
    exit 1
fi
echo "  zsh sources base-aliases.sh → gs alias defined"

# Verify: EDITOR from shared .zshrc
RESULT=$(zsh -c "source \$HOME/.zshrc && echo \$EDITOR" 2>/dev/null)
if [ "$RESULT" != "nvim" ]; then
    echo "  ASSERT FAIL: expected EDITOR=nvim from .zshrc, got: '$RESULT'"
    exit 1
fi
echo "  zsh .zshrc sets EDITOR=nvim"

# ── Switch to personal and verify shell changes ──
facet apply personal

RESULT=$(zsh -c "source \$HOME/.zshrc && echo \$COMPANY" 2>/dev/null)
if [ -n "$RESULT" ]; then
    echo "  ASSERT FAIL: COMPANY should be unset in personal profile, got: '$RESULT'"
    exit 1
fi
echo "  after switch: COMPANY is unset"

RESULT=$(zsh -c "source \$HOME/.zshrc && echo \$SIDE_PROJECTS" 2>/dev/null)
if [ "$RESULT" != "true" ]; then
    echo "  ASSERT FAIL: expected SIDE_PROJECTS=true from personal-env.sh"
    exit 1
fi
echo "  after switch: SIDE_PROJECTS=true"

# Verify: personal alias loaded, acme alias gone
RESULT=$(zsh -c "source \$HOME/.zshrc && alias blog" 2>/dev/null)
if [[ "$RESULT" != *"cd ~/blog"* ]]; then
    echo "  ASSERT FAIL: expected 'blog' alias from personal-aliases.sh"
    exit 1
fi
echo "  after switch: blog alias defined"

# acme alias should be gone
RESULT=$(zsh -c "source \$HOME/.zshrc && alias deploy 2>&1" || true)
if [[ "$RESULT" != *"not found"* ]] && [[ "$RESULT" != *"no such"* ]]; then
    # On some zsh versions the error message differs, but it shouldn't succeed
    if [[ "$RESULT" == *"pnpm"* ]]; then
        echo "  ASSERT FAIL: 'deploy' alias should not exist in personal profile"
        exit 1
    fi
fi
echo "  after switch: deploy alias removed"

echo "  full shell integration verified across profile switch"
```

**Why this matters:** Suites 03 and 05 verify that files are placed correctly. Suite 11 verifies the entire chain actually works: `.zshrc` → `for f in ~/.facet.d/*.sh(N)` → sourced snippets → environment variables and aliases are live. This catches issues like incorrect symlink targets, broken glob patterns, or shell syntax errors that file-existence checks would miss.

### 7.7 — Makefile Targets

File: `Makefile` (add to existing)

```makefile
# --- Binary builds ---

build:
	go build -o facet .

build-linux:
	GOOS=linux GOARCH=amd64 go build -o e2e/facet-linux .

build-linux-arm:
	GOOS=linux GOARCH=arm64 go build -o e2e/facet-linux-arm64 .

# --- Unit tests ---

test:
	go test ./...

# --- E2E: Docker (Linux, with real apt) ---

e2e: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -e FACET_E2E_REAL_PACKAGES=1 facet-e2e

# Run a single suite in Docker
e2e-suite: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -e FACET_E2E_REAL_PACKAGES=1 facet-e2e \
		bash -c 'SUITE_DIR=/opt/e2e/suites FIXTURE_DIR=/opt/e2e/fixtures bash /opt/e2e/suites/$(SUITE).sh'

# Debug: drop into the container
e2e-shell: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -it --entrypoint /bin/bash facet-e2e

# --- E2E: Native (macOS or Linux, mocked packages, isolated HOME) ---
# Safe to run on your real machine — never touches your actual config files.

e2e-local: build
	@echo "Running E2E locally (HOME will be sandboxed, packages mocked)"
	@PATH="$$PWD:$$PATH" bash e2e/harness.sh

# Run single suite locally
e2e-local-suite: build
	@PATH="$$PWD:$$PATH" bash e2e/harness.sh e2e/suites/$(SUITE).sh

# --- CI convenience ---

ci: test e2e
	@echo "All tests passed"
```

### 7.8 — Go Test Wrapper (optional)

File: `e2e/e2e_test.go`

For developers who prefer `go test`. Runs the Docker E2E from Go's test framework.

```go
//go:build e2e
// +build e2e

package e2e

import (
    "os"
    "os/exec"
    "runtime"
    "testing"
)

func TestE2E_Docker(t *testing.T) {
    if _, err := exec.LookPath("docker"); err != nil {
        t.Skip("docker not found, skipping Docker E2E")
    }

    goos := "linux"
    goarch := "amd64"
    if runtime.GOARCH == "arm64" {
        goarch = "arm64"
    }

    build := exec.Command("go", "build", "-o", "facet-linux", "..")
    build.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %s\n%s", err, out)
    }

    dockerBuild := exec.Command("docker", "build", "-t", "facet-e2e", "-f", "Dockerfile.ubuntu", ".")
    if out, err := dockerBuild.CombinedOutput(); err != nil {
        t.Fatalf("docker build failed: %s\n%s", err, out)
    }

    dockerRun := exec.Command("docker", "run", "--rm",
        "-e", "FACET_E2E_REAL_PACKAGES=1",
        "facet-e2e")
    dockerRun.Stdout = os.Stdout
    dockerRun.Stderr = os.Stderr
    if err := dockerRun.Run(); err != nil {
        t.Fatalf("E2E tests failed: %s", err)
    }
}

func TestE2E_Native(t *testing.T) {
    // This test is safe to run on your real machine.
    // The harness sets HOME to a temp directory — nothing touches your real configs.

    build := exec.Command("go", "build", "-o", "facet", "..")
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %s\n%s", err, out)
    }

    harness := exec.Command("bash", "harness.sh")
    harness.Env = append(os.Environ(), "PATH="+os.Getenv("PWD")+"/..:"+os.Getenv("PATH"))
    harness.Stdout = os.Stdout
    harness.Stderr = os.Stderr
    if err := harness.Run(); err != nil {
        t.Fatalf("Native E2E tests failed: %s", err)
    }
}
```

Run with: `go test -tags e2e -v -run Docker ./e2e/` or `go test -tags e2e -v -run Native ./e2e/`

### 7.9 — macOS Isolation: How It Works

**The problem:** running E2E natively on macOS would write to your real `~/.gitconfig`, `~/.claude.json`, `~/.cursor/mcp.json`, etc.

**The solution:** the harness overrides `HOME` to a temp directory before running any suite.

```
Your real machine                    During E2E run
─────────────────                    ──────────────
HOME=/Users/you                      HOME=/tmp/facet-e2e.A1B2C3/suite.X1Y2Z3
~/.gitconfig (yours, untouched)      ~/.gitconfig = /tmp/.../suite.X1Y2Z3/.gitconfig
~/.claude.json (yours, untouched)    ~/.claude.json = /tmp/.../suite.X1Y2Z3/.claude.json
~/.facet/ (yours, untouched)        ~/.facet/ = /tmp/.../suite.X1Y2Z3/.facet/
```

**What's isolated:**
- All dotfile writes (`~/.gitconfig`, `~/.zshrc`, `~/.npmrc`)
- All AI tool configs (`~/.claude.json`, `~/.claude/settings.json`, `~/.cursor/mcp.json`, `~/.codex/config.toml`)
- The `.facet.d/` directory
- The `.facet/` config repo itself
- The `.state.json` state file

**What's mocked (on native runs):**
- `brew` / `apt-get` — mock binaries that log to `$HOME/.mock-packages` instead of installing
- `claude`, `cursor`, `codex` — mock binaries that exist in PATH and have config directories
- Package binaries like `tree` — mock stubs so `which tree` succeeds

**What uses the real system:**
- `git` — needed for `facet init` (creates a real repo in the temp dir)
- `jq` — needed for assertion helpers (must be installed on your machine)
- The facet binary itself

**Cleanup:** `trap cleanup EXIT` in the harness ensures the temp directory is deleted even on failure, Ctrl+C, or any unexpected exit.

### 7.10 — CI Pipeline

File: `.github/workflows/e2e.yml`

```yaml
name: E2E Tests

on:
  push:
    branches: [main]
  pull_request:

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go test ./...

  e2e-linux:
    needs: unit
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: make e2e
        # Runs in Docker with real apt, full isolation

  e2e-macos:
    needs: unit
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: brew install jq  # needed for assert_json_field
      - run: make e2e-local
        # Runs natively with mocked packages, HOME sandboxed
        # Safe: never touches the runner's real config
```

### Phase 7 Done When

- `make e2e` runs all 11 suites in Docker and exits 0
- `make e2e-local` runs all suites natively with isolated HOME — your real configs are untouched
- `make e2e-suite SUITE=05-profile-switch` runs a single suite in Docker
- `make e2e-local-suite SUITE=05-profile-switch` runs a single suite natively
- `make e2e-shell` drops into the Docker container for debugging
- Suite 11 verifies `zsh -c "source $HOME/.zshrc && echo $COMPANY"` returns `acme` — full shell chain works
- After `make e2e-local`, `ls /tmp/facet-e2e*` shows nothing (cleanup worked)
- Your real `~/.gitconfig`, `~/.claude.json`, `~/.cursor/`, `~/.codex/` are identical before and after `make e2e-local`
- CI runs both Linux (Docker + real apt) and macOS (native + mocked packages)

---

## Implementation Order

This is the recommended sequence. Each step should be a separate commit/PR.

```
Step 1:  Project scaffold — go mod init, cobra setup, main.go, Makefile
Step 2:  types.go — all data structures
Step 3:  loader.go + tests — parse YAML
Step 4:  merger.go + tests — merge logic
Step 5:  resolver.go + tests — var substitution
Step 6:  Test fixtures (basic + realworld + edge)
Step 7:  deployer.go + tests — symlink and template
Step 8:  dotd.go + tests — .facet.d/ management
Step 9:  detect.go — OS detection
Step 10: brew.go + tests — Homebrew backend
Step 11: adapter.go + permissions.go — AI adapter interface
Step 12: claude.go + tests — Claude Code adapter
Step 13: cursor.go + tests — Cursor adapter
Step 14: codex.go + tests — Codex adapter
Step 15: reporter.go — terminal output
Step 16: apply.go — wire it all together
Step 17: status.go, diff.go — read state
Step 18: init_cmd.go, doctor.go
Step 19: E2E infrastructure — Dockerfile, harness (HOME isolation), helpers, mock-tools, fixtures
Step 20: E2E suites 01-09 (config, AI, profile switching, idempotency, edge cases)
Step 21: E2E suites 10-11 (packages with mock/real, shell integration verification)
Step 22: apt.go + real package tests in Docker
Step 23: CI pipeline — GitHub Actions for Linux Docker + macOS native
```

---

## Out of Scope (for now)

- Multi-level extends (only `extends: base` supported)
- Plugin system for custom adapters
- Remote apply (SSH)
- Lock files for package versions
- GUI / TUI
- Copilot adapter (add later, same pattern)
- Drift detection beyond basic symlink checking
- Hooks / pre/post-apply scripts
- Windows support
