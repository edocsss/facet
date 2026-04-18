# Restructure, DI, and Cleanup Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure internal packages by business domain, introduce proper DI with interfaces, eliminate init()/globals, remove integration tests, and clean up E2E binaries.

**Architecture:** The `cmd/` layer becomes a thin Cobra adapter. All business logic moves to `internal/app/` which orchestrates domain packages (`profile/`, `deploy/`, `packages/`) via injected interfaces. State types dissolve into their owning packages. Reporter moves to `common/reporter/` as a pure formatter with no business imports. All wiring happens explicitly in `main.go`.

**Tech Stack:** Go 1.23.5, Cobra, testify, gopkg.in/yaml.v3

---

## File Structure

### New directories to create
- `internal/profile/` — renamed from `internal/config/`
- `internal/app/` — orchestration + state
- `internal/common/reporter/` — moved from `internal/reporter/`

### Directories to delete (after migration)
- `internal/config/`
- `internal/state/`
- `internal/reporter/`

### Final structure
```
main.go                           # Explicit DI wiring, build command tree, execute
cmd/
  root.go                         # NewRootCmd(app) → *cobra.Command, no globals
  apply.go                        # newApplyCmd — thin flag parsing, calls app.Apply()
  init_cmd.go                     # newInitCmd — thin, calls app.Init()
  status.go                       # newStatusCmd — thin, calls app.Status()
internal/
  profile/
    types.go                      # FacetMeta, FacetConfig, PackageEntry, InstallCmd (from config/)
    types_test.go                 # (from config/)
    loader.go                     # Loader struct with LoadMeta, LoadConfig (from config/)
    loader_test.go                # (from config/)
    merger.go                     # Merge, deepMergeVars, mergePackages, mergeConfigs (from config/)
    merger_test.go                # (from config/)
    resolver.go                   # Resolve, SubstituteVars, ValidateProfile (from config/)
    resolver_test.go              # (from config/)
  deploy/
    deployer.go                   # Deployer struct implements Service interface; owns ConfigResult type
    deployer_test.go              # Updated to use deploy.ConfigResult
    pathexpand.go                 # ExpandPath, ValidateSourcePath (unchanged)
    pathexpand_test.go            # Fixed: use t.Setenv("HOME", ...) to avoid reading real host HOME
  packages/
    runner.go                     # CommandRunner interface + ShellRunner concrete
    installer.go                  # Installer struct with injected CommandRunner; owns PackageResult type
    installer_test.go             # Updated: uses mock CommandRunner, no real shell commands
  app/
    interfaces.go                 # All interfaces consumed by App: ProfileLoader, Reporter, StateStore, DeployerFactory, Installer
    app.go                        # App struct, Deps, New()
    apply.go                      # App.Apply() — full apply workflow from cmd/apply.go
    init.go                       # App.Init() — scaffolding from cmd/init_cmd.go
    status.go                     # App.Status() — status + validity checks from cmd/status.go
    state.go                      # ApplyState type + FileStateStore (Read/Write/CanaryWrite)
    report.go                     # App helper methods for printing apply/status reports
    state_test.go                 # (from state/)
    apply_test.go                 # Unit tests with mocked deps
    init_test.go                  # Unit tests with mocked deps
    status_test.go                # Unit tests with mocked deps
  common/
    reporter/
      reporter.go                 # Pure formatter: Success, Warning, Error, Header, PrintLine, Separator
      reporter_test.go            # (adapted from reporter/)
e2e/
  e2e_test.go                    # Updated: t.Cleanup to remove built binaries
AGENTS.md                        # Updated with architectural rules
```

### Import path changes
| Old | New |
|-----|-----|
| `facet/internal/config` | `facet/internal/profile` |
| `facet/internal/state` | `facet/internal/app` (for ApplyState) |
| `facet/internal/reporter` | `facet/internal/common/reporter` |
| `facet/internal/deploy` | `facet/internal/deploy` (unchanged) |
| `facet/internal/packages` | `facet/internal/packages` (unchanged) |
| `facet/cmd` | `facet/cmd` (unchanged, but API changes) |

---

## Transition strategy

During Chunk 1, the `cmd/` layer still contains business logic and must keep compiling. Each task that changes an internal package also updates `cmd/` files as a temporary bridge. These bridge changes are throwaway — `cmd/` is completely rewritten in Chunk 3 (Task 14).

The old `internal/reporter/` and `internal/state/` stay alive until Task 16 (Chunk 3), when they are deleted after all consumers have migrated.

---

## Chunk 1: Foundation — Profile Rename, Deploy/Packages Interface, Reporter Move

This chunk does the mechanical renames and introduces interfaces into leaf packages. No business logic moves yet. Tests must pass after each task.

### Task 1: Rename `internal/config/` → `internal/profile/`

**Files:**
- Rename: `internal/config/*.go` → `internal/profile/*.go`
- Modify: all files importing `facet/internal/config`

- [ ] **Step 1: Create `internal/profile/` and copy files**

```bash
mkdir -p internal/profile
cp internal/config/types.go internal/profile/types.go
cp internal/config/types_test.go internal/profile/types_test.go
cp internal/config/loader.go internal/profile/loader.go
cp internal/config/loader_test.go internal/profile/loader_test.go
cp internal/config/merger.go internal/profile/merger.go
cp internal/config/merger_test.go internal/profile/merger_test.go
cp internal/config/resolver.go internal/profile/resolver.go
cp internal/config/resolver_test.go internal/profile/resolver_test.go
```

- [ ] **Step 2: Update package declarations in all profile/ files**

Change `package config` → `package profile` in every file in `internal/profile/`.

- [ ] **Step 3: Update all import paths**

In every file that imports `facet/internal/config`, change to `facet/internal/profile`. Files to update:
- `cmd/apply.go` — `"facet/internal/config"` → `"facet/internal/profile"` and update all `config.` references to `profile.`
- `internal/deploy/deployer.go` — `"facet/internal/config"` → `"facet/internal/profile"` and update `config.SubstituteVars` → `profile.SubstituteVars`
- `internal/packages/installer.go` — `"facet/internal/config"` → `"facet/internal/profile"` and update `config.PackageEntry` → `profile.PackageEntry`
- `internal/packages/installer_test.go` — same

- [ ] **Step 4: Delete `internal/config/`**

```bash
rm -rf internal/config
```

- [ ] **Step 5: Run tests to verify**

```bash
go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: rename internal/config to internal/profile"
```

---

### Task 2: Add `CommandRunner` interface to packages, fix test side effects

**Files:**
- Create: `internal/packages/runner.go`
- Modify: `internal/packages/installer.go`
- Modify: `internal/packages/installer_test.go`

- [ ] **Step 1: Create `internal/packages/runner.go` with interface and concrete implementation**

```go
package packages

import (
	"fmt"
	"os/exec"
)

// CommandRunner abstracts shell command execution.
type CommandRunner interface {
	Run(command string) error
}

// ShellRunner executes commands via sh -c.
type ShellRunner struct{}

func NewShellRunner() *ShellRunner {
	return &ShellRunner{}
}

func (r *ShellRunner) Run(command string) error {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
```

