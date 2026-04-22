# Verbose Progress Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--verbose` / `-v` global flag to `facet` that emits inline stage and item progress during `facet apply`.

**Architecture:** Add `Progress(msg string)` to the `Reporter` interface; the concrete reporter only prints when `verbose=true` (set via `SetVerbose`). A `VerboseSetter` interface in `cmd/root.go` lets the root command configure the reporter after Cobra parses the flag via `PersistentPreRunE`. The `Apply` method calls `Progress` freely at each stage; the reporter silently discards calls when not verbose.

**Tech Stack:** Go 1.21+, Cobra, `github.com/stretchr/testify`

---

## File Map

| Action | File | Change |
|---|---|---|
| Modify | `internal/app/interfaces.go` | Add `Progress(msg string)` to `Reporter` interface |
| Modify | `internal/app/test_helpers_test.go` | Add `Progress` to `mockReporter` |
| Modify | `internal/common/reporter/reporter.go` | Add `verbose bool`, `SetVerbose(bool)`, `Progress(msg string)` |
| Modify | `internal/common/reporter/reporter_test.go` | Tests for `Progress` and `SetVerbose` |
| Modify | `internal/app/apply.go` | Add `Progress` calls at each stage and item |
| Modify | `internal/app/apply_test.go` | Test that Progress messages are emitted |
| Modify | `cmd/root.go` | Add `VerboseSetter` interface, `--verbose`/`-v` flag, `PersistentPreRunE` |
| Modify | `main.go` | Pass `r` as `VerboseSetter` to `NewRootCmd` |
| Modify | `internal/docs/topics/commands.md` | Add `--verbose` to Global Flags and apply section |
| Modify | `README.md` | Add `--verbose` to global flags table and apply examples |
| Modify | `docs/architecture/v1-design-spec.md` | Add `--verbose` to CLI flags section |
| Create | `e2e/suites/16-verbose-flag.sh` | E2E test for verbose output |

---

### Task 1: Extend Reporter interface and update mockReporter

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/test_helpers_test.go`

- [ ] **Step 1: Add `Progress` to the `Reporter` interface**

In `internal/app/interfaces.go`, add `Progress` after `PrintLine`:

```go
// Reporter handles formatted terminal output.
type Reporter interface {
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Header(msg string)
	PrintLine(msg string)
	Dim(text string) string
	Progress(msg string)
}
```

- [ ] **Step 2: Add `Progress` to mockReporter**

In `internal/app/test_helpers_test.go`, add:

```go
func (m *mockReporter) Progress(msg string) { m.messages = append(m.messages, "progress: "+msg) }
```

The full struct after the change:

```go
type mockReporter struct {
	messages []string
}

func (m *mockReporter) Success(msg string)     { m.messages = append(m.messages, "success: "+msg) }
func (m *mockReporter) Warning(msg string)     { m.messages = append(m.messages, "warning: "+msg) }
func (m *mockReporter) Error(msg string)       { m.messages = append(m.messages, "error: "+msg) }
func (m *mockReporter) Header(msg string)      { m.messages = append(m.messages, "header: "+msg) }
func (m *mockReporter) PrintLine(msg string)   { m.messages = append(m.messages, "line: "+msg) }
func (m *mockReporter) Dim(text string) string { return text }
func (m *mockReporter) Progress(msg string)    { m.messages = append(m.messages, "progress: "+msg) }
```

- [ ] **Step 3: Verify the build still compiles**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add internal/app/interfaces.go internal/app/test_helpers_test.go
git commit -m "feat: add Progress to Reporter interface and mock"
```

---

### Task 2: Add verbose support to concrete reporter (TDD)

**Files:**
- Modify: `internal/common/reporter/reporter.go`
- Modify: `internal/common/reporter/reporter_test.go`

- [ ] **Step 1: Write failing tests for `Progress` and `SetVerbose`**

Append to `internal/common/reporter/reporter_test.go`:

```go
func TestReporter_Progress_Verbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)
	r.Progress("Deploying configs")
	assert.Contains(t, buf.String(), "Deploying configs")
}

func TestReporter_Progress_Silent_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	// verbose defaults to false
	r.Progress("Deploying configs")
	assert.Empty(t, buf.String())
}

func TestReporter_SetVerbose_TogglesBehavior(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Progress("before enable") // should be silent
	r.SetVerbose(true)
	r.Progress("after enable") // should print
	r.SetVerbose(false)
	r.Progress("after disable") // should be silent again

	out := buf.String()
	assert.NotContains(t, out, "before enable")
	assert.Contains(t, out, "after enable")
	assert.NotContains(t, out, "after disable")
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/common/reporter/... -v -run "TestReporter_Progress|TestReporter_SetVerbose"
```

