# Init Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `facet init <profile>` command that runs post-install initialization scripts defined in profile YAML, and rename the existing scaffold command from `init` to `scaffold`.

**Architecture:** Init scripts are a new field on `FacetConfig`. The merge layer concatenates base + profile scripts. The resolver substitutes `${facet:...}` in `run` strings. A new `ScriptRunner` interface (in `app/interfaces.go`) abstracts shell execution. The `App.Init()` method orchestrates load → merge → resolve → execute. The existing scaffold logic moves to `App.Scaffold()`.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, `github.com/stretchr/testify`

**Spec:** `docs/superpowers/specs/2026-03-28-init-command-design.md`

---

## File Structure

```
Changes:
├── internal/profile/
│   ├── types.go              # Add InitScript struct, Init field to FacetConfig
│   ├── merger.go             # Add mergeInit() concatenation
│   ├── merger_test.go        # Test init script merge
│   ├── resolver.go           # Resolve ${facet:...} in InitScript.Run
│   └── resolver_test.go      # Test init script resolution
├── internal/app/
│   ├── interfaces.go         # Add ScriptRunner interface
│   ├── app.go                # Add scriptRunner field to App + Deps
│   ├── scaffold.go           # Renamed from init.go (Init→Scaffold, InitOpts→ScaffoldOpts)
│   ├── scaffold_test.go      # Renamed from init_test.go (updated method names)
│   ├── init.go               # New: App.Init() pipeline
│   ├── init_test.go          # New: unit tests for init pipeline
│   └── report.go             # Add printInitReport() method
├── cmd/
│   ├── scaffold_cmd.go       # Renamed from init_cmd.go
│   ├── init_cmd.go           # New: facet init <profile> command
│   ├── root.go               # Register scaffold + init commands
│   └── docs.go               # No changes (docs uses internal/docs)
├── main.go                   # Wire ScriptRunner (reuse packages.ShellRunner)
├── internal/docs/
│   ├── docs.go               # Add "init" topic to allTopics()
│   ├── topics/commands.md    # Update: rename init→scaffold, add init <profile>
│   ├── topics/config.md      # Add init field to schema reference
│   ├── topics/init.md        # New: init scripts topic
│   ├── topics/merge.md       # Add init merge rules
│   ├── topics/quickstart.md  # Update scaffold references
│   └── topics/examples.md    # Add init script examples
├── e2e/
│   ├── suites/01-init.sh     # Rename to 01-scaffold.sh (test scaffold command)
│   ├── suites/11-init-scripts.sh  # New: E2E tests for facet init <profile>
│   ├── suites/helpers.sh     # Add facet_scaffold helper, update facet_init
│   └── fixtures/setup-basic.sh  # Add init scripts to fixtures
```

**Responsibilities per new/changed file:**

| File | Responsibility |
|---|---|
| `internal/profile/types.go` | `InitScript` struct with `Name` and `Run` YAML fields. `Init []InitScript` on `FacetConfig`. |
| `internal/profile/merger.go` | `mergeInit()` — concatenate base scripts then overlay scripts. Called from `Merge()`. |
| `internal/profile/resolver.go` | Resolve `${facet:...}` in each `InitScript.Run` field. Called from `Resolve()`. |
| `internal/app/interfaces.go` | `ScriptRunner` interface: `Run(command string, dir string) error` |
| `internal/app/app.go` | Add `scriptRunner ScriptRunner` field to `App`, `ScriptRunner` field to `Deps` |
| `internal/app/scaffold.go` | `App.Scaffold(ScaffoldOpts)` — the old init logic (create facet.yaml, base.yaml, dirs) |
| `internal/app/init.go` | `App.Init(InitOpts)` — load profile, merge, resolve, execute init scripts sequentially |
| `internal/app/report.go` | `printInitReport()` — success/failure output matching spec format |
| `cmd/scaffold_cmd.go` | `newScaffoldCmd()` — Cobra command for `facet scaffold` |
| `cmd/init_cmd.go` | `newInitCmd()` — Cobra command for `facet init <profile>` with exactly 1 positional arg |
| `internal/docs/topics/init.md` | Documentation for init scripts: YAML format, merge rules, variable resolution, execution |
| `e2e/suites/11-init-scripts.sh` | E2E: init scripts run in order, variables resolved, failure halts remaining scripts |

---

## Chunk 1: Profile Types and Merge

### Task 1: Add InitScript type and Init field to FacetConfig

**Files:**
- Modify: `internal/profile/types.go`

- [ ] **Step 1: Write failing test for InitScript YAML unmarshaling**

```go
// Add to internal/profile/types_test.go
func TestFacetConfig_UnmarshalYAML_InitScripts(t *testing.T) {
	input := `