- [ ] **Step 2: Add `PackageResult` type and convert Installer to struct with injected runner**

Rewrite `internal/packages/installer.go`:

```go
package packages

import (
	"fmt"
	"runtime"

	"facet/internal/profile"
)

// PackageResult records the result of a single package install.
type PackageResult struct {
	Name    string `json:"name"`
	Install string `json:"install"`
	Status  string `json:"status"` // "ok", "failed", or "skipped"
	Error   string `json:"error,omitempty"`
}

// DetectOS returns "macos" or "linux".
func DetectOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "linux"
}

// GetInstallCommand returns the install command for the given OS.
// Returns the command and whether to skip (true = skip, no command for this OS).
func GetInstallCommand(pkg profile.PackageEntry, osName string) (string, bool) {
	cmd, ok := pkg.Install.ForOS(osName)
	if !ok {
		return "", true
	}
	return cmd, false
}

// Installer handles package installation using an injected CommandRunner.
type Installer struct {
	runner CommandRunner
	osName string
}

// NewInstaller creates an Installer with the given runner and OS name.
func NewInstaller(runner CommandRunner, osName string) *Installer {
	return &Installer{runner: runner, osName: osName}
}

// InstallAll runs install commands for all packages.
// Failed installs are recorded but do not stop other installations.
func (inst *Installer) InstallAll(pkgs []profile.PackageEntry) []PackageResult {
	results := make([]PackageResult, 0, len(pkgs))

	for _, pkg := range pkgs {
		cmd, skip := GetInstallCommand(pkg, inst.osName)

		pr := PackageResult{
			Name:    pkg.Name,
			Install: cmd,
		}

		if skip {
			pr.Status = "skipped"
			pr.Error = fmt.Sprintf("no install command for OS %q", inst.osName)
			results = append(results, pr)
			continue
		}

		err := inst.runner.Run(cmd)
		if err != nil {
			pr.Status = "failed"
			pr.Error = err.Error()
		} else {
			pr.Status = "ok"
		}

		results = append(results, pr)
	}

	return results
}
```

- [ ] **Step 3: Rewrite tests to use mock runner (no real shell commands)**

Rewrite `internal/packages/installer_test.go`:

```go
package packages

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/profile"
)

// mockRunner records commands and returns preset errors.
type mockRunner struct {
	commands []string
	failOn   map[string]error
}

func newMockRunner() *mockRunner {
	return &mockRunner{failOn: make(map[string]error)}
}

func (m *mockRunner) Run(command string) error {
	m.commands = append(m.commands, command)
	if err, ok := m.failOn[command]; ok {
		return err
	}
	return nil
}

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	if runtime.GOOS == "darwin" {
		assert.Equal(t, "macos", os)
	} else {
		assert.Equal(t, "linux", os)
	}
}

func TestGetInstallCommand_Simple(t *testing.T) {
	pkg := profile.PackageEntry{
		Name:    "ripgrep",
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, skip := GetInstallCommand(pkg, "macos")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)

	cmd, skip = GetInstallCommand(pkg, "linux")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)
}

func TestGetInstallCommand_PerOS(t *testing.T) {
	pkg := profile.PackageEntry{
		Name: "lazydocker",
		Install: profile.InstallCmd{
			PerOS: map[string]string{
				"macos": "brew install lazydocker",
				"linux": "go install github.com/jesseduffield/lazydocker@latest",
			},
		},
	}

	cmd, skip := GetInstallCommand(pkg, "macos")
	assert.Equal(t, "brew install lazydocker", cmd)
	assert.False(t, skip)

	cmd, skip = GetInstallCommand(pkg, "linux")
	assert.Equal(t, "go install github.com/jesseduffield/lazydocker@latest", cmd)
	assert.False(t, skip)
}

func TestGetInstallCommand_MissingOS(t *testing.T) {
	pkg := profile.PackageEntry{
		Name: "xcode-tools",
		Install: profile.InstallCmd{
			PerOS: map[string]string{
				"macos": "xcode-select --install",
			},
		},
	}

	_, skip := GetInstallCommand(pkg, "linux")
	assert.True(t, skip)
}

func TestInstallAll_CollectsResults(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{Name: "echo-test", Install: profile.InstallCmd{Command: "echo hello"}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, "echo-test", results[0].Name)
	assert.Equal(t, "ok", results[0].Status)
	assert.Equal(t, []string{"echo hello"}, runner.commands)
}

func TestInstallAll_FailureContinues(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["will-fail-cmd"] = errors.New("exit status 1")
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{Name: "will-fail", Install: profile.InstallCmd{Command: "will-fail-cmd"}},
		{Name: "will-pass", Install: profile.InstallCmd{Command: "echo ok"}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 2)
	assert.Equal(t, "failed", results[0].Status)
	assert.NotEmpty(t, results[0].Error)
	assert.Equal(t, "ok", results[1].Status)
}

func TestInstallAll_SkippedOS(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "linux")

	pkgs := []profile.PackageEntry{
		{Name: "mac-only", Install: profile.InstallCmd{
			PerOS: map[string]string{"macos": "echo mac"},
		}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Empty(t, runner.commands) // runner was never called
}

func TestInstallResultFields(t *testing.T) {
	r := PackageResult{Name: "git", Install: "brew install git", Status: "ok"}
	assert.Equal(t, "ok", r.Status)
}
```

- [ ] **Step 4: Bridge — update cmd/apply.go to use new Installer struct**

In `cmd/apply.go`, replace the free function call:
```go
// OLD:
osName := packages.DetectOS()
pkgResults := packages.InstallAll(resolved.Packages, osName)

// NEW:
osName := packages.DetectOS()
runner := packages.NewShellRunner()
installer := packages.NewInstaller(runner, osName)
pkgResults := installer.InstallAll(resolved.Packages)
```

Also update the `pkgResults` usage — it was `[]state.PackageState`, now it's `[]packages.PackageResult`. Update the `ApplyState` construction to convert:
```go
// Convert PackageResult → state.PackageState for the old state package
statePkgs := make([]state.PackageState, len(pkgResults))
for i, pr := range pkgResults {
    statePkgs[i] = state.PackageState{Name: pr.Name, Install: pr.Install, Status: pr.Status, Error: pr.Error}
}
```

Use `statePkgs` in the `state.ApplyState` construction. This bridge is discarded in Task 14.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v -count=1
```

Expected: all pass, no real shell commands in package tests.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: add CommandRunner interface to packages, mock in tests"
```

---

### Task 3: Add `ConfigResult` type to deploy, add `Service` interface

**Files:**
- Modify: `internal/deploy/deployer.go`
- Modify: `internal/deploy/deployer_test.go`

- [ ] **Step 1: Replace state.ConfigState with local ConfigResult, add Service interface**

In `internal/deploy/deployer.go`:
- Remove import of `facet/internal/state`
- Add `ConfigResult` type (same fields and JSON tags as `state.ConfigState`)
- Add `Service` interface
- Update all references from `state.ConfigState` → `ConfigResult`