Expected: FAIL — `r.SetVerbose` and `r.Progress` are undefined.

- [ ] **Step 3: Implement `verbose`, `SetVerbose`, and `Progress` in reporter.go**

In `internal/common/reporter/reporter.go`, update the struct and add two methods:

```go
// Reporter handles formatted terminal output.
type Reporter struct {
	w       io.Writer
	color   bool
	verbose bool
}
```

Add after the existing methods (before the closing of the file):

```go
// SetVerbose enables or disables progress output. Called by the cmd layer after flag parsing.
func (r *Reporter) SetVerbose(v bool) {
	r.verbose = v
}

// Progress prints a progress message, but only when verbose mode is enabled.
func (r *Reporter) Progress(msg string) {
	if !r.verbose {
		return
	}
	fmt.Fprintf(r.w, "%s\n", msg)
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
go test ./internal/common/reporter/... -v -run "TestReporter_Progress|TestReporter_SetVerbose"
```

Expected: PASS — all three new tests green.

- [ ] **Step 5: Run the full reporter test suite to check for regressions**

```bash
go test ./internal/common/reporter/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/common/reporter/reporter.go internal/common/reporter/reporter_test.go
git commit -m "feat: add SetVerbose and Progress to reporter"
```

---

### Task 3: Add progress logging to Apply (TDD)

**Files:**
- Modify: `internal/app/apply.go`
- Modify: `internal/app/apply_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/apply_test.go`:

```go
func TestApply_EmitsProgressMessages(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(cfgDir, "configs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "configs", ".zshrc"), []byte("# zshrc"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	r := &mockReporter{}
	mockDep := &mockDeployer{}
	stateStore := &mockStateStore{}
	baseCfg := &profile.FacetConfig{}

	targetPath := filepath.Join(stateDir, ".zshrc")
	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): baseCfg,
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				Configs: map[string]string{
					targetPath: "configs/.zshrc",
				},
				Packages: []profile.PackageEntry{
					{Name: "git", Install: profile.InstallCmd{Command: "brew install git"}},
				},
				PreApply: []profile.ScriptEntry{
					{Name: "setup", Run: "echo setup"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "teardown", Run: "echo teardown"},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:     r,
		Loader:       loader,
		BaseResolver: newStaticBaseResolver(baseCfg),
		Installer:    &mockInstaller{},
		StateStore:   stateStore,
		ScriptRunner: &mockScriptRunner{},
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	var progressMessages []string
	for _, msg := range r.messages {
		if strings.HasPrefix(msg, "progress: ") {
			progressMessages = append(progressMessages, msg)
		}
	}

	assert.Contains(t, progressMessages, "progress: Loading profile")
	assert.Contains(t, progressMessages, "progress: Resolving extends")
	assert.Contains(t, progressMessages, "progress: Merging layers")
	assert.Contains(t, progressMessages, "progress: Deploying configs")
	assert.Contains(t, progressMessages, "progress:   → "+targetPath)
	assert.Contains(t, progressMessages, "progress: Installing packages")
	assert.Contains(t, progressMessages, "progress:   → git")
	assert.Contains(t, progressMessages, "progress: Running pre_apply scripts")
	assert.Contains(t, progressMessages, "progress:   → setup")
	assert.Contains(t, progressMessages, "progress: Running post_apply scripts")
	assert.Contains(t, progressMessages, "progress:   → teardown")
}

func TestApply_EmitsUnapplyProgress_OnForce(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(cfgDir, "configs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "configs", ".zshrc"), []byte("# zshrc"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	prevTarget := filepath.Join(stateDir, ".zshrc")
	r := &mockReporter{}
	mockDep := &mockDeployer{}
	baseCfg := &profile.FacetConfig{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): baseCfg,
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				Configs: map[string]string{prevTarget: "configs/.zshrc"},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	stateStore := &mockStateStore{
		state: &ApplyState{
			Profile: "work",
			Configs: []deploy.ConfigResult{
				{Target: prevTarget, Source: "configs/.zshrc", Strategy: deploy.StrategySymlink},
			},
		},
	}

	a := New(Deps{
		Reporter:     r,
		Loader:       loader,
		BaseResolver: newStaticBaseResolver(baseCfg),
		Installer:    &mockInstaller{},
		StateStore:   stateStore,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		Force:     true,
	})
	require.NoError(t, err)

	var progressMessages []string
	for _, msg := range r.messages {
		if strings.HasPrefix(msg, "progress: ") {
			progressMessages = append(progressMessages, msg)
		}
	}

	assert.Contains(t, progressMessages, "progress: Unapplying previous state")
	assert.Contains(t, progressMessages, "progress:   → removing "+prevTarget)
}
```