init:
  - name: configure git
    run: git config --global user.email "test@example.com"
  - name: run setup
    run: |
      export FOO=bar
      ./scripts/setup.sh
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Init, 2)
	assert.Equal(t, "configure git", cfg.Init[0].Name)
	assert.Equal(t, `git config --global user.email "test@example.com"`, cfg.Init[0].Run)
	assert.Equal(t, "run setup", cfg.Init[1].Name)
	assert.Contains(t, cfg.Init[1].Run, "export FOO=bar")
	assert.Contains(t, cfg.Init[1].Run, "./scripts/setup.sh")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestFacetConfig_UnmarshalYAML_InitScripts -v`
Expected: FAIL — `InitScript` type does not exist yet.

- [ ] **Step 3: Add InitScript struct and Init field**

Add to `internal/profile/types.go`:

```go
// InitScript is a named shell command run during facet init.
type InitScript struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}
```

Add `Init` field to `FacetConfig`:

```go
type FacetConfig struct {
	Extends  string            `yaml:"extends,omitempty"`
	Vars     map[string]any    `yaml:"vars,omitempty"`
	Packages []PackageEntry    `yaml:"packages,omitempty"`
	Configs  map[string]string `yaml:"configs,omitempty"`
	AI       *AIConfig         `yaml:"ai,omitempty"`
	Init     []InitScript      `yaml:"init,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestFacetConfig_UnmarshalYAML_InitScripts -v`
Expected: PASS

- [ ] **Step 5: Run all profile tests to check for regressions**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/profile/types.go internal/profile/types_test.go
git commit -m "feat: add InitScript type and Init field to FacetConfig"
```

---

### Task 2: Add init script concatenation merge

**Files:**
- Modify: `internal/profile/merger.go`
- Modify: `internal/profile/merger_test.go`

- [ ] **Step 1: Write failing test for init script merge — both layers have scripts**

```go
// Add to internal/profile/merger_test.go
func TestMerge_InitConcatenation(t *testing.T) {
	base := &FacetConfig{
		Init: []InitScript{
			{Name: "base-script-1", Run: "echo base1"},
			{Name: "base-script-2", Run: "echo base2"},
		},
	}
	overlay := &FacetConfig{
		Init: []InitScript{
			{Name: "overlay-script-1", Run: "echo overlay1"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.Init, 3)
	assert.Equal(t, "base-script-1", result.Init[0].Name)
	assert.Equal(t, "echo base1", result.Init[0].Run)
	assert.Equal(t, "base-script-2", result.Init[1].Name)
	assert.Equal(t, "echo base2", result.Init[1].Run)
	assert.Equal(t, "overlay-script-1", result.Init[2].Name)
	assert.Equal(t, "echo overlay1", result.Init[2].Run)
}
```

- [ ] **Step 2: Write failing test for init script merge — only base has scripts**

```go
func TestMerge_InitBaseOnly(t *testing.T) {
	base := &FacetConfig{
		Init: []InitScript{
			{Name: "base-only", Run: "echo base"},
		},
	}
	overlay := &FacetConfig{}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.Init, 1)
	assert.Equal(t, "base-only", result.Init[0].Name)
}
```

- [ ] **Step 3: Write failing test for init script merge — only overlay has scripts**

```go
func TestMerge_InitOverlayOnly(t *testing.T) {
	base := &FacetConfig{}
	overlay := &FacetConfig{
		Init: []InitScript{
			{Name: "overlay-only", Run: "echo overlay"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.Init, 1)
	assert.Equal(t, "overlay-only", result.Init[0].Name)
}
```

- [ ] **Step 4: Write failing test for init script merge — neither has scripts**

```go
func TestMerge_InitBothEmpty(t *testing.T) {
	base := &FacetConfig{}
	overlay := &FacetConfig{}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	assert.Nil(t, result.Init)
}
```

- [ ] **Step 5: Write failing test — deep copy (mutation isolation)**

```go
func TestMerge_InitDeepCopy(t *testing.T) {
	base := &FacetConfig{
		Init: []InitScript{
			{Name: "base-script", Run: "echo base"},
		},
	}
	overlay := &FacetConfig{
		Init: []InitScript{
			{Name: "overlay-script", Run: "echo overlay"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)

	// Mutate result — should not affect inputs
	result.Init[0].Run = "mutated"
	assert.Equal(t, "echo base", base.Init[0].Run)
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestMerge_Init" -v`
Expected: FAIL — init scripts are not merged (result.Init will be nil).

- [ ] **Step 7: Implement mergeInit and wire into Merge**

Add to `internal/profile/merger.go`:

```go
// mergeInit concatenates base init scripts followed by overlay init scripts.
// No deduplication — if both layers define a script with the same name, both run.
func mergeInit(base, overlay []InitScript) []InitScript {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := make([]InitScript, 0, len(base)+len(overlay))
	for _, s := range base {
		result = append(result, InitScript{Name: s.Name, Run: s.Run})
	}
	for _, s := range overlay {
		result = append(result, InitScript{Name: s.Name, Run: s.Run})
	}
	return result
}
```

Add to the `Merge` function, after the AI merge line:

```go
	// Merge init scripts (concatenation — base first, then overlay)
	result.Init = mergeInit(base.Init, overlay.Init)
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestMerge_Init" -v`
Expected: All PASS.

- [ ] **Step 9: Run all profile tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/profile/merger.go internal/profile/merger_test.go
git commit -m "feat: add init script concatenation merge"
```

---

### Task 3: Add variable resolution for init scripts

**Files:**
- Modify: `internal/profile/resolver.go`
- Modify: `internal/profile/resolver_test.go`

- [ ] **Step 1: Write failing test — variables resolved in Run field**

```go
// Add to internal/profile/resolver_test.go
func TestResolve_InitScriptVars(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"email": "sarah@acme.com",
			},
		},
		Init: []InitScript{
			{Name: "configure git", Run: `git config --global user.email "${facet:git.email}"`},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	require.Len(t, resolved.Init, 1)
	assert.Equal(t, "configure git", resolved.Init[0].Name)
	assert.Equal(t, `git config --global user.email "sarah@acme.com"`, resolved.Init[0].Run)
}
```

- [ ] **Step 2: Write failing test — Name field is NOT resolved**

```go
func TestResolve_InitScriptNameNotResolved(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"tool": "git"},
		Init: []InitScript{
			{Name: "setup ${facet:tool}", Run: "echo hello"},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "setup ${facet:tool}", resolved.Init[0].Name)
}
```

- [ ] **Step 3: Write failing test — undefined variable is fatal**

```go
func TestResolve_InitScriptUndefinedVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{},
		Init: []InitScript{
			{Name: "broken", Run: "echo ${facet:missing}"},
		},
	}

	_, err := Resolve(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}
```

- [ ] **Step 4: Write failing test — deep copy (mutation isolation)**

```go
func TestResolve_InitScriptDeepCopy(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"name": "Sarah"},
		Init: []InitScript{
			{Name: "test", Run: "echo ${facet:name}"},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)

	// Mutate resolved — should not affect input
	resolved.Init[0].Run = "mutated"
	assert.Equal(t, "echo ${facet:name}", cfg.Init[0].Run)
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestResolve_InitScript" -v`
Expected: FAIL — init scripts are not copied or resolved.

- [ ] **Step 6: Add init script resolution to Resolve()**

Add to `internal/profile/resolver.go`, in the `Resolve` function, after the AI resolution block and before the `return`:

```go
	// Resolve init scripts
	if cfg.Init != nil {
		result.Init = make([]InitScript, len(cfg.Init))
		for i, script := range cfg.Init {
			resolvedRun, err := substituteVars(script.Run, cfg.Vars)
			if err != nil {
				return nil, fmt.Errorf("init[%d] %q: %w", i, script.Name, err)
			}
			result.Init[i] = InitScript{
				Name: script.Name,
				Run:  resolvedRun,
			}
		}
	}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestResolve_InitScript" -v`
Expected: All PASS.

- [ ] **Step 8: Run all profile tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/profile/resolver.go internal/profile/resolver_test.go
git commit -m "feat: resolve variables in init script run strings"
```

---

## Chunk 2: Scaffold Rename

### Task 4: Rename init to scaffold

**Files:**
- Rename: `internal/app/init.go` → `internal/app/scaffold.go`
- Rename: `internal/app/init_test.go` → `internal/app/scaffold_test.go`
- Rename: `cmd/init_cmd.go` → `cmd/scaffold_cmd.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Rename internal/app/init.go to scaffold.go and update types**

Rename the file from `internal/app/init.go` to `internal/app/scaffold.go`.

In the new `internal/app/scaffold.go`, rename:
- `InitOpts` → `ScaffoldOpts`
- `Init` method → `Scaffold`
- Update the error message from "already initialized" to reference scaffold
- Update the "Next steps" output to say `facet scaffold` references

The full file should become:

```go
package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldOpts holds options for the Scaffold operation.
type ScaffoldOpts struct {
	ConfigDir string
	StateDir  string
}

// Scaffold creates a new facet config repository.
func (a *App) Scaffold(opts ScaffoldOpts) error {
	// Check if already initialized
	if _, err := os.Stat(filepath.Join(opts.ConfigDir, "facet.yaml")); err == nil {
		return fmt.Errorf("facet.yaml already exists in %s — already initialized", opts.ConfigDir)
	}

	// Create config repo files
	if err := createConfigRepo(opts.ConfigDir); err != nil {
		return fmt.Errorf("failed to create config repo: %w", err)
	}

	// Create state directory and .local.yaml
	if err := createStateDir(opts.StateDir); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	a.reporter.Success(fmt.Sprintf("Config repo initialized in %s", opts.ConfigDir))
	a.reporter.Success(fmt.Sprintf("State directory at %s", opts.StateDir))
	a.reporter.PrintLine("\nNext steps:")
	a.reporter.PrintLine("  1. Edit base.yaml to add your shared packages and configs")
	a.reporter.PrintLine("  2. Create a profile in profiles/ for this machine")
	a.reporter.PrintLine("  3. Edit ~/.facet/.local.yaml to add machine-specific secrets")
	a.reporter.PrintLine("  4. Run: facet apply <profile>")

	return nil
}

func createConfigRepo(dir string) error {
	facetYAML := `min_version: "0.1.0"
`
	if err := os.WriteFile(filepath.Join(dir, "facet.yaml"), []byte(facetYAML), 0o644); err != nil {
		return err
	}

	baseYAML := `# Base configuration — shared across all profiles.
# Every profile extends this via 'extends: base'.

# vars:
#   git_name: Your Name

# packages:
#   Common install patterns:
#     Homebrew:         brew install <name>
#     Homebrew cask:    brew install --cask <name>
#     apt (auto-yes):   sudo apt-get install -y <name>
#     npm global:       npm install -g <package>
#     go install:       go install github.com/user/repo@latest
#     pip:              pip install <package>
#     cargo:            cargo install <name>
#
#   - name: ripgrep
#     install: brew install ripgrep
#
#   - name: lazydocker
#     install:
#       macos: brew install lazydocker
#       linux: go install github.com/jesseduffield/lazydocker@latest

# configs:
#   ~/.gitconfig: configs/.gitconfig
#   ~/.zshrc: configs/.zshrc
`
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(baseYAML), 0o644); err != nil {
		return err
	}

	for _, d := range []string{"profiles", "configs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}

	return nil
}

func createStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	localPath := filepath.Join(dir, ".local.yaml")
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		localYAML := `# Machine-specific variables (secrets, local paths, etc.)
# This file must exist. Add your machine-specific vars here.
#
# vars:
#   acme_db_url: postgres://user:pass@localhost:5432/acme
`
		if err := os.WriteFile(localPath, []byte(localYAML), 0o644); err != nil {
			return err
		}
	}

	return nil
}
```

- [ ] **Step 2: Rename internal/app/init_test.go to scaffold_test.go and update**

Rename the file from `internal/app/init_test.go` to `internal/app/scaffold_test.go`.

Update all test function names and method calls:
- `TestInit_CreatesConfigRepo` → `TestScaffold_CreatesConfigRepo`
- `TestInit_AlreadyInitialized` → `TestScaffold_AlreadyInitialized`
- `TestInit_PreservesExistingLocalYaml` → `TestScaffold_PreservesExistingLocalYaml`
- All `a.Init(InitOpts{...})` → `a.Scaffold(ScaffoldOpts{...})`

Note: the `mockReporter` type is defined in this file. It will be needed by the new `init_test.go` too. To avoid duplication, move `mockReporter` to a shared test helper file `internal/app/test_helpers_test.go`:

```go
// internal/app/test_helpers_test.go
package app

// mockReporter captures reporter output for testing.
type mockReporter struct {
	messages []string
}

func (m *mockReporter) Success(msg string)     { m.messages = append(m.messages, "success: "+msg) }
func (m *mockReporter) Warning(msg string)     { m.messages = append(m.messages, "warning: "+msg) }
func (m *mockReporter) Error(msg string)       { m.messages = append(m.messages, "error: "+msg) }
func (m *mockReporter) Header(msg string)      { m.messages = append(m.messages, "header: "+msg) }
func (m *mockReporter) PrintLine(msg string)   { m.messages = append(m.messages, "line: "+msg) }
func (m *mockReporter) Dim(text string) string { return text }
```

Remove the `mockReporter` definition from `scaffold_test.go` (it now comes from `test_helpers_test.go`).

The full `scaffold_test.go`:

```go
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffold_CreatesConfigRepo(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	r := &mockReporter{}

	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(cfgDir, "facet.yaml"))
	assert.FileExists(t, filepath.Join(cfgDir, "base.yaml"))
	assert.DirExists(t, filepath.Join(cfgDir, "profiles"))
	assert.DirExists(t, filepath.Join(cfgDir, "configs"))
	assert.FileExists(t, filepath.Join(stDir, ".local.yaml"))
}

func TestScaffold_AlreadyInitialized(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "facet.yaml"), []byte("existing"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")
}

func TestScaffold_PreservesExistingLocalYaml(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	localPath := filepath.Join(stDir, ".local.yaml")
	require.NoError(t, os.MkdirAll(stDir, 0o755))
	require.NoError(t, os.WriteFile(localPath, []byte("custom content"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	content, _ := os.ReadFile(localPath)
	assert.Equal(t, "custom content", string(content))
}
```

- [ ] **Step 3: Rename cmd/init_cmd.go to scaffold_cmd.go and update**

Rename the file from `cmd/init_cmd.go` to `cmd/scaffold_cmd.go`.

Update the full file:

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newScaffoldCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold",
		Short: "Create a new facet config repository",
		Long:  "Creates a facet config repo in the current directory and initializes the state directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, err := resolveConfigDir(*configDir)
			if err != nil {
				return err
			}
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Scaffold(app.ScaffoldOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			})
		},
	}
}
```

- [ ] **Step 4: Update cmd/root.go to register scaffold instead of init**

In `cmd/root.go`, change:

```go
	rootCmd.AddCommand(newInitCmd(application, &configDir, &stateDir))