```go
// ConfigResult records a single deployed config.
type ConfigResult struct {
	Target   string `json:"target"`
	Source   string `json:"source"`
	Strategy string `json:"strategy"` // "symlink" or "template"
}

// Service is the interface for deployment operations.
type Service interface {
	DeployOne(targetPath, source string, force bool) (ConfigResult, error)
	Unapply(configs []ConfigResult) error
	Rollback() error
	Deployed() []ConfigResult
}
```

Update the `Deployer` struct:
- `deployed []state.ConfigState` → `deployed []ConfigResult`
- `ownedTargets` constructor param: `ownedConfigs []state.ConfigState` → `ownedConfigs []ConfigResult`
- All method signatures updated accordingly
- Change `config.SubstituteVars` → `profile.SubstituteVars` (import path already changed in Task 1)

- [ ] **Step 2: Update deployer_test.go**

Remove import of `facet/internal/state`. Replace all `state.ConfigState` with `deploy.ConfigResult` (since tests are in `package deploy`, just use `ConfigResult`).

- [ ] **Step 3: Run tests**

```bash
go test ./internal/deploy/... -v -count=1
```

Expected: all pass.

- [ ] **Step 4: Bridge — update cmd/apply.go for new deploy types**

Update `cmd/apply.go`:
- `state` import stays (still needed for ApplyState, Read, Write, CanaryWrite)
- Change `var prevConfigs []state.ConfigState` → `var prevConfigs []deploy.ConfigResult`
- Where `prevState.Configs` (still `[]state.ConfigState` from the JSON) is passed to the deployer, convert:

```go
// Convert state.ConfigState → deploy.ConfigResult for deployer
var prevConfigs []deploy.ConfigResult
if prevState != nil {
    for _, c := range prevState.Configs {
        prevConfigs = append(prevConfigs, deploy.ConfigResult{Target: c.Target, Source: c.Source, Strategy: c.Strategy})
    }
}
```

- Similarly, convert `deployer.Deployed()` (`[]deploy.ConfigResult`) back to `[]state.ConfigState` when building the `state.ApplyState`.

This bridge is discarded in Task 14.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: add ConfigResult type and Service interface to deploy"
```

---

### Task 4: Move reporter to `common/reporter/`, strip to pure formatter

**Files:**
- Create: `internal/common/reporter/reporter.go`
- Create: `internal/common/reporter/reporter_test.go`
- Delete: `internal/reporter/`

- [ ] **Step 1: Create `internal/common/reporter/` with stripped reporter**

The reporter becomes a pure formatting utility with NO business domain imports. Remove `PrintApplyReport`, `PrintStatus`, `ValidityCheck`, and `timeSince` — these move to `app/` later.

```go
package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// Reporter handles formatted terminal output.
type Reporter struct {
	w     io.Writer
	color bool
}

// New creates a new Reporter.
func New(w io.Writer, color bool) *Reporter {
	return &Reporter{w: w, color: color}
}

// NewDefault creates a Reporter that writes to stdout with auto-detected color support.
func NewDefault() *Reporter {
	color := os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
	return &Reporter{w: os.Stdout, color: color}
}

func (r *Reporter) colorize(color, text string) string {
	if !r.color {
		return text
	}
	return color + text + colorReset
}

// Success prints a success message with a checkmark.
func (r *Reporter) Success(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorGreen, "✓"), msg)
}

// Warning prints a warning message.
func (r *Reporter) Warning(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorYellow, "⚠"), msg)
}

// Error prints an error message.
func (r *Reporter) Error(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorRed, "✗"), msg)
}

// Header prints a section header.
func (r *Reporter) Header(msg string) {
	fmt.Fprintf(r.w, "\n%s\n", r.colorize(colorBold, msg))
}

// PrintLine prints a formatted line.
func (r *Reporter) PrintLine(msg string) {
	fmt.Fprintf(r.w, "%s\n", msg)
}

// Dim returns the text with dim styling (for use in formatted output).
func (r *Reporter) Dim(text string) string {
	return r.colorize(colorDim, text)
}

// Separator prints a visual separator line.
func (r *Reporter) Separator() {
	fmt.Fprintf(r.w, "%s\n", strings.Repeat("─", 60))
}
```

- [ ] **Step 2: Create `internal/common/reporter/reporter_test.go`**

```go
package reporter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReporter_Success(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Success("test message")
	output := buf.String()
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "test message")
}

func TestReporter_Warning(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Warning("warning message")
	output := buf.String()
	assert.Contains(t, output, "⚠")
	assert.Contains(t, output, "warning message")
}

func TestReporter_Error(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Error("error message")
	output := buf.String()
	assert.Contains(t, output, "✗")
	assert.Contains(t, output, "error message")
}

func TestReporter_ColorDisabled(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Success("test message")
	output := buf.String()
	assert.NotContains(t, output, "\033[")
	assert.Contains(t, output, "test message")
}

func TestReporter_Header(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Header("Section Title")
	output := buf.String()
	assert.Contains(t, output, "Section Title")
}
```

- [ ] **Step 3: Update imports in cmd/ files**

Update `cmd/init_cmd.go`, `cmd/apply.go`, `cmd/status.go`:
- `"facet/internal/reporter"` → `"facet/internal/common/reporter"`

Note: `cmd/apply.go` and `cmd/status.go` call `PrintApplyReport` and `PrintStatus` which no longer exist. For now, inline the formatting logic temporarily in cmd/ (these files will be gutted in Chunk 2 anyway). Or simply comment out the report calls and mark with `// TODO: move to app/`.