Also add `"strings"` to the import block of `apply_test.go` if not already present.

- [ ] **Step 2: Verify the test fails**

```bash
go test ./internal/app/... -v -run "TestApply_EmitsProgressMessages|TestApply_EmitsUnapplyProgress"
```

Expected: FAIL — progress messages are not yet emitted.

- [ ] **Step 3: Add progress calls to `apply.go`**

In `internal/app/apply.go`, make these changes:

**Before loading profile (after LoadMeta, before LoadConfig):**

```go
// Step 1: Load facet.yaml
_, err = a.loader.LoadMeta(opts.ConfigDir)
if err != nil {
    return fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory, or run facet scaffold to create one.\n  detail: %w", err)
}

a.reporter.Progress("Loading profile")
// Step 2: Load profile
profilePath := filepath.Join(opts.ConfigDir, "profiles", profileName+".yaml")
```

**Before resolving extends:**

```go
a.reporter.Progress("Resolving extends")
resolvedBase, err := a.baseResolver.Resolve(profileCfg.Extends, opts.ConfigDir)
```

**Before merging layers:**

```go
a.reporter.Progress("Merging layers")
// Step 4: Merge layers
merged, err := profile.Merge(baseCfg, profileCfg)
```

**Before the unapply block (inside `if shouldUnapply {`):**

```go
if shouldUnapply {
    a.reporter.Progress("Unapplying previous state")
    if stages["configs"] {
        for _, cfg := range prevState.Configs {
            a.reporter.Progress(fmt.Sprintf("  → removing %s", cfg.Target))
        }
    }
    if stages["ai"] && a.aiOrchestrator != nil && prevState.AI != nil {
```

**At the top of the `if stages["configs"] {` deploy block:**

```go
if stages["configs"] {
    a.reporter.Progress("Deploying configs")
    var deployErr error

    targets := make([]string, 0, len(resolved.Configs))
    for target := range resolved.Configs {
        targets = append(targets, target)
    }
    sort.Strings(targets)

    for _, target := range targets {
        a.reporter.Progress(fmt.Sprintf("  → %s", target))
        source := resolved.Configs[target]
```

**Before running pre_apply:**

```go
if stages["pre_apply"] {
    if len(resolved.PreApply) > 0 {
        a.reporter.Progress("Running pre_apply scripts")
    }
    if err := a.runScripts(resolved.PreApply, opts.ConfigDir, "pre_apply"); err != nil {
        return err
    }
}
```

**Before installing packages:**

```go
if stages["packages"] {
    a.reporter.Progress("Installing packages")
    for _, pkg := range resolved.Packages {
        a.reporter.Progress(fmt.Sprintf("  → %s", pkg.Name))
    }
    pkgResults = a.installer.InstallAll(resolved.Packages)
}
```

**Before running post_apply:**

```go
if stages["post_apply"] {
    if len(resolved.PostApply) > 0 {
        a.reporter.Progress("Running post_apply scripts")
    }
    if err := a.runScripts(resolved.PostApply, opts.ConfigDir, "post_apply"); err != nil {
        return err
    }
}
```

**At the top of the `if stages["ai"] {` block:**

```go
if stages["ai"] {
    a.reporter.Progress("Applying AI configuration")
    if a.aiOrchestrator != nil {
```

**In `runScripts`, add per-script progress inside the loop:**

```go
for _, script := range scripts {
    a.reporter.Progress(fmt.Sprintf("  → %s", script.Name))
    dir := script.WorkDir
    if dir == "" {
        dir = fallbackDir
    }
    if err := a.scriptRunner.Run(script.Run, dir); err != nil {
        return fmt.Errorf("%s script %q failed: %w", stageName, script.Name, err)
    }
}
```