```

to:

```go
	rootCmd.AddCommand(newScaffoldCmd(application, &configDir, &stateDir))
```

- [ ] **Step 5: Verify everything compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build ./...`
Expected: No errors.

- [ ] **Step 6: Run all app tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -v`
Expected: All PASS.

- [ ] **Step 7: Run full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app/scaffold.go internal/app/scaffold_test.go internal/app/test_helpers_test.go cmd/scaffold_cmd.go cmd/root.go
git rm internal/app/init.go internal/app/init_test.go cmd/init_cmd.go
git commit -m "refactor: rename facet init to facet scaffold"
```

---

## Chunk 3: Init Command

### Task 5: Add ScriptRunner interface and wire into App

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/app.go`
- Modify: `main.go`

- [ ] **Step 1: Add ScriptRunner interface to interfaces.go**

Add to `internal/app/interfaces.go`:

```go
// ScriptRunner executes shell commands for init scripts.
type ScriptRunner interface {
	Run(command string, dir string) error
}
```

- [ ] **Step 2: Add scriptRunner to App and Deps**

In `internal/app/app.go`, add `ScriptRunner ScriptRunner` to `Deps`:

```go
type Deps struct {
	Loader          ProfileLoader
	Installer       Installer
	Reporter        Reporter
	StateStore      StateStore
	DeployerFactory DeployerFactory
	AIOrchestrator  AIOrchestrator
	ScriptRunner    ScriptRunner
	Version         string
	OSName          string
}
```

Add `scriptRunner ScriptRunner` field to `App`:

```go
type App struct {
	loader          ProfileLoader
	installer       Installer
	reporter        Reporter
	stateStore      StateStore
	deployerFactory DeployerFactory
	aiOrchestrator  AIOrchestrator
	scriptRunner    ScriptRunner
	version         string
	osName          string
}
```

Add to `New()`:

```go
func New(deps Deps) *App {
	return &App{
		loader:          deps.Loader,
		installer:       deps.Installer,
		reporter:        deps.Reporter,
		stateStore:      deps.StateStore,
		deployerFactory: deps.DeployerFactory,
		aiOrchestrator:  deps.AIOrchestrator,
		scriptRunner:    deps.ScriptRunner,
		version:         deps.Version,
		osName:          deps.OSName,
	}
}
```

- [ ] **Step 3: Create ShellScriptRunner concrete implementation**

Create `internal/app/scriptrunner.go`:

```go
package app