Pragmatic approach: temporarily keep the old `internal/reporter/` alongside the new `internal/common/reporter/` until Chunk 2 migrates the business logic. Only `cmd/init_cmd.go` (which only calls `Success()`) switches to the new import now.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/common/reporter/... -v -count=1
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "refactor: create common/reporter as pure formatter"
```

---

### Task 5: Fix pathexpand_test.go host side effects

**Files:**
- Modify: `internal/deploy/pathexpand_test.go`

- [ ] **Step 1: Add `t.Setenv("HOME", ...)` to tests that call `os.UserHomeDir()`**

Update `TestExpandPath_Tilde`, `TestExpandPath_TildeAlone`, `TestExpandPath_DollarHOME`, `TestExpandPath_DollarBraceHOME` to use a controlled HOME:

```go
func TestExpandPath_Tilde(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("~/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}

func TestExpandPath_TildeAlone(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("~")
	require.NoError(t, err)
	assert.Equal(t, homeDir, result)
}

func TestExpandPath_DollarHOME(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("$HOME/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}

func TestExpandPath_DollarBraceHOME(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("${HOME}/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/deploy/... -v -count=1
```

Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "fix: isolate pathexpand tests from host HOME directory"
```

---

## Chunk 2: App Package — State, Interfaces, Orchestration

This chunk creates the `app/` package, moves state types/persistence, defines all interfaces, and migrates business logic from `cmd/`.

### Task 6: Create `app/` package — state types and persistence

**Files:**
- Create: `internal/app/state.go`
- Create: `internal/app/state_test.go`

- [ ] **Step 1: Create `internal/app/state.go`**

```go
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"facet/internal/deploy"
	"facet/internal/packages"
)

const stateFile = ".state.json"

// ApplyState records the result of a facet apply run.
type ApplyState struct {
	Profile      string                   `json:"profile"`
	AppliedAt    time.Time                `json:"applied_at"`
	FacetVersion string                   `json:"facet_version"`
	Packages     []packages.PackageResult `json:"packages"`
	Configs      []deploy.ConfigResult    `json:"configs"`
}

// FileStateStore handles reading and writing state to the filesystem.
type FileStateStore struct{}

// NewFileStateStore creates a new FileStateStore.
func NewFileStateStore() *FileStateStore {
	return &FileStateStore{}
}

// Write saves the apply state to .state.json in the state directory.
func (s *FileStateStore) Write(stateDir string, st *ApplyState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := filepath.Join(stateDir, stateFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// Read loads the apply state from .state.json.
// Returns nil, nil if the file does not exist (no previous apply).
func (s *FileStateStore) Read(stateDir string) (*ApplyState, error) {
	path := filepath.Join(stateDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var st ApplyState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &st, nil
}

// CanaryWrite performs an early write to .state.json to detect permission or disk errors
// before doing any real work.
func (s *FileStateStore) CanaryWrite(stateDir string) error {
	path := filepath.Join(stateDir, stateFile)

	// If file already exists, check we can write to it
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("cannot write to %s: %w", path, err)
		}
		f.Close()
		return nil
	}

	// File doesn't exist — try to create it with a minimal state
	canary := &ApplyState{
		Profile:      "_canary",
		AppliedAt:    time.Now(),
		FacetVersion: "0.1.0",
	}
	return s.Write(stateDir, canary)
}
```

- [ ] **Step 2: Create `internal/app/state_test.go`**

```go
package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/deploy"
	"facet/internal/packages"
)

func TestFileStateStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	s := &ApplyState{
		Profile:      "acme",
		AppliedAt:    time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC),
		FacetVersion: "0.1.0",
		Packages: []packages.PackageResult{
			{Name: "git", Install: "brew install git", Status: "ok"},
			{Name: "docker", Install: "brew install docker", Status: "failed", Error: "not found"},
		},
		Configs: []deploy.ConfigResult{
			{Target: "~/.gitconfig", Source: "configs/.gitconfig", Strategy: "template"},
			{Target: "~/.zshrc", Source: "configs/.zshrc", Strategy: "symlink"},
		},
	}

	err := store.Write(dir, s)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "acme", loaded.Profile)
	assert.Equal(t, "0.1.0", loaded.FacetVersion)
	assert.Len(t, loaded.Packages, 2)
	assert.Len(t, loaded.Configs, 2)
	assert.Equal(t, "failed", loaded.Packages[1].Status)
	assert.Equal(t, "template", loaded.Configs[0].Strategy)
}

func TestFileStateStore_Read_Missing(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()
	s, err := store.Read(dir)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestFileStateStore_Read_Corrupted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".state.json"), []byte("{{{bad json"), 0o644))

	store := NewFileStateStore()
	_, err := store.Read(dir)
	assert.Error(t, err)
}

func TestFileStateStore_CanaryWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()
	err := store.CanaryWrite(dir)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".state.json"))
	assert.NoError(t, err)
}

func TestFileStateStore_CanaryWrite_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o444))
	defer os.Chmod(dir, 0o755) // cleanup

	store := NewFileStateStore()
	err := store.CanaryWrite(dir)
	assert.Error(t, err)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/... -v -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: create app package with state types and FileStateStore"
```

---

### Task 7: Create `app/interfaces.go` — all interfaces consumed by App

**Files:**
- Create: `internal/app/interfaces.go`

- [ ] **Step 1: Create `internal/app/interfaces.go`**

```go
package app

import (
	"facet/internal/deploy"
	"facet/internal/profile"
)

// ProfileLoader loads and parses facet configuration files.
type ProfileLoader interface {
	LoadMeta(configDir string) (*profile.FacetMeta, error)
	LoadConfig(path string) (*profile.FacetConfig, error)
}

// Reporter handles formatted terminal output.
type Reporter interface {
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Header(msg string)
	PrintLine(msg string)
	Dim(text string) string
}

// StateStore handles reading and writing apply state.
type StateStore interface {
	Read(stateDir string) (*ApplyState, error)
	Write(stateDir string, s *ApplyState) error
	CanaryWrite(stateDir string) error
}

// Installer handles package installation.
type Installer interface {
	InstallAll(pkgs []profile.PackageEntry) []PackageResult
}

// PackageResult is re-exported from packages to avoid forcing consumers to import packages.
type PackageResult = packages.PackageResult
```

Note: the `PackageResult` type alias requires an import. However, to avoid a circular import (app imports packages, and this is a type alias), this is fine since `packages` doesn't import `app`.

Actually, remove the type alias — consumers of `app` should import `packages` directly if they need `PackageResult`. The `ApplyState` struct already references `packages.PackageResult` in its field type, so consumers who interact with `ApplyState` already transitively depend on `packages`.

Update the file to add the missing import:

```go
package app

import (
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)

// ProfileLoader loads and parses facet configuration files.
type ProfileLoader interface {
	LoadMeta(configDir string) (*profile.FacetMeta, error)
	LoadConfig(path string) (*profile.FacetConfig, error)
}

// Reporter handles formatted terminal output.
type Reporter interface {
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Header(msg string)
	PrintLine(msg string)
	Dim(text string) string
}

// StateStore handles reading and writing apply state.
type StateStore interface {
	Read(stateDir string) (*ApplyState, error)
	Write(stateDir string, s *ApplyState) error
	CanaryWrite(stateDir string) error
}

// Installer handles package installation.
type Installer interface {
	InstallAll(pkgs []profile.PackageEntry) []packages.PackageResult
}

// DeployerFactory creates a deploy.Service for a given configuration.
type DeployerFactory func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service
```

- [ ] **Step 2: Run tests (compile check)**

```bash
go build ./internal/app/...
```

Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: define app interfaces for DI (ProfileLoader, Reporter, StateStore, Installer, DeployerFactory)"
```

---

### Task 8: Convert profile.Loader to struct, implement ProfileLoader interface

**Files:**
- Modify: `internal/profile/loader.go`
- Modify: `internal/profile/loader_test.go`

- [ ] **Step 1: Convert free functions to Loader struct methods**

Update `internal/profile/loader.go`:

```go
package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Loader handles loading and parsing facet configuration files.
type Loader struct{}

// NewLoader creates a new Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// LoadMeta reads and parses facet.yaml from the given config directory.
func (l *Loader) LoadMeta(configDir string) (*FacetMeta, error) {
	path := filepath.Join(configDir, "facet.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read facet.yaml: %w (is this a facet config directory?)", err)
	}
	var meta FacetMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse facet.yaml: %w", err)
	}
	return &meta, nil
}

// LoadConfig reads and parses a single YAML config file (base.yaml, profile, or .local.yaml).
func (l *Loader) LoadConfig(path string) (*FacetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(path), err)
	}
	var cfg FacetConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filepath.Base(path), err)
	}
	return &cfg, nil
}