- [ ] **Step 4: Verify the new tests pass**

```bash
go test ./internal/app/... -v -run "TestApply_EmitsProgressMessages|TestApply_EmitsUnapplyProgress"
```

Expected: PASS.

- [ ] **Step 5: Run the full app test suite to check for regressions**

```bash
go test ./internal/app/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/apply.go internal/app/apply_test.go
git commit -m "feat: emit progress messages during apply stages"
```

---

### Task 4: Wire `--verbose` / `-v` global flag

**Files:**
- Modify: `cmd/root.go`
- Modify: `main.go`

- [ ] **Step 1: Add `VerboseSetter` interface and update `NewRootCmd` in `cmd/root.go`**

Replace the contents of `cmd/root.go` with:

```go
package cmd

import (
	"fmt"
	"os"

	"facet/internal/app"

	"github.com/spf13/cobra"
)

// VerboseSetter allows the cmd layer to enable verbose output on the reporter
// after Cobra has parsed the persistent --verbose flag.
type VerboseSetter interface {
	SetVerbose(bool)
}

// NewRootCmd builds the full command tree and returns the root command.
func NewRootCmd(application *app.App, vs VerboseSetter) *cobra.Command {
	var configDir, stateDir string
	var verbose bool

	rootCmd := &cobra.Command{
		Use:     "facet",
		Short:   "Developer environment configuration manager",
		Version: "0.1.0",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			vs.SetVerbose(verbose)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "Path to facet config repo (default: current directory)")
	rootCmd.PersistentFlags().StringVarP(&stateDir, "state-dir", "s", "", "Path to machine-local state directory (default: ~/.facet)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show stage-by-stage progress during apply")

	rootCmd.AddCommand(newApplyCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newDocsCmd())
	rootCmd.AddCommand(newScaffoldCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newStatusCmd(application, &configDir, &stateDir))

	aiCmd := newAICmd()
	skillsCmd := newAISkillsCmd()
	skillsCmd.AddCommand(newAISkillsCheckCmd(application))
	skillsCmd.AddCommand(newAISkillsUpdateCmd(application))
	aiCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(aiCmd)

	return rootCmd
}

// resolveConfigDir returns the config directory path.
// Uses --config-dir flag if set, otherwise current working directory.
func resolveConfigDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return os.Getwd()
}

// resolveStateDir returns the state directory path.
// Uses --state-dir flag if set, otherwise ~/.facet/.
func resolveStateDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return home + "/.facet", nil
}
```

- [ ] **Step 2: Update `main.go` to pass `r` as the `VerboseSetter`**

Change the `NewRootCmd` call in `main.go` from:

```go
rootCmd := cmd.NewRootCmd(application)
```

to:

```go
rootCmd := cmd.NewRootCmd(application, r)
```

- [ ] **Step 3: Build to confirm the wiring compiles**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 4: Smoke test manually**

```bash
go run . --help
```

Expected: output includes `--verbose` / `-v` in the flags list.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go main.go
git commit -m "feat: add --verbose/-v global flag wired to reporter"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `internal/docs/topics/commands.md`
- Modify: `README.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Update `internal/docs/topics/commands.md`**

Add `--verbose` to the Global Flags table:

```markdown
## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config-dir` | `-c` | Current directory | Path to the facet config repo |
| `--state-dir` | `-s` | `~/.facet` | Path to the machine-local state directory |
| `--verbose` | `-v` | false | Show stage-by-stage progress during apply |
```

Add a usage example to the `facet apply` flags list:

```markdown
Flags:

- `--dry-run`: preview the resolved actions without writing changes
- `--force`: replace conflicting non-facet files and unapply previous state first when needed
- `--skip-failure`: warn on deploy failures instead of rolling back immediately
- `--stages`: comma-separated list of stages to run (default: all)
- `--verbose` / `-v`: stream stage and item progress as apply runs
```

Add usage example to the apply command block:

```bash
facet apply work
facet apply work --dry-run
facet apply work --force
facet apply work --verbose
facet apply work --skip-failure
facet apply work --stages configs,packages
```

- [ ] **Step 2: Update `README.md`**

In the global flags table (around line 267), add:

```markdown
| `--verbose` | `-v` | false | Stream stage and item progress during apply |
```

In the apply examples block (around lines 246–250), add:

```bash
facet apply work --verbose           # stream stage-by-stage progress
```

- [ ] **Step 3: Update `docs/architecture/v1-design-spec.md`**

Find the Global flags table (around line 404) and add `--verbose`:

```markdown
### Global flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config-dir` | `-c` | Current working directory | Path to the facet config repo |
| `--state-dir` | `-s` | `~/.facet/` | Path to the machine-local state directory |
| `--verbose` | `-v` | false | Stream stage and item progress during apply |
```

Find the `facet apply <profile>` flags list (around line 432) and add:

```markdown
**Flags:**
- `--dry-run` — preview what would happen without making changes
- `--force` — unapply + apply, skip prompts
- `--skip-failure` — warn on config deploy failure instead of rollback
- `--verbose` / `-v` — stream stage and item progress as apply runs
```

- [ ] **Step 4: Verify the docs build**

```bash
go build ./...
go test ./cmd/... -v
```

Expected: PASS (docs are embedded at build time; a build failure means a broken embed).

- [ ] **Step 5: Commit**

```bash
git add internal/docs/topics/commands.md README.md docs/architecture/v1-design-spec.md
git commit -m "docs: add --verbose flag to commands, README, and design spec"
```

---

### Task 6: Add E2E test suite and run pre-commit

**Files:**
- Create: `e2e/suites/16-verbose-flag.sh`

- [ ] **Step 1: Write the E2E test suite**

Create `e2e/suites/16-verbose-flag.sh`:

```bash
#!/bin/bash
# e2e/suites/16-verbose-flag.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# --verbose shows stage progress lines
output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose work 2>&1)
echo "$output" | grep -q "Loading profile" || { echo "  FAIL: missing 'Loading profile' in verbose output"; exit 1; }
echo "$output" | grep -q "Resolving extends" || { echo "  FAIL: missing 'Resolving extends' in verbose output"; exit 1; }
echo "$output" | grep -q "Deploying configs" || { echo "  FAIL: missing 'Deploying configs' in verbose output"; exit 1; }
echo "$output" | grep -q "Installing packages" || { echo "  FAIL: missing 'Installing packages' in verbose output"; exit 1; }
echo "$output" | grep -q "Running pre_apply scripts" || { echo "  FAIL: missing 'Running pre_apply scripts' in verbose output"; exit 1; }
echo "$output" | grep -q "Running post_apply scripts" || { echo "  FAIL: missing 'Running post_apply scripts' in verbose output"; exit 1; }
echo "  --verbose shows stage progress lines"

# item-level detail appears under each stage
echo "$output" | grep -q "ripgrep" || { echo "  FAIL: missing package item 'ripgrep' in verbose output"; exit 1; }
echo "$output" | grep -q "create-pre-marker" || { echo "  FAIL: missing pre_apply item 'create-pre-marker' in verbose output"; exit 1; }
echo "  --verbose shows item-level detail"

# -v short form works identically
output_short=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply -v work 2>&1)
echo "$output_short" | grep -q "Loading profile" || { echo "  FAIL: -v short form missing 'Loading profile'"; exit 1; }
echo "  -v short form works"

# Without --verbose, stage progress lines must not appear
output_quiet=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work 2>&1)
if echo "$output_quiet" | grep -q "Loading profile"; then
    echo "  FAIL: 'Loading profile' appeared without --verbose flag"
    exit 1
fi
if echo "$output_quiet" | grep -q "Deploying configs"; then
    echo "  FAIL: 'Deploying configs' appeared without --verbose flag"
    exit 1
fi
echo "  without --verbose, no stage progress lines shown"

# --verbose --force shows unapply progress
output_force=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose --force work 2>&1)
echo "$output_force" | grep -q "Unapplying previous state" || { echo "  FAIL: missing 'Unapplying previous state' in verbose --force output"; exit 1; }
echo "  --verbose --force shows unapply progress"
```

- [ ] **Step 2: Make the file executable**

```bash
chmod +x e2e/suites/16-verbose-flag.sh
```

- [ ] **Step 3: Run the full pre-commit check**

```bash
make pre-commit
```

Expected: unit tests, native E2E tests, and Docker Linux E2E tests all PASS. The output will include suite `16-verbose-flag` passing.

If Docker is not running, start it first, then re-run.

- [ ] **Step 4: Commit**

```bash
git add e2e/suites/16-verbose-flag.sh
git commit -m "test(e2e): add verbose flag suite 16"
```