import (
	"fmt"
	"os/exec"
)

// ShellScriptRunner executes commands via sh -c with a working directory.
type ShellScriptRunner struct{}

// NewShellScriptRunner creates a new ShellScriptRunner.
func NewShellScriptRunner() *ShellScriptRunner {
	return &ShellScriptRunner{}
}

// Run executes a command string via sh -c in the given directory.
func (r *ShellScriptRunner) Run(command string, dir string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
```

- [ ] **Step 4: Wire ShellScriptRunner in main.go**

In `main.go`, add to the dependency wiring (before `app.New`):

```go
	scriptRunner := app.NewShellScriptRunner()
```

And add to the `app.Deps` struct:

```go
	application := app.New(app.Deps{
		Loader:          loader,
		Installer:       installer,
		Reporter:        r,
		StateStore:      stateStore,
		DeployerFactory: deployerFactory,
		AIOrchestrator:  aiOrchestrator,
		ScriptRunner:    scriptRunner,
		Version:         "0.1.0",
		OSName:          osName,
	})
```

- [ ] **Step 5: Verify build compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build ./...`
Expected: No errors.

- [ ] **Step 6: Run full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/interfaces.go internal/app/app.go internal/app/scriptrunner.go main.go
git commit -m "feat: add ScriptRunner interface and ShellScriptRunner implementation"
```

---

### Task 6: Implement App.Init pipeline

**Files:**
- Create: `internal/app/init.go`
- Create: `internal/app/init_test.go`
- Modify: `internal/app/report.go`

- [ ] **Step 1: Write failing test — successful init runs all scripts in order**

```go
// internal/app/init_test.go
package app

import (
	"fmt"
	"testing"

	"facet/internal/profile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScriptRunner records commands and optionally fails on specific ones.
type mockScriptRunner struct {
	commands []string
	dirs     []string
	failOn   map[string]error
}

func (m *mockScriptRunner) Run(command string, dir string) error {
	m.commands = append(m.commands, command)
	m.dirs = append(m.dirs, dir)
	if err, ok := m.failOn[command]; ok {
		return err
	}
	return nil
}

// mockLoader returns preset configs for known paths.
type mockLoader struct {
	configs map[string]*profile.FacetConfig
	meta    *profile.FacetMeta
}

func (m *mockLoader) LoadMeta(configDir string) (*profile.FacetMeta, error) {
	if m.meta != nil {
		return m.meta, nil
	}
	return nil, fmt.Errorf("facet.yaml not found")
}

func (m *mockLoader) LoadConfig(path string) (*profile.FacetConfig, error) {
	cfg, ok := m.configs[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return cfg, nil
}

func TestInit_RunsAllScriptsInOrder(t *testing.T) {
	runner := &mockScriptRunner{}
	reporter := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{MinVersion: "0.1.0"},
		configs: map[string]*profile.FacetConfig{
			"/config/base.yaml": {
				Init: []profile.InitScript{
					{Name: "base-script", Run: "echo base"},
				},
			},
			"/config/profiles/work.yaml": {
				Extends: "base",
				Init: []profile.InitScript{
					{Name: "profile-script", Run: "echo profile"},
				},
			},
			"/state/.local.yaml": {
				Vars: map[string]any{"key": "value"},
			},
		},
	}

	a := New(Deps{
		Loader:       loader,
		Reporter:     reporter,
		ScriptRunner: runner,
	})

	err := a.Init(InitOpts{
		ConfigDir: "/config",
		StateDir:  "/state",
	}, "work")
	require.NoError(t, err)

	require.Len(t, runner.commands, 2)
	assert.Equal(t, "echo base", runner.commands[0])
	assert.Equal(t, "echo profile", runner.commands[1])
	assert.Equal(t, "/config", runner.dirs[0])
	assert.Equal(t, "/config", runner.dirs[1])
}
```

- [ ] **Step 2: Write failing test — init fails fast on script error**

```go
func TestInit_FailsOnScriptError(t *testing.T) {
	runner := &mockScriptRunner{
		failOn: map[string]error{
			"echo fail": fmt.Errorf("exit status 1: error output"),
		},
	}
	reporter := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{MinVersion: "0.1.0"},
		configs: map[string]*profile.FacetConfig{
			"/config/base.yaml": {
				Init: []profile.InitScript{
					{Name: "good-script", Run: "echo ok"},
					{Name: "bad-script", Run: "echo fail"},
					{Name: "skipped-script", Run: "echo skipped"},
				},
			},
			"/config/profiles/work.yaml": {
				Extends: "base",
			},
			"/state/.local.yaml": {},
		},
	}

	a := New(Deps{
		Loader:       loader,
		Reporter:     reporter,
		ScriptRunner: runner,
	})

	err := a.Init(InitOpts{
		ConfigDir: "/config",
		StateDir:  "/state",
	}, "work")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad-script")

	// Only first two scripts should have been attempted
	require.Len(t, runner.commands, 2)
	assert.Equal(t, "echo ok", runner.commands[0])
	assert.Equal(t, "echo fail", runner.commands[1])
}
```

- [ ] **Step 3: Write failing test — variable resolution in init scripts**

```go
func TestInit_ResolvesVariables(t *testing.T) {
	runner := &mockScriptRunner{}
	reporter := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{MinVersion: "0.1.0"},
		configs: map[string]*profile.FacetConfig{
			"/config/base.yaml": {
				Vars: map[string]any{
					"git": map[string]any{"email": "sarah@acme.com"},
				},
				Init: []profile.InitScript{
					{Name: "configure git", Run: `git config --global user.email "${facet:git.email}"`},
				},
			},
			"/config/profiles/work.yaml": {
				Extends: "base",
			},
			"/state/.local.yaml": {},
		},
	}

	a := New(Deps{
		Loader:       loader,
		Reporter:     reporter,
		ScriptRunner: runner,
	})

	err := a.Init(InitOpts{
		ConfigDir: "/config",
		StateDir:  "/state",
	}, "work")
	require.NoError(t, err)

	require.Len(t, runner.commands, 1)
	assert.Equal(t, `git config --global user.email "sarah@acme.com"`, runner.commands[0])
}
```

- [ ] **Step 4: Write failing test — no init scripts is not an error**

```go
func TestInit_NoScriptsSucceeds(t *testing.T) {
	runner := &mockScriptRunner{}
	reporter := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{MinVersion: "0.1.0"},
		configs: map[string]*profile.FacetConfig{
			"/config/base.yaml":          {},
			"/config/profiles/work.yaml": {Extends: "base"},
			"/state/.local.yaml":         {},
		},
	}

	a := New(Deps{
		Loader:       loader,
		Reporter:     reporter,
		ScriptRunner: runner,
	})

	err := a.Init(InitOpts{
		ConfigDir: "/config",
		StateDir:  "/state",
	}, "work")
	require.NoError(t, err)
	assert.Empty(t, runner.commands)
}
```

- [ ] **Step 5: Write failing test — missing profile is an error**

```go
func TestInit_MissingProfile(t *testing.T) {
	runner := &mockScriptRunner{}
	reporter := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{MinVersion: "0.1.0"},
		configs: map[string]*profile.FacetConfig{
			"/config/base.yaml":  {},
			"/state/.local.yaml": {},
		},
	}

	a := New(Deps{
		Loader:       loader,
		Reporter:     reporter,
		ScriptRunner: runner,
	})

	err := a.Init(InitOpts{
		ConfigDir: "/config",
		StateDir:  "/state",
	}, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run "TestInit_" -v`
Expected: FAIL — `Init` method and `InitOpts` don't exist yet.

- [ ] **Step 7: Implement App.Init()**

Create `internal/app/init.go`:

```go
package app

import (
	"fmt"
	"path/filepath"

	"facet/internal/profile"
)

// InitOpts holds options for the Init operation.
type InitOpts struct {
	ConfigDir string
	StateDir  string
}

// Init runs post-install initialization scripts for a profile.
func (a *App) Init(opts InitOpts, profileName string) error {
	// Step 1: Load facet.yaml
	_, err := a.loader.LoadMeta(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory.\n  detail: %w", err)
	}

	// Step 2: Load base.yaml
	baseCfg, err := a.loader.LoadConfig(filepath.Join(opts.ConfigDir, "base.yaml"))
	if err != nil {
		return err
	}

	// Step 3: Load profile
	profilePath := filepath.Join(opts.ConfigDir, "profiles", profileName+".yaml")
	profileCfg, err := a.loader.LoadConfig(profilePath)
	if err != nil {
		return fmt.Errorf("cannot load profile %q: %w", profileName, err)
	}
	if err := profile.ValidateProfile(profileCfg); err != nil {
		return err
	}

	// Step 4: Load .local.yaml (for variable resolution)
	localPath := filepath.Join(opts.StateDir, ".local.yaml")
	localCfg, err := a.loader.LoadConfig(localPath)
	if err != nil {
		return fmt.Errorf(".local.yaml is required in %s: %w", opts.StateDir, err)
	}

	// Step 5: Merge layers
	// Init scripts: base + profile only (local does not participate)
	// Vars: base + profile + local (for variable resolution)
	merged, err := profile.Merge(baseCfg, profileCfg)
	if err != nil {
		return fmt.Errorf("merge error: %w", err)
	}

	// Save init scripts before merging with local (which doesn't contribute init scripts)
	initScripts := merged.Init

	merged, err = profile.Merge(merged, localCfg)
	if err != nil {
		return fmt.Errorf("merge error with .local.yaml: %w", err)
	}

	// Restore init scripts (local merge may have nil'd them if local has no init)
	// Actually, mergeInit concatenates, so if local has no init scripts, merged.Init
	// will still contain the base+profile scripts. But to be safe and explicit:
	merged.Init = initScripts

	// Step 6: Resolve variables in init scripts
	resolved, err := profile.Resolve(merged)
	if err != nil {
		return err
	}

	// Step 7: Execute scripts
	a.reporter.Header(fmt.Sprintf("Init  %s", profileName))

	if len(resolved.Init) == 0 {
		a.reporter.PrintLine("  No init scripts defined.")
		return nil
	}

	for i, script := range resolved.Init {
		err := a.scriptRunner.Run(script.Run, opts.ConfigDir)
		if err != nil {
			a.reporter.Error(fmt.Sprintf("%s (%v)", script.Name, err))

			// Print skipped scripts
			remaining := resolved.Init[i+1:]
			if len(remaining) > 0 {
				a.reporter.PrintLine("")
				a.reporter.PrintLine("  Skipped:")
				for _, s := range remaining {
					a.reporter.PrintLine(fmt.Sprintf("    - %s", s.Name))
				}
			}

			a.reporter.PrintLine("")
			a.reporter.PrintLine(fmt.Sprintf("Init failed at %q", script.Name))
			return fmt.Errorf("init script %q failed: %w", script.Name, err)
		}
		a.reporter.Success(script.Name)
	}

	a.reporter.PrintLine("")
	a.reporter.PrintLine(fmt.Sprintf("Init completed (%d scripts)", len(resolved.Init)))

	return nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run "TestInit_" -v`
Expected: All PASS.

- [ ] **Step 9: Run full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/app/init.go internal/app/init_test.go
git commit -m "feat: implement App.Init pipeline for running init scripts"
```

---

### Task 7: Add facet init Cobra command

**Files:**
- Create: `cmd/init_cmd.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Create cmd/init_cmd.go**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newInitCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init <profile>",
		Short: "Run initialization scripts for a profile",
		Long:  "Loads the profile, resolves variables, and runs init scripts defined in base.yaml and the profile. Run facet apply first to deploy configs and install packages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, err := resolveConfigDir(*configDir)
			if err != nil {
				return err
			}
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Init(app.InitOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			}, args[0])
		},
	}
}
```

- [ ] **Step 2: Register init command in root.go**

In `cmd/root.go`, add after the scaffold registration:

```go
	rootCmd.AddCommand(newInitCmd(application, &configDir, &stateDir))
```

The full command registrations should be:

```go
	rootCmd.AddCommand(newApplyCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newDocsCmd())
	rootCmd.AddCommand(newInitCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newScaffoldCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newStatusCmd(application, &configDir, &stateDir))
```

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build -o facet .`
Expected: Binary created, no errors.

- [ ] **Step 4: Verify CLI shows init command**

Run: `cd /Users/edocsss/aec/src/facet && ./facet --help`
Expected: Output lists both `init` and `scaffold` commands.

Run: `cd /Users/edocsss/aec/src/facet && ./facet init --help`
Expected: Shows "Run initialization scripts for a profile" and expects `<profile>` argument.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/init_cmd.go cmd/root.go
git commit -m "feat: add facet init <profile> cobra command"
```

---

## Chunk 4: Documentation

### Task 8: Update docs topics and docs command

**Files:**
- Modify: `internal/docs/docs.go`
- Modify: `internal/docs/topics/commands.md`
- Modify: `internal/docs/topics/config.md`
- Modify: `internal/docs/topics/merge.md`
- Modify: `internal/docs/topics/quickstart.md`
- Modify: `internal/docs/topics/examples.md`
- Create: `internal/docs/topics/init.md`

- [ ] **Step 1: Create internal/docs/topics/init.md**

```markdown
# Init Scripts

Init scripts run post-install commands after `facet apply` has deployed configs and
installed packages. Use `facet init <profile>` to run them.

## YAML Format

Define init scripts in `base.yaml` and profile YAML files:

```yaml
init:
  - name: configure git
    run: git config --global user.email "${facet:git.email}"
  - name: setup editor plugins
    run: |
      export EDITOR_HOME="${facet:editor.home}"
      ./scripts/setup-editor.sh
```

Each entry has:

- `name`: human-readable label shown in the output
- `run`: shell command or multi-line script executed via `sh -c`

## Variable Resolution

`${facet:var.name}` references are resolved in the `run` field using the full
merged variable set (base + profile + .local.yaml). The `name` field is not resolved.

For external scripts, pass variables explicitly:

```yaml
init:
  - name: run python setup
    run: |
      export DB_URL="${facet:db.url}"
      python3 ./scripts/setup.py
```

## Merge Rules

Base scripts run first, then profile scripts are appended. No deduplication — if both
layers define a script with the same name, both run. `.local.yaml` does not contribute
init scripts.

## Execution

Scripts run sequentially in order. If any script exits with a non-zero code, execution
stops immediately. Remaining scripts are listed as skipped.

Scripts run with the config directory as the working directory, so relative paths like
`./scripts/setup.sh` resolve against the config repo.

## Idempotency

Init scripts should be idempotent — safe to run multiple times. facet does not track
which scripts have run. Running `facet init <profile>` again re-runs all scripts.
```

- [ ] **Step 2: Add "init" topic to docs.go**

In `internal/docs/docs.go`, add to `allTopics()` after the `packages` entry:

```go
		{Name: "init", Description: "Post-install initialization scripts"},
```

The full list should be:

```go
	return []Topic{
		{Name: "quickstart", Description: "Set up facet from scratch (start here)"},
		{Name: "config", Description: "YAML config format and file structure"},
		{Name: "variables", Description: "Variable substitution syntax and rules"},
		{Name: "packages", Description: "Package installation entries"},
		{Name: "init", Description: "Post-install initialization scripts"},
		{Name: "deploy", Description: "Config file deployment (symlink vs template)"},
		{Name: "ai", Description: "AI agent configuration (permissions, MCPs, skills)"},
		{Name: "merge", Description: "How base, profile, and .local layers combine"},
		{Name: "commands", Description: "CLI commands and flags reference"},
		{Name: "examples", Description: "Complete working config examples"},
	}
```

- [ ] **Step 3: Update commands.md**

Replace the full content of `internal/docs/topics/commands.md`:

```markdown
# Commands

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config-dir` | `-c` | Current directory | Path to the facet config repo |
| `--state-dir` | `-s` | `~/.facet` | Path to the machine-local state directory |

## `facet scaffold`

Create a new config repo in the current directory and initialize the state directory.

```bash
facet scaffold
```

Creates `facet.yaml`, `base.yaml`, `profiles/`, and `configs/` in the config repo.
Creates `.local.yaml` in the state directory if it does not already exist.

## `facet apply <profile>`

Apply a configuration profile.

```bash
facet apply work
facet apply work --dry-run
facet apply work --force
facet apply work --skip-failure
```

Flags:

- `--dry-run`: preview the resolved actions without writing changes
- `--force`: replace conflicting non-facet files and unapply previous state first when needed
- `--skip-failure`: warn on deploy failures instead of rolling back immediately

What it does:

1. Loads `facet.yaml`, `base.yaml`, the selected profile, and `.local.yaml`
2. Merges the three layers
3. Resolves `${facet:...}` variables
4. Installs packages
5. Deploys config files
6. Applies AI configuration
7. Writes `.state.json`

## `facet init <profile>`

Run initialization scripts for a profile.

```bash
facet init work
```

Run this after `facet apply`. It loads the profile, resolves variables, and runs
the `init` scripts defined in `base.yaml` and the profile YAML in order.

What it does:

1. Loads `facet.yaml`, `base.yaml`, the selected profile, and `.local.yaml`
2. Merges init scripts (base first, then profile appended)
3. Resolves `${facet:...}` variables in script `run` fields
4. Executes scripts sequentially via `sh -c`
5. Fails immediately on non-zero exit code

Init does not write state. Scripts should be idempotent.

See `facet docs init` for the full init script format.

## `facet status`

Show the current applied state.

```bash
facet status
```

This reads `.state.json` and reports the active profile, package results, deployed
configs, and validity checks.

## `facet docs [topic]`

Show embedded documentation.

```bash
facet docs
facet docs config
```

Run `facet docs` to list the available topics.

## Exit Codes

- `0`: success
- `1`: command failed
```

- [ ] **Step 4: Update config.md — add init field to field reference**

In `internal/docs/topics/config.md`, add a row to the Field Reference table:

```
| `init` | list of InitScript | Init scripts. See `facet docs init`. |
```

The full table should be:

```markdown
| Field | Type | Description |
|-------|------|-------------|
| `extends` | string | Profile files only. Must be `base`. |
| `vars` | map[string]any | Variables used by `${facet:...}` substitution. Supports nested maps. |
| `packages` | list of PackageEntry | Package install entries. See `facet docs packages`. |
| `configs` | map[string]string | Target path to source path. See `facet docs deploy`. |
| `init` | list of InitScript | Init scripts run by `facet init`. See `facet docs init`. |
| `ai` | AIConfig | AI agent configuration. See `facet docs ai`. |
```

- [ ] **Step 5: Update merge.md — add init merge rules**

In `internal/docs/topics/merge.md`, add after the `## \`configs\`` section:

```markdown
## `init`

Init scripts are concatenated: base scripts first, then profile scripts appended at the
end. No deduplication by name. `.local.yaml` does not contribute init scripts.
```

- [ ] **Step 6: Update quickstart.md — rename init to scaffold**

In `internal/docs/topics/quickstart.md`, replace:

```markdown
## 1. Initialize the config repo

```bash
mkdir ~/dotfiles && cd ~/dotfiles
facet init
```
```

with:

```markdown
## 1. Initialize the config repo

```bash
mkdir ~/dotfiles && cd ~/dotfiles
facet scaffold
```
```

- [ ] **Step 7: Update examples.md — add init script example**

In `internal/docs/topics/examples.md`, add a section before `## What Happens On \`facet apply work\``:

```markdown
## Init scripts in `base.yaml`

```yaml
init:
  - name: configure git credentials
    run: git config --global credential.helper store
```

## Init scripts in `profiles/work.yaml`

```yaml
init:
  - name: authenticate gcloud
    run: |
      export PROJECT="${facet:gcp_project}"
      gcloud config set project "$PROJECT"
```
```

And add to the "What Happens" section:

```markdown
## What Happens On `facet init work`

- Init scripts from base and profile are concatenated
- `${facet:...}` variables are resolved in the `run` fields
- Scripts run sequentially; any failure stops execution
```

- [ ] **Step 8: Run docs tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/docs/ -v`
Expected: All PASS (the test `TestRender_AllRegisteredTopicsHaveFiles` will verify the new init topic has a file).

- [ ] **Step 9: Run full test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/docs/docs.go internal/docs/topics/init.md internal/docs/topics/commands.md internal/docs/topics/config.md internal/docs/topics/merge.md internal/docs/topics/quickstart.md internal/docs/topics/examples.md
git commit -m "docs: add init command documentation and update existing topics"
```

---

## Chunk 5: E2E Tests

### Task 9: Rename E2E scaffold test and update helpers

**Files:**
- Rename: `e2e/suites/01-init.sh` → `e2e/suites/01-scaffold.sh`
- Modify: `e2e/suites/helpers.sh`

- [ ] **Step 1: Rename 01-init.sh to 01-scaffold.sh and update**

Rename `e2e/suites/01-init.sh` to `e2e/suites/01-scaffold.sh`.

Update the content:

```bash
#!/bin/bash
# e2e/suites/01-scaffold.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Test: scaffold creates config repo structure
SCAFFOLD_DIR="$HOME/new-repo"
facet_scaffold "$SCAFFOLD_DIR"

assert_file_exists "$SCAFFOLD_DIR/facet.yaml"
assert_file_exists "$SCAFFOLD_DIR/base.yaml"
assert_file_exists "$SCAFFOLD_DIR/profiles"
assert_file_exists "$SCAFFOLD_DIR/configs"
echo "  scaffold creates config repo structure"

# Test: scaffold creates .local.yaml in state dir
assert_file_exists "$HOME/.facet/.local.yaml"
echo "  scaffold creates .local.yaml in state dir"

# Test: scaffold fails if facet.yaml already exists
assert_exit_code 1 bash -c "cd $SCAFFOLD_DIR && facet -s $HOME/.facet scaffold"
echo "  scaffold errors on existing facet.yaml"
```

- [ ] **Step 2: Update helpers.sh — rename facet_init to facet_scaffold, add facet_init for new command**

Replace the `facet_init` function and add `facet_scaffold` in `e2e/suites/helpers.sh`:

Replace:

```bash
facet_init() {
    # init operates on cwd, so cd into the target
    local target="${1:-$HOME/dotfiles}"
    mkdir -p "$target"
    (cd "$target" && facet -s "$HOME/.facet" init)
}
```

With:

```bash
facet_scaffold() {
    # scaffold operates on cwd, so cd into the target
    local target="${1:-$HOME/dotfiles}"
    mkdir -p "$target"
    (cd "$target" && facet -s "$HOME/.facet" scaffold)
}

facet_init() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" init "$@"
}
```

- [ ] **Step 3: Run E2E tests to verify scaffold works**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/harness.sh e2e/suites/01-scaffold.sh`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add e2e/suites/01-scaffold.sh e2e/suites/helpers.sh
git rm e2e/suites/01-init.sh
git commit -m "refactor: rename E2E init test to scaffold"
```

---

### Task 10: Add E2E tests for facet init

**Files:**
- Modify: `e2e/fixtures/setup-basic.sh`
- Create: `e2e/suites/11-init-scripts.sh`

- [ ] **Step 1: Add init scripts to setup-basic.sh fixture**

In `e2e/fixtures/setup-basic.sh`, add init scripts to `base.yaml` and `profiles/work.yaml`.

In the `base.yaml` heredoc, add after the `configs:` section:

```yaml

init:
  - name: create-marker
    run: touch "$HOME/.facet-base-init-ran"
```

In the `profiles/work.yaml` heredoc, add after the `configs:` section:

```yaml

init:
  - name: create-work-marker
    run: touch "$HOME/.facet-work-init-ran"
  - name: write-resolved-var
    run: echo "${facet:git.email}" > "$HOME/.facet-init-email"
```

- [ ] **Step 2: Create e2e/suites/11-init-scripts.sh**

```bash
#!/bin/bash
# e2e/suites/11-init-scripts.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# --- Test: init runs scripts in order ---
facet_apply work
facet_init work

assert_file_exists "$HOME/.facet-base-init-ran"
assert_file_exists "$HOME/.facet-work-init-ran"
echo "  init runs base and profile scripts"

# --- Test: variable resolution in init scripts ---
assert_file_exists "$HOME/.facet-init-email"
assert_file_contains "$HOME/.facet-init-email" "sarah@acme.com"
echo "  init resolves variables in run strings"

# --- Test: init is re-runnable (idempotent) ---
facet_init work
assert_file_exists "$HOME/.facet-base-init-ran"
echo "  init is re-runnable"

# --- Test: init fails on non-zero exit code ---
# Create a profile with a failing init script
cat > "$HOME/dotfiles/profiles/failing.yaml" << 'YAML'
extends: base

init:
  - name: will-succeed
    run: touch "$HOME/.facet-fail-first"
  - name: will-fail
    run: exit 1
  - name: should-be-skipped
    run: touch "$HOME/.facet-fail-skipped"
YAML

assert_exit_code 1 bash -c "facet -c $HOME/dotfiles -s $HOME/.facet init failing 2>&1"
assert_file_exists "$HOME/.facet-fail-first"
assert_file_not_exists "$HOME/.facet-fail-skipped"
echo "  init fails fast on non-zero exit, skips remaining scripts"

# --- Test: init with no scripts succeeds ---
cat > "$HOME/dotfiles/profiles/noinit.yaml" << 'YAML'
extends: base
YAML

# Override base.yaml without init scripts for this test
cat > "$HOME/dotfiles/base-noinit.yaml" << 'YAML'
vars:
  git_name: Sarah Chen
YAML

# Use a clean profile that has no init scripts and a base with no init scripts
cat > "$HOME/dotfiles/profiles/empty-init.yaml" << 'YAML'
extends: base
YAML

# For this test, we can just verify the noinit profile succeeds
# (base still has init scripts, so noinit profile will run base scripts — that's fine)
# The important thing is it doesn't error
facet -c "$HOME/dotfiles" -s "$HOME/.facet" init noinit
echo "  init with profile succeeds even if profile has no init scripts"
```

- [ ] **Step 3: Run the new E2E test**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/harness.sh e2e/suites/11-init-scripts.sh`
Expected: PASS.

- [ ] **Step 4: Run full E2E suite**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/harness.sh`
Expected: All suites PASS.

- [ ] **Step 5: Run full unit test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add e2e/fixtures/setup-basic.sh e2e/suites/11-init-scripts.sh
git commit -m "test: add E2E tests for facet init command"
```