// ValidateProfile checks that a profile config has a valid extends field.
func ValidateProfile(cfg *FacetConfig) error {
	if cfg.Extends == "" {
		return fmt.Errorf("profile is missing 'extends' field (must be 'extends: base')")
	}
	if cfg.Extends != "base" {
		return fmt.Errorf("profile has 'extends: %s' but only 'extends: base' is supported", cfg.Extends)
	}
	return nil
}
```

Note: `ValidateProfile` stays a free function — it's pure validation with no I/O.

- [ ] **Step 2: Update loader_test.go to use Loader struct**

Replace `LoadMeta(dir)` with `loader.LoadMeta(dir)` etc.:

```go
// At the top of each test that uses LoadMeta/LoadConfig:
loader := NewLoader()

// Then use:
meta, err := loader.LoadMeta(dir)
cfg, err := loader.LoadConfig(filepath.Join(dir, "base.yaml"))
```

- [ ] **Step 3: Bridge — update cmd/apply.go and cmd/init_cmd.go to use Loader struct**

In `cmd/apply.go`, add at the start of `runApply`:
```go
loader := profile.NewLoader()
```
Then replace all `profile.LoadMeta(...)` → `loader.LoadMeta(...)` and `profile.LoadConfig(...)` → `loader.LoadConfig(...)`.

In `cmd/init_cmd.go`, if it calls LoadMeta/LoadConfig (it doesn't currently), update similarly.

- [ ] **Step 4: Run tests**

```bash
go test ./... -v -count=1
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "refactor: convert profile loader to struct for DI"
```

---

### Task 9: Create `app/app.go` — App struct and constructor

**Files:**
- Create: `internal/app/app.go`

- [ ] **Step 1: Create `internal/app/app.go`**

```go
package app

// Deps holds all dependencies for the App.
type Deps struct {
	Loader          ProfileLoader
	Installer       Installer
	Reporter        Reporter
	StateStore      StateStore
	DeployerFactory DeployerFactory
	Version         string
}

// App is the application service layer that orchestrates all facet operations.
type App struct {
	loader          ProfileLoader
	installer       Installer
	reporter        Reporter
	stateStore      StateStore
	deployerFactory DeployerFactory
	version         string
}

// New creates a new App with the given dependencies.
func New(deps Deps) *App {
	return &App{
		loader:          deps.Loader,
		installer:       deps.Installer,
		reporter:        deps.Reporter,
		stateStore:      deps.StateStore,
		deployerFactory: deps.DeployerFactory,
		version:         deps.Version,
	}
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/app/...
```

Expected: compiles.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: create App struct with dependency injection"
```

---

### Task 10: Create `app/report.go` — report formatting helpers

**Files:**
- Create: `internal/app/report.go`

- [ ] **Step 1: Create `internal/app/report.go`**

Migrate `PrintApplyReport`, `PrintStatus`, and `PrintNoState` logic from the old reporter. These call `app.Reporter` interface methods.

```go
package app

import (
	"fmt"
	"time"

	"facet/internal/deploy"
)

// ValidityCheck represents the result of checking a deployed config.
type ValidityCheck struct {
	Target string
	Valid  bool
	Error  string
}

func (a *App) printApplyReport(s *ApplyState) {
	a.reporter.Header(fmt.Sprintf("Applied profile: %s", s.Profile))

	if len(s.Packages) > 0 {
		a.reporter.Header("Packages")
		for _, pkg := range s.Packages {
			switch pkg.Status {
			case "ok":
				a.reporter.Success(fmt.Sprintf("%-20s %s", pkg.Name, a.reporter.Dim(pkg.Install)))
			case "failed":
				a.reporter.Error(fmt.Sprintf("%-20s %s — failed: %s", pkg.Name, pkg.Install, pkg.Error))
			case "skipped":
				a.reporter.Warning(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			}
		}
	}

	if len(s.Configs) > 0 {
		a.reporter.Header("Configs")
		for _, cfg := range s.Configs {
			a.reporter.Success(fmt.Sprintf("%-30s → %-30s (%s)", cfg.Target, cfg.Source, cfg.Strategy))
		}
	}
}

func (a *App) printStatus(s *ApplyState, checks []ValidityCheck) {
	a.reporter.Header(fmt.Sprintf("Profile: %s", s.Profile))
	a.reporter.PrintLine(fmt.Sprintf("  Applied: %s (%s ago)", s.AppliedAt.Format(time.RFC3339), timeSince(s.AppliedAt)))

	if len(s.Packages) > 0 {
		a.reporter.Header("Packages")
		for _, pkg := range s.Packages {
			switch pkg.Status {
			case "ok":
				a.reporter.Success(fmt.Sprintf("%-20s %s", pkg.Name, a.reporter.Dim(pkg.Install)))
			case "failed":
				a.reporter.Error(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			case "skipped":
				a.reporter.Warning(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			}
		}
	}

	if len(s.Configs) > 0 {
		a.reporter.Header("Configs")
		checkMap := make(map[string]ValidityCheck)
		for _, c := range checks {
			checkMap[c.Target] = c
		}

		for _, cfg := range s.Configs {
			check, hasCheck := checkMap[cfg.Target]
			if hasCheck && !check.Valid {
				a.reporter.Error(fmt.Sprintf("%-30s → %-30s (%s) (%s)", cfg.Target, cfg.Source, cfg.Strategy, check.Error))
			} else {
				a.reporter.Success(fmt.Sprintf("%-30s → %-30s (%s)", cfg.Target, cfg.Source, cfg.Strategy))
			}
		}
	}
}

func (a *App) printNoState() {
	a.reporter.PrintLine("No profile has been applied yet.")
	a.reporter.PrintLine("Run: facet apply <profile>")
}

// configResultsToDeployConfigs converts deploy.ConfigResult to the same type.
// This is a helper for converting ApplyState.Configs to []deploy.ConfigResult
// when passing to the deployer.
func configResultsToDeployConfigs(configs []deploy.ConfigResult) []deploy.ConfigResult {
	return configs
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/app/...
```

Expected: compiles.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: add report formatting helpers to app"
```

---

### Task 11: Create `app/init.go` — Init workflow

**Files:**
- Create: `internal/app/init.go`
- Create: `internal/app/init_test.go`

- [ ] **Step 1: Create `internal/app/init.go`**

```go
package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitOpts holds options for the Init operation.
type InitOpts struct {
	ConfigDir string
	StateDir  string
}

// Init initializes a new facet config repository.
func (a *App) Init(opts InitOpts) error {
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

- [ ] **Step 2: Create `internal/app/init_test.go`**

```go
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReporter struct {
	messages []string
}

func (m *mockReporter) Success(msg string)  { m.messages = append(m.messages, "success: "+msg) }
func (m *mockReporter) Warning(msg string)  { m.messages = append(m.messages, "warning: "+msg) }
func (m *mockReporter) Error(msg string)    { m.messages = append(m.messages, "error: "+msg) }
func (m *mockReporter) Header(msg string)   { m.messages = append(m.messages, "header: "+msg) }
func (m *mockReporter) PrintLine(msg string) { m.messages = append(m.messages, "line: "+msg) }
func (m *mockReporter) Dim(text string) string { return text }

func TestInit_CreatesConfigRepo(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	r := &mockReporter{}

	a := New(Deps{Reporter: r})
	err := a.Init(InitOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(cfgDir, "facet.yaml"))
	assert.FileExists(t, filepath.Join(cfgDir, "base.yaml"))
	assert.DirExists(t, filepath.Join(cfgDir, "profiles"))
	assert.DirExists(t, filepath.Join(cfgDir, "configs"))
	assert.FileExists(t, filepath.Join(stDir, ".local.yaml"))
}

func TestInit_AlreadyInitialized(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "facet.yaml"), []byte("existing"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Init(InitOpts{ConfigDir: cfgDir, StateDir: stDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")
}

func TestInit_PreservesExistingLocalYaml(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	localPath := filepath.Join(stDir, ".local.yaml")
	require.NoError(t, os.MkdirAll(stDir, 0o755))
	require.NoError(t, os.WriteFile(localPath, []byte("custom content"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Init(InitOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	content, _ := os.ReadFile(localPath)
	assert.Equal(t, "custom content", string(content))
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/... -v -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Init workflow to app"
```

---

### Task 12: Create `app/status.go` — Status workflow

**Files:**
- Create: `internal/app/status.go`
- Create: `internal/app/status_test.go`

- [ ] **Step 1: Create `internal/app/status.go`**

```go
package app

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/internal/deploy"
)

// StatusOpts holds options for the Status operation.
type StatusOpts struct {
	ConfigDir string
	StateDir  string
}

// Status displays the current facet status.
func (a *App) Status(opts StatusOpts) error {
	s, err := a.stateStore.Read(opts.StateDir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if s == nil {
		a.printNoState()
		return nil
	}

	checks := runValidityChecks(s, opts.ConfigDir)
	a.printStatus(s, checks)

	return nil
}

// runValidityChecks checks that deployed configs are still valid.
func runValidityChecks(s *ApplyState, cfgDir string) []ValidityCheck {
	var checks []ValidityCheck

	for _, cfg := range s.Configs {
		check := ValidityCheck{Target: cfg.Target}

		info, err := os.Lstat(cfg.Target)
		if err != nil {
			check.Valid = false
			check.Error = "file missing"
			checks = append(checks, check)
			continue
		}

		switch cfg.Strategy {
		case "symlink":
			if info.Mode()&os.ModeSymlink == 0 {
				check.Valid = false
				check.Error = "expected symlink, found regular file"
			} else {
				symlinkTarget, err := os.Readlink(cfg.Target)
				if err != nil {
					check.Valid = false
					check.Error = "cannot read symlink target"
				} else if _, err := os.Stat(symlinkTarget); err != nil {
					check.Valid = false
					check.Error = "symlink target does not exist (broken symlink)"
				} else if cfgDir != "" {
					expectedSource := filepath.Join(cfgDir, cfg.Source)
					if symlinkTarget != expectedSource {
						check.Valid = false
						check.Error = fmt.Sprintf("symlink points to wrong source: got %s, want %s", symlinkTarget, expectedSource)
					} else {
						check.Valid = true
					}
				} else {
					check.Valid = true
				}
			}
		case "template":
			check.Valid = true
		default:
			check.Valid = true
		}

		checks = append(checks, check)
	}

	return checks
}

// convertConfigsToDeployResults converts ApplyState configs for use with deployer.
func convertConfigsToDeployResults(configs []deploy.ConfigResult) []deploy.ConfigResult {
	return configs
}
```

- [ ] **Step 2: Create `internal/app/status_test.go`**

```go
package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/deploy"
)

type mockStateStore struct {
	state    *ApplyState
	readErr  error
	written  *ApplyState
	writeErr error
}

func (m *mockStateStore) Read(stateDir string) (*ApplyState, error) {
	return m.state, m.readErr
}

func (m *mockStateStore) Write(stateDir string, s *ApplyState) error {
	m.written = s
	return m.writeErr
}

func (m *mockStateStore) CanaryWrite(stateDir string) error {
	return nil
}

func TestStatus_NoState(t *testing.T) {
	r := &mockReporter{}
	store := &mockStateStore{state: nil}
	a := New(Deps{Reporter: r, StateStore: store})

	err := a.Status(StatusOpts{StateDir: t.TempDir()})
	require.NoError(t, err)

	assert.Contains(t, r.messages[0], "No profile has been applied")
}

func TestStatus_WithState(t *testing.T) {
	r := &mockReporter{}
	store := &mockStateStore{
		state: &ApplyState{
			Profile:   "work",
			AppliedAt: time.Now(),
			Configs: []deploy.ConfigResult{
				{Target: "/tmp/does-not-exist", Source: "configs/.zshrc", Strategy: "symlink"},
			},
		},
	}
	a := New(Deps{Reporter: r, StateStore: store})

	err := a.Status(StatusOpts{StateDir: t.TempDir()})
	require.NoError(t, err)

	// Should have printed profile header
	found := false
	for _, msg := range r.messages {
		if assert.ObjectsAreEqual("header: Profile: work", msg) {
			found = true
		}
	}
	assert.True(t, found, "expected header with profile name")
}

func TestRunValidityChecks_SymlinkValid(t *testing.T) {
	cfgDir := t.TempDir()
	homeDir := t.TempDir()

	source := filepath.Join(cfgDir, "configs/.zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))

	target := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, target))

	s := &ApplyState{
		Configs: []deploy.ConfigResult{
			{Target: target, Source: "configs/.zshrc", Strategy: "symlink"},
		},
	}

	checks := runValidityChecks(s, cfgDir)
	require.Len(t, checks, 1)
	assert.True(t, checks[0].Valid)
}

func TestRunValidityChecks_FileMissing(t *testing.T) {
	s := &ApplyState{
		Configs: []deploy.ConfigResult{
			{Target: "/tmp/nonexistent-facet-test-path", Source: "configs/.zshrc", Strategy: "symlink"},
		},
	}

	checks := runValidityChecks(s, "")
	require.Len(t, checks, 1)
	assert.False(t, checks[0].Valid)
	assert.Equal(t, "file missing", checks[0].Error)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/... -v -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Status workflow to app"
```

---

### Task 13: Create `app/apply.go` — Apply workflow

**Files:**
- Create: `internal/app/apply.go`
- Create: `internal/app/apply_test.go`

- [ ] **Step 1: Create `internal/app/apply.go`**

```go
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"facet/internal/deploy"
	"facet/internal/profile"
)

// ApplyOpts holds options for the Apply operation.
type ApplyOpts struct {
	ConfigDir   string
	StateDir    string
	Force       bool
	SkipFailure bool
}

// Apply loads, merges, and applies a configuration profile.
func (a *App) Apply(profileName string, opts ApplyOpts) error {
	// Step 1: Load facet.yaml
	_, err := a.loader.LoadMeta(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory, or run facet init to create one.\n  detail: %w", err)
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

	// Step 4: Load .local.yaml
	localPath := filepath.Join(opts.StateDir, ".local.yaml")
	localCfg, err := a.loader.LoadConfig(localPath)
	if err != nil {
		return fmt.Errorf(".local.yaml is required in %s: %w", opts.StateDir, err)
	}

	// Step 5: Merge layers
	merged, err := profile.Merge(baseCfg, profileCfg)
	if err != nil {
		return fmt.Errorf("merge error: %w", err)
	}
	merged, err = profile.Merge(merged, localCfg)
	if err != nil {
		return fmt.Errorf("merge error with .local.yaml: %w", err)
	}

	// Step 6: Resolve variables
	resolved, err := profile.Resolve(merged)
	if err != nil {
		return err
	}

	// Step 7: Canary write to .state.json
	if err := os.MkdirAll(opts.StateDir, 0o755); err != nil {
		return fmt.Errorf("cannot create state directory %s: %w", opts.StateDir, err)
	}
	if err := a.stateStore.CanaryWrite(opts.StateDir); err != nil {
		return fmt.Errorf("cannot write state file: %w", err)
	}

	// Read previous state for unapply
	prevState, err := a.stateStore.Read(opts.StateDir)
	if err != nil {
		a.reporter.Warning(fmt.Sprintf("Could not read previous state: %v", err))
	}

	var prevConfigs []deploy.ConfigResult
	if prevState != nil {
		prevConfigs = prevState.Configs
	}

	// Unapply previous state if needed
	if prevState != nil {
		shouldUnapply := opts.Force || prevState.Profile != profileName
		if shouldUnapply {
			unapplyDeployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, nil)
			if err := unapplyDeployer.Unapply(prevState.Configs); err != nil {
				a.reporter.Warning(fmt.Sprintf("Unapply warning: %v", err))
			}
			prevConfigs = nil
		} else {
			// Same profile reapply — find orphaned configs to clean up
			newTargets := make(map[string]bool)
			for target := range resolved.Configs {
				expanded, err := deploy.ExpandPath(target)
				if err == nil {
					newTargets[expanded] = true
				}
			}
			var orphans []deploy.ConfigResult
			for _, cfg := range prevState.Configs {
				if !newTargets[cfg.Target] {
					orphans = append(orphans, cfg)
				}
			}
			if len(orphans) > 0 {
				orphanDeployer := a.deployerFactory(opts.ConfigDir, "", nil, nil)
				if err := orphanDeployer.Unapply(orphans); err != nil {
					a.reporter.Warning(fmt.Sprintf("Orphan cleanup warning: %v", err))
				}
			}
		}
	}

	// Step 8: Install packages
	pkgResults := a.installer.InstallAll(resolved.Packages)

	// Step 9: Deploy configs (sorted for deterministic order)
	deployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, prevConfigs)
	var deployErr error

	targets := make([]string, 0, len(resolved.Configs))
	for target := range resolved.Configs {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	for _, target := range targets {
		source := resolved.Configs[target]

		if err := deploy.ValidateSourcePath(source, opts.ConfigDir); err != nil {
			if opts.SkipFailure {
				a.reporter.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
				continue
			}
			deployer.Rollback()
			return fmt.Errorf("config deployment failed: %w", err)
		}

		expandedTarget, err := deploy.ExpandPath(target)
		if err != nil {
			if opts.SkipFailure {
				a.reporter.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
				continue
			}
			deployer.Rollback()
			return fmt.Errorf("config deployment failed: %w", err)
		}

		_, err = deployer.DeployOne(expandedTarget, source, opts.Force)
		if err != nil {
			if opts.SkipFailure {
				a.reporter.Warning(fmt.Sprintf("Config deploy warning: %v", err))
				continue
			}
			deployErr = err
			break
		}
	}

	if deployErr != nil {
		a.reporter.Error(fmt.Sprintf("Config deployment failed: %v", deployErr))
		a.reporter.Warning("Rolling back deployed configs...")
		deployer.Rollback()
		if prevState == nil {
			os.Remove(filepath.Join(opts.StateDir, ".state.json"))
		}
		return fmt.Errorf("config deployment failed (rolled back): %w", deployErr)
	}

	// Step 10: Write final .state.json
	applyState := &ApplyState{
		Profile:      profileName,
		AppliedAt:    time.Now().UTC(),
		FacetVersion: a.version,
		Packages:     pkgResults,
		Configs:      deployer.Deployed(),
	}

	if err := a.stateStore.Write(opts.StateDir, applyState); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	// Step 11: Print report
	a.printApplyReport(applyState)

	return nil
}
```

- [ ] **Step 2: Create `internal/app/apply_test.go`**

```go
package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)

type mockLoader struct {
	meta    *profile.FacetMeta
	configs map[string]*profile.FacetConfig
	err     error
}

func (m *mockLoader) LoadMeta(configDir string) (*profile.FacetMeta, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.meta, nil
}

func (m *mockLoader) LoadConfig(path string) (*profile.FacetConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	cfg, ok := m.configs[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return cfg, nil
}

type mockInstaller struct {
	results []packages.PackageResult
}

func (m *mockInstaller) InstallAll(pkgs []profile.PackageEntry) []packages.PackageResult {
	return m.results
}

type mockDeployer struct {
	deployed []deploy.ConfigResult
	err      error
}

func (m *mockDeployer) DeployOne(targetPath, source string, force bool) (deploy.ConfigResult, error) {
	r := deploy.ConfigResult{Target: targetPath, Source: source, Strategy: "symlink"}
	if m.err != nil {
		return r, m.err
	}
	m.deployed = append(m.deployed, r)
	return r, nil
}

func (m *mockDeployer) Unapply(configs []deploy.ConfigResult) error { return nil }
func (m *mockDeployer) Rollback() error                            { return nil }
func (m *mockDeployer) Deployed() []deploy.ConfigResult            { return m.deployed }

func TestApply_MissingFacetYaml(t *testing.T) {
	r := &mockReporter{}
	loader := &mockLoader{err: fmt.Errorf("file not found")}

	a := New(Deps{
		Reporter: r,
		Loader:   loader,
	})

	err := a.Apply("work", ApplyOpts{ConfigDir: "/fake", StateDir: t.TempDir()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "facet config directory")
}
```

Note: Add `"fmt"` to imports in apply_test.go.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/... -v -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Apply workflow to app"
```

---

## Chunk 3: Rewire cmd/ and main.go, Cleanup

This chunk replaces the cmd/ layer with thin Cobra adapters, rewires main.go with explicit DI, and cleans up deleted packages.

### Task 14: Rewrite `cmd/` — thin adapters, no globals, no init()

**Files:**
- Rewrite: `cmd/root.go`
- Rewrite: `cmd/apply.go`
- Rewrite: `cmd/init_cmd.go`
- Rewrite: `cmd/status.go`
- Delete: `cmd/integration_test.go`

- [ ] **Step 1: Rewrite `cmd/root.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

// NewRootCmd builds the full command tree and returns the root command.
func NewRootCmd(application *app.App) *cobra.Command {
	var configDir, stateDir string

	rootCmd := &cobra.Command{
		Use:     "facet",
		Short:   "Developer environment configuration manager",
		Version: "0.1.0",
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "Path to facet config repo (default: current directory)")
	rootCmd.PersistentFlags().StringVarP(&stateDir, "state-dir", "s", "", "Path to machine-local state directory (default: ~/.facet)")

	rootCmd.AddCommand(newApplyCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newInitCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newStatusCmd(application, &configDir, &stateDir))

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

Add missing imports: `"fmt"`, `"os"`.

- [ ] **Step 2: Rewrite `cmd/apply.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newApplyCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	var force, skipFailure bool

	cmd := &cobra.Command{
		Use:   "apply <profile>",
		Short: "Apply a configuration profile",
		Long:  "Loads, merges, and applies a configuration profile to this machine.",
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
			return application.Apply(args[0], app.ApplyOpts{
				ConfigDir:   cfgDir,
				StateDir:    stDir,
				Force:       force,
				SkipFailure: skipFailure,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Unapply + apply, skip prompts for conflicting files")
	cmd.Flags().BoolVar(&skipFailure, "skip-failure", false, "Warn on config deploy failure instead of rolling back")

	return cmd
}
```

- [ ] **Step 3: Rewrite `cmd/init_cmd.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newInitCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new facet config repository",
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
			return application.Init(app.InitOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			})
		},
	}
}
```

- [ ] **Step 4: Rewrite `cmd/status.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newStatusCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current facet status",
		Long:  "Displays the currently applied profile, packages, configs, and their validity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, _ := resolveConfigDir(*configDir) // best-effort for symlink source verification
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Status(app.StatusOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			})
		},
	}
}
```

- [ ] **Step 5: Delete `cmd/integration_test.go`**

```bash
rm cmd/integration_test.go
```

- [ ] **Step 6: Compile check**

```bash
go build ./cmd/...
```

Expected: compiles. (Will fully pass with main.go update in next task.)

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "refactor: rewrite cmd as thin Cobra adapters, remove init() and globals"
```

---

### Task 15: Rewrite `main.go` with explicit DI wiring

**Files:**
- Rewrite: `main.go`

- [ ] **Step 1: Rewrite `main.go`**

```go
package main

import (
	"fmt"
	"os"

	"facet/cmd"
	"facet/internal/app"
	"facet/internal/common/reporter"
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)

func main() {
	// Create concrete implementations
	loader := profile.NewLoader()
	r := reporter.NewDefault()
	runner := packages.NewShellRunner()
	osName := packages.DetectOS()
	installer := packages.NewInstaller(runner, osName)
	stateStore := app.NewFileStateStore()
	deployerFactory := func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
		return deploy.NewDeployer(configDir, homeDir, vars, ownedConfigs)
	}

	// Create app with all dependencies
	application := app.New(app.Deps{
		Loader:          loader,
		Installer:       installer,
		Reporter:        r,
		StateStore:      stateStore,
		DeployerFactory: deployerFactory,
		Version:         "0.1.0",
	})

	// Build command tree and execute
	rootCmd := cmd.NewRootCmd(application)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Run full test suite**

```bash
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 3: Build and smoke test**

```bash
go build -o facet . && ./facet --help && ./facet --version
rm facet
```

Expected: help and version output appear correctly.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: rewrite main.go with explicit DI wiring"
```

---

### Task 16: Delete old packages (`internal/state/`, `internal/reporter/`)

**Files:**
- Delete: `internal/state/`
- Delete: `internal/reporter/`

- [ ] **Step 1: Verify no remaining imports of old packages**

```bash
grep -r '"facet/internal/state"' --include='*.go' .
grep -r '"facet/internal/reporter"' --include='*.go' .
```

Expected: no results (only docs/ may have references, which is fine).

- [ ] **Step 2: Delete old packages**

```bash
rm -rf internal/state internal/reporter
```

- [ ] **Step 3: Run tests**

```bash
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "chore: delete old state/ and reporter/ packages (migrated to app/ and common/reporter/)"
```

---

### Task 17: E2E binary cleanup

**Files:**
- Modify: `e2e/e2e_test.go`

- [ ] **Step 1: Add `t.Cleanup` calls to remove built binaries**

```go
func TestE2E_Docker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found, skipping Docker E2E")
	}

	goarch := "amd64"
	if runtime.GOARCH == "arm64" {
		goarch = "arm64"
	}

	build := exec.Command("go", "build", "-o", "facet-linux", "..")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	t.Cleanup(func() { os.Remove("facet-linux") })

	// ... rest unchanged
}

func TestE2E_Native(t *testing.T) {
	build := exec.Command("go", "build", "-o", "facet", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	t.Cleanup(func() { os.Remove("facet") })

	// ... rest unchanged
}
```

- [ ] **Step 2: Commit**

```bash
git add -A && git commit -m "fix: clean up E2E binaries after test runs"
```

---

### Task 18: Update AGENTS.md with architectural rules

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Update AGENTS.md**

Add the following sections after the existing Architecture Documents table:

```markdown
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
- `app/` — orchestration (apply, init, status workflows), state persistence
- `common/reporter/` — pure terminal formatting (no business imports)

The `cmd/` layer is a thin Cobra adapter — it only parses flags and delegates to `app.App`.

### Testing
- All unit tests must be side-effect-free on the host. Use `t.TempDir()` for filesystem, `t.Setenv()` for environment variables, and mock interfaces for I/O (shell commands, etc.).
- Never use monkey patching. Use interfaces and dependency injection for testability.
- E2E tests (under `e2e/`) may execute real commands but must clean up artifacts (binaries, temp files).
```

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md && git commit -m "docs: add architectural rules to AGENTS.md"
```

---

### Task 19: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v -count=1
```

Expected: all pass.

- [ ] **Step 2: Build binary**

```bash
go build -o facet . && ./facet --help && rm facet
```

Expected: builds and runs successfully.

- [ ] **Step 3: Verify no remaining old imports**

```bash
grep -rn '"facet/internal/config"' --include='*.go' .
grep -rn '"facet/internal/state"' --include='*.go' .
grep -rn '"facet/internal/reporter"' --include='*.go' . | grep -v common
grep -rn 'func init()' --include='*.go' .
```

Expected: no matches (except possibly docs/).

- [ ] **Step 4: Verify no global mutable vars in cmd/**

```bash
grep -n '^var ' cmd/*.go
```

Expected: no results.

- [ ] **Step 5: Final commit if any cleanup needed**

```bash
git status
```
