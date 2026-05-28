# Pi Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a first-class `pi.extensions` config block that installs declared Pi extensions and removes only previously managed extensions that are no longer declared.

**Architecture:** Add a new `internal/pi` domain package for Pi extension reconciliation, extend profile parsing/merging/resolution with `PiConfig`, add `PiState` to app state, and insert a `pi` apply stage after `packages` and before `post_apply`. The manager shells out to `pi extension install/remove`, records only successful installs, and leaves unmanaged extensions untouched.

**Tech Stack:** Go 1.21+, Cobra, yaml.v3, existing Facet profile/app/state patterns, bash E2E harness with mocked CLI tools.

---

## File Structure

- Create `internal/pi/types.go` — resolved Pi config and persistent Pi state types.
- Create `internal/pi/manager.go` — `Manager` with `Apply` and `Unapply` reconciliation logic.
- Create `internal/pi/manager_test.go` — unit tests for install/remove/state behavior.
- Modify `internal/profile/types.go` — add `Pi *PiConfig` to `FacetConfig` and define `PiConfig`.
- Modify `internal/profile/merger.go` — merge `pi.extensions` by name while preserving order.
- Modify `internal/profile/resolver.go` — resolve variables in extension names and deep-copy Pi config.
- Modify profile tests — cover load/merge/resolve behavior.
- Modify `internal/app/interfaces.go` — add `PiManager` dependency interface.
- Modify `internal/app/state.go` — add `Pi *pi.PiState` to `ApplyState`.
- Modify `internal/app/apply.go` — add `pi` stage and state carry-forward/unapply behavior.
- Modify `internal/app/report.go` — dry-run, apply report, and status output for Pi extensions.
- Modify `internal/app/apply_test.go` and `internal/app/state_test.go` — app-level unit coverage.
- Modify `main.go` — wire `pi.NewManager` with `execrunner.New()`.
- Modify docs: `internal/docs/topics/config.md`, `internal/docs/topics/commands.md`, create `internal/docs/topics/pi.md`, update `internal/docs/docs.go`, `README.md`, `docs/architecture/v1-design-spec.md`.
- Modify E2E: `e2e/fixtures/mock-tools.sh`, add `e2e/suites/18-pi-extensions.sh`.

---

### Task 1: Profile schema, merge, and resolve

**Files:**
- Modify: `internal/profile/types.go`
- Modify: `internal/profile/merger.go`
- Modify: `internal/profile/resolver.go`
- Test: `internal/profile/loader_test.go`
- Test: `internal/profile/merger_test.go`
- Test: `internal/profile/resolver_test.go`

- [ ] **Step 1: Add failing loader test for `pi.extensions`**

Add to `internal/profile/loader_test.go`:

```go
func TestLoadConfig_WithPiExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`pi:
  extensions:
    - pi-interactive-shell
    - "@gotgenes/pi-session-tools"
`), 0o644))

	cfg, err := NewLoader().LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Pi)
	assert.Equal(t, []string{"pi-interactive-shell", "@gotgenes/pi-session-tools"}, cfg.Pi.Extensions)
}
```

Run:

```bash
go test ./internal/profile -run TestLoadConfig_WithPiExtensions -count=1
```

Expected: FAIL because `FacetConfig` does not yet have a `Pi` field.

- [ ] **Step 2: Add profile types**

In `internal/profile/types.go`, add `Pi` to `FacetConfig`:

```go
Pi *PiConfig `yaml:"pi,omitempty"`
```

Add below `ScriptEntry` or near AI types:

```go
// PiConfig holds Pi-specific configuration.
type PiConfig struct {
	Extensions []string `yaml:"extensions,omitempty"`
}
```

- [ ] **Step 3: Verify loader test passes**

Run:

```bash
go test ./internal/profile -run TestLoadConfig_WithPiExtensions -count=1
```

Expected: PASS.

- [ ] **Step 4: Add failing merge tests**

Add to `internal/profile/merger_test.go`:

```go
func TestMergePiExtensions_UnionsByName(t *testing.T) {
	base := &FacetConfig{Pi: &PiConfig{Extensions: []string{"pi-lens", "pi-subagents"}}}
	overlay := &FacetConfig{Pi: &PiConfig{Extensions: []string{"pi-subagents", "@gotgenes/pi-session-tools"}}}

	merged, err := Merge(base, overlay)
	require.NoError(t, err)
	require.NotNil(t, merged.Pi)
	assert.Equal(t, []string{"pi-lens", "pi-subagents", "@gotgenes/pi-session-tools"}, merged.Pi.Extensions)
}

func TestMergePi_NilWhenAbsent(t *testing.T) {
	merged, err := Merge(&FacetConfig{}, &FacetConfig{})
	require.NoError(t, err)
	assert.Nil(t, merged.Pi)
}
```

Run:

```bash
go test ./internal/profile -run 'TestMergePi' -count=1
```

Expected: FAIL because merge does not populate `Pi`.

- [ ] **Step 5: Implement Pi merge**

In `internal/profile/merger.go`, add after AI merge:

```go
// Merge Pi config
result.Pi = mergePi(base.Pi, overlay.Pi)
```

Add helper functions:

```go
func mergePi(base, overlay *PiConfig) *PiConfig {
	if base == nil && overlay == nil {
		return nil
	}
	result := &PiConfig{}
	seen := make(map[string]struct{})
	for _, ext := range appendPiExtensions(nil, base) {
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		result.Extensions = append(result.Extensions, ext)
	}
	for _, ext := range appendPiExtensions(nil, overlay) {
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		result.Extensions = append(result.Extensions, ext)
	}
	if len(result.Extensions) == 0 {
		return result
	}
	return result
}

func appendPiExtensions(dst []string, cfg *PiConfig) []string {
	if cfg == nil {
		return dst
	}
	return append(dst, cfg.Extensions...)
}
```

- [ ] **Step 6: Verify merge tests pass**

Run:

```bash
go test ./internal/profile -run 'TestMergePi' -count=1
```

Expected: PASS.

- [ ] **Step 7: Add failing resolve tests**

Add to `internal/profile/resolver_test.go`:

```go
func TestResolvePiExtensions_SubstitutesVariables(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"pi_ext": "pi-lens"},
		Pi:   &PiConfig{Extensions: []string{"${facet:pi_ext}", "pi-subagents"}},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	require.NotNil(t, resolved.Pi)
	assert.Equal(t, []string{"pi-lens", "pi-subagents"}, resolved.Pi.Extensions)
}

func TestResolvePiExtensions_UndefinedVariableErrors(t *testing.T) {
	cfg := &FacetConfig{Pi: &PiConfig{Extensions: []string{"${facet:missing}"}}}

	_, err := Resolve(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pi.extensions[0]")
}
```

Run:

```bash
go test ./internal/profile -run 'TestResolvePiExtensions' -count=1
```

Expected: FAIL because `Resolve` does not copy/resolve Pi config.

- [ ] **Step 8: Implement Pi resolve**

In `internal/profile/resolver.go`, after AI resolution, add:

```go
resolvedPi, err := resolvePi(cfg.Pi, cfg.Vars)
if err != nil {
	return nil, err
}
result.Pi = resolvedPi
```

Add helper:

```go
func resolvePi(piCfg *PiConfig, vars map[string]any) (*PiConfig, error) {
	if piCfg == nil {
		return nil, nil
	}
	result := &PiConfig{}
	if piCfg.Extensions != nil {
		result.Extensions = make([]string, len(piCfg.Extensions))
		for i, ext := range piCfg.Extensions {
			resolved, err := substituteVars(ext, vars)
			if err != nil {
				return nil, fmt.Errorf("pi.extensions[%d]: %w", i, err)
			}
			result.Extensions[i] = resolved
		}
	}
	return result, nil
}
```

- [ ] **Step 9: Verify profile package**

Run:

```bash
go test ./internal/profile -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/profile
git commit -m "feat(profile): model pi extensions config"
```

---

### Task 2: Pi extension manager domain package

**Files:**
- Create: `internal/pi/types.go`
- Create: `internal/pi/manager.go`
- Create: `internal/pi/manager_test.go`

- [ ] **Step 1: Write failing manager tests**

Create `internal/pi/manager_test.go`:

```go
package pi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	commands []string
	fail     map[string]error
}

func (m *mockRunner) Run(name string, args ...string) error {
	cmd := name
	for _, arg := range args {
		cmd += " " + arg
	}
	m.commands = append(m.commands, cmd)
	if err, ok := m.fail[cmd]; ok {
		return err
	}
	return nil
}

func (m *mockRunner) RunInteractive(name string, args ...string) error {
	return m.Run(name, args...)
}

type mockReporter struct {
	warnings []string
	success  []string
}

func (m *mockReporter) Success(msg string) { m.success = append(m.success, msg) }
func (m *mockReporter) Warning(msg string) { m.warnings = append(m.warnings, msg) }

func TestManagerApply_InstallsCurrentAndRemovesOrphans(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	state, err := mgr.Apply(&Config{Extensions: []string{"pi-lens", "pi-subagents"}}, &PiState{Extensions: []string{"pi-lens", "old-ext"}})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"pi extension remove old-ext",
		"pi extension install pi-lens",
		"pi extension install pi-subagents",
	}, runner.commands)
	assert.Equal(t, []string{"pi-lens", "pi-subagents"}, state.Extensions)
}

func TestManagerApply_RecordsOnlySuccessfulInstalls(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{
		"pi extension install broken-ext": fmt.Errorf("boom"),
	}}
	reporter := &mockReporter{}
	mgr := NewManager(runner, reporter)

	state, err := mgr.Apply(&Config{Extensions: []string{"ok-ext", "broken-ext"}}, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"ok-ext"}, state.Extensions)
	assert.Len(t, reporter.warnings, 1)
}

func TestManagerApply_NilConfigRemovesPreviousManagedExtensions(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	state, err := mgr.Apply(nil, &PiState{Extensions: []string{"pi-lens"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"pi extension remove pi-lens"}, runner.commands)
	assert.Nil(t, state)
}

func TestManagerUnapply_RemovesPreviousManagedExtensions(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	require.NoError(t, mgr.Unapply(&PiState{Extensions: []string{"pi-lens", "pi-subagents"}}))
	assert.Equal(t, []string{
		"pi extension remove pi-lens",
		"pi extension remove pi-subagents",
	}, runner.commands)
}
```

Run:

```bash
go test ./internal/pi -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 2: Add Pi types**

Create `internal/pi/types.go`:

```go
package pi

// Config is the effective Pi configuration to apply.
type Config struct {
	Extensions []string
}

// PiState records Pi extensions managed by Facet.
type PiState struct {
	Extensions []string `json:"extensions,omitempty"`
}
```

- [ ] **Step 3: Add manager implementation**

Create `internal/pi/manager.go`:

```go
package pi

import (
	"fmt"
	"sort"
)

// CommandRunner executes commands directly without a shell.
type CommandRunner interface {
	Run(name string, args ...string) error
	RunInteractive(name string, args ...string) error
}

// Reporter emits user-facing status messages.
type Reporter interface {
	Success(msg string)
	Warning(msg string)
}

// Manager reconciles Pi extension state.
type Manager struct {
	runner   CommandRunner
	reporter Reporter
}

func NewManager(runner CommandRunner, reporter Reporter) *Manager {
	return &Manager{runner: runner, reporter: reporter}
}

func (m *Manager) Apply(config *Config, previousState *PiState) (*PiState, error) {
	current := make(map[string]struct{})
	if config != nil {
		for _, ext := range config.Extensions {
			if ext == "" {
				continue
			}
			current[ext] = struct{}{}
		}
	}

	if previousState != nil {
		for _, ext := range previousState.Extensions {
			if _, keep := current[ext]; keep {
				continue
			}
			if err := m.runner.Run("pi", "extension", "remove", ext); err != nil {
				m.reporter.Warning(fmt.Sprintf("failed to remove Pi extension %q: %v", ext, err))
			} else {
				m.reporter.Success(fmt.Sprintf("removed Pi extension %s", ext))
			}
		}
	}

	if len(current) == 0 {
		return nil, nil
	}

	extensions := sortedKeys(current)
	state := &PiState{}
	for _, ext := range extensions {
		if err := m.runner.Run("pi", "extension", "install", ext); err != nil {
			m.reporter.Warning(fmt.Sprintf("failed to install Pi extension %q: %v", ext, err))
			continue
		}
		m.reporter.Success(fmt.Sprintf("installed Pi extension %s", ext))
		state.Extensions = append(state.Extensions, ext)
	}
	if len(state.Extensions) == 0 {
		return nil, nil
	}
	return state, nil
}

func (m *Manager) Unapply(previousState *PiState) error {
	if previousState == nil {
		return nil
	}
	for _, ext := range previousState.Extensions {
		if err := m.runner.Run("pi", "extension", "remove", ext); err != nil {
			m.reporter.Warning(fmt.Sprintf("failed to remove Pi extension %q: %v", ext, err))
			continue
		}
		m.reporter.Success(fmt.Sprintf("removed Pi extension %s", ext))
	}
	return nil
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
```

- [ ] **Step 4: Verify manager tests pass**

Run:

```bash
go test ./internal/pi -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pi
git commit -m "feat(pi): add extension manager"
```

---

### Task 3: App integration and state reconciliation

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/state.go`
- Modify: `internal/app/apply.go`
- Modify: `internal/app/report.go`
- Modify: `internal/app/apply_test.go`
- Modify: `internal/app/state_test.go`
- Modify: `main.go`

- [ ] **Step 1: Add failing app test for Pi apply**

In `internal/app/apply_test.go`, add imports for `facet/internal/pi` if not auto-added.

Add test doubles near `mockAIOrchestrator`:

```go
type mockPiManager struct {
	applyCalled   bool
	applyConfig   *pi.Config
	applyPrev     *pi.PiState
	applyResult   *pi.PiState
	applyErr      error
	unapplyCalled bool
	unapplyPrev   *pi.PiState
	unapplyErr    error
}

func (m *mockPiManager) Apply(config *pi.Config, previousState *pi.PiState) (*pi.PiState, error) {
	m.applyCalled = true
	m.applyConfig = config
	m.applyPrev = previousState
	return m.applyResult, m.applyErr
}

func (m *mockPiManager) Unapply(previousState *pi.PiState) error {
	m.unapplyCalled = true
	m.unapplyPrev = previousState
	return m.unapplyErr
}
```

Add test:

```go
func TestApply_WithPiExtensions(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	piMgr := &mockPiManager{applyResult: &pi.PiState{Extensions: []string{"pi-lens"}}}
	stateStore := &mockStateStore{}
	baseCfg := &profile.FacetConfig{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): baseCfg,
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				Pi:      &profile.PiConfig{Extensions: []string{"pi-lens"}},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		BaseResolver: newStaticBaseResolver(baseCfg),
		Installer:    &mockInstaller{},
		StateStore:   stateStore,
		PiManager:    piMgr,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return &mockDeployer{}
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{ConfigDir: cfgDir, StateDir: stateDir})
	require.NoError(t, err)
	assert.True(t, piMgr.applyCalled)
	require.NotNil(t, piMgr.applyConfig)
	assert.Equal(t, []string{"pi-lens"}, piMgr.applyConfig.Extensions)
	require.NotNil(t, stateStore.written.Pi)
	assert.Equal(t, []string{"pi-lens"}, stateStore.written.Pi.Extensions)
}
```

Run:

```bash
go test ./internal/app -run TestApply_WithPiExtensions -count=1
```

Expected: FAIL because app dependencies/state do not support Pi.

- [ ] **Step 2: Add app interface and state field**

In `internal/app/interfaces.go`, import `facet/internal/pi` and add:

```go
// PiManager handles Pi extension lifecycle.
type PiManager interface {
	Apply(config *pi.Config, previousState *pi.PiState) (*pi.PiState, error)
	Unapply(previousState *pi.PiState) error
}
```

In `internal/app/app.go`, add to `Deps` and `App` structs:

```go
PiManager PiManager
```

and wire it in `New`.

In `internal/app/state.go`, import `facet/internal/pi` and add to `ApplyState`:

```go
Pi *pi.PiState `json:"pi,omitempty"`
```

- [ ] **Step 3: Add Pi stage name**

In `internal/app/apply.go`, change `validStages` to:

```go
return []string{"configs", "pre_apply", "packages", "pi", "post_apply", "ai"}
```

- [ ] **Step 4: Add Pi state variables and unapply handling**

In `Apply`, track previous Pi state near previous AI state:

```go
var prevPiState *pi.PiState
```

When reading previous state:

```go
prevPiState = prevState.Pi
```

In previous-state unapply, add:

```go
willUnapplyPi := stages["pi"] && a.piManager != nil && prevState.Pi != nil
```

Include it in the condition and call:

```go
if willUnapplyPi {
	if err := a.reporter.ProgressStep("  -> Pi unapply", func() error {
		return a.piManager.Unapply(prevState.Pi)
	}); err != nil {
		a.reporter.Error(fmt.Sprintf("Pi unapply failed: %v", err))
	}
	prevPiState = nil
}
```

- [ ] **Step 5: Add Pi apply stage**

After packages and before post_apply in `internal/app/apply.go`, add:

```go
var piState *pi.PiState
var effectivePi *pi.Config
if resolved.Pi != nil {
	effectivePi = &pi.Config{Extensions: append([]string{}, resolved.Pi.Extensions...)}
}
if stages["pi"] && a.piManager != nil && (effectivePi != nil || prevPiState != nil) {
	done := a.reporter.ProgressStart("Applying Pi extensions")
	var piErr error
	piState, piErr = a.piManager.Apply(effectivePi, prevPiState)
	if piErr != nil {
		a.reporter.Error(fmt.Sprintf("Pi extensions failed: %v", piErr))
		done("failed", piErr)
	} else {
		done("done", nil)
	}
}
```

Before writing state, carry forward skipped stage:

```go
if !stages["pi"] && prevState != nil {
	piState = prevState.Pi
}
```

Add to `ApplyState` literal:

```go
Pi: piState,
```

- [ ] **Step 6: Verify Pi apply test passes**

Run:

```bash
go test ./internal/app -run TestApply_WithPiExtensions -count=1
```

Expected: PASS.

- [ ] **Step 7: Add app tests for removal, force/profile switch, and stage skip**

Add to `internal/app/apply_test.go`:

```go
func TestApply_SameProfileRemovalOfPiSectionReconcilesPreviousState(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	piMgr := &mockPiManager{}
	stateStore := &mockStateStore{state: &ApplyState{Profile: "work", Pi: &pi.PiState{Extensions: []string{"pi-lens"}}}}
	baseCfg := &profile.FacetConfig{}
	loader := &mockLoader{meta: &profile.FacetMeta{}, configs: map[string]*profile.FacetConfig{
		filepath.Join(cfgDir, "base.yaml"):             baseCfg,
		filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
		filepath.Join(stateDir, ".local.yaml"):         {},
	}}

	a := New(Deps{Reporter: &mockReporter{}, Loader: loader, BaseResolver: newStaticBaseResolver(baseCfg), Installer: &mockInstaller{}, StateStore: stateStore, PiManager: piMgr, DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service { return &mockDeployer{} }, OSName: "macos"})
	require.NoError(t, a.Apply("work", ApplyOpts{ConfigDir: cfgDir, StateDir: stateDir}))
	assert.True(t, piMgr.applyCalled)
	assert.Nil(t, piMgr.applyConfig)
	assert.Nil(t, stateStore.written.Pi)
}

func TestApply_ProfileSwitchTriggersPiUnapply(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	prevPi := &pi.PiState{Extensions: []string{"pi-lens"}}
	piMgr := &mockPiManager{}
	stateStore := &mockStateStore{state: &ApplyState{Profile: "work", Pi: prevPi}}
	baseCfg := &profile.FacetConfig{}
	loader := &mockLoader{meta: &profile.FacetMeta{}, configs: map[string]*profile.FacetConfig{
		filepath.Join(cfgDir, "base.yaml"): baseCfg,
		filepath.Join(cfgDir, "profiles", "personal.yaml"): {Extends: "base", Pi: &profile.PiConfig{Extensions: []string{"pi-subagents"}}},
		filepath.Join(stateDir, ".local.yaml"): {},
	}}

	a := New(Deps{Reporter: &mockReporter{}, Loader: loader, BaseResolver: newStaticBaseResolver(baseCfg), Installer: &mockInstaller{}, StateStore: stateStore, PiManager: piMgr, DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service { return &mockDeployer{} }, OSName: "macos"})
	require.NoError(t, a.Apply("personal", ApplyOpts{ConfigDir: cfgDir, StateDir: stateDir}))
	assert.True(t, piMgr.unapplyCalled)
	assert.Equal(t, prevPi, piMgr.unapplyPrev)
	assert.True(t, piMgr.applyCalled)
	assert.Nil(t, piMgr.applyPrev)
}

func TestApply_StagesPackagesPreservesPreviousPiState(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	prevPi := &pi.PiState{Extensions: []string{"pi-lens"}}
	piMgr := &mockPiManager{}
	stateStore := &mockStateStore{state: &ApplyState{Profile: "work", Pi: prevPi}}
	baseCfg := &profile.FacetConfig{Packages: []profile.PackageEntry{{Name: "git", Install: profile.InstallCmd{Command: "true"}}}}
	loader := &mockLoader{meta: &profile.FacetMeta{}, configs: map[string]*profile.FacetConfig{
		filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
		filepath.Join(stateDir, ".local.yaml"):         {},
	}}

	installerCalled := false
	a := New(Deps{Reporter: &mockReporter{}, Loader: loader, BaseResolver: newStaticBaseResolver(baseCfg), Installer: &trackingInstaller{called: &installerCalled}, StateStore: stateStore, PiManager: piMgr, DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service { return &mockDeployer{} }, OSName: "macos"})
	require.NoError(t, a.Apply("work", ApplyOpts{ConfigDir: cfgDir, StateDir: stateDir, Stages: "packages"}))
	assert.True(t, installerCalled)
	assert.False(t, piMgr.applyCalled)
	assert.Equal(t, prevPi, stateStore.written.Pi)
}
```

Run:

```bash
go test ./internal/app -run 'TestApply_.*Pi|TestApply_StagesPackagesPreservesPreviousPiState' -count=1
```

Expected: PASS.

- [ ] **Step 8: Add state persistence test**

In `internal/app/state_test.go`, add import `facet/internal/pi` and test:

```go
func TestStateStore_RoundTripsPiState(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()
	original := &ApplyState{
		Profile: "work",
		AppliedAt: time.Now().UTC(),
		FacetVersion: "test",
		Pi: &pi.PiState{Extensions: []string{"pi-lens", "pi-subagents"}},
	}

	require.NoError(t, store.Write(dir, original))
	loaded, err := store.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded.Pi)
	assert.Equal(t, []string{"pi-lens", "pi-subagents"}, loaded.Pi.Extensions)
}
```

Run:

```bash
go test ./internal/app -run TestStateStore_RoundTripsPiState -count=1
```

Expected: PASS.

- [ ] **Step 9: Add reporting/dry-run output**

In `internal/app/report.go`, add Pi printing in `printApplyReport` and `printStatus` before AI:

```go
if s.Pi != nil {
	a.printPiState(s.Pi)
}
```

In `printDryRun`, after packages and before post-apply scripts:

```go
if resolved.Pi != nil && len(resolved.Pi.Extensions) > 0 {
	a.reporter.Header("Pi extensions to install")
	for _, ext := range resolved.Pi.Extensions {
		a.reporter.Success(ext)
	}
}
```

Add helper:

```go
func (a *App) printPiState(piState *pi.PiState) {
	a.reporter.Header("Pi")
	for _, ext := range piState.Extensions {
		a.reporter.Success(fmt.Sprintf("Extension: %s", ext))
	}
}
```

Ensure `internal/app/report.go` imports `facet/internal/pi`.

- [ ] **Step 10: Wire main**

In `main.go`, import `facet/internal/pi` if not auto-added.

After `providers` or near AI setup:

```go
piManager := pi.NewManager(aiRunner, r)
```

In `app.New(app.Deps{...})`, add:

```go
PiManager: piManager,
```

- [ ] **Step 11: Verify app package and build**

Run:

```bash
go test ./internal/app -count=1
go test ./internal/pi -count=1
go build ./...
```

Expected: PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/app internal/pi main.go
git commit -m "feat(app): apply pi extension state"
```

---

### Task 4: Documentation updates

**Files:**
- Modify: `internal/docs/docs.go`
- Create: `internal/docs/topics/pi.md`
- Modify: `internal/docs/topics/config.md`
- Modify: `internal/docs/topics/commands.md`
- Modify: `README.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Add `pi` docs topic**

Create `internal/docs/topics/pi.md`:

```markdown
# Pi Extensions

The `pi:` block manages Pi coding-agent extensions during `facet apply`.

```yaml
pi:
  extensions:
    - pi-interactive-shell
    - pi-lens
    - pi-subagents
    - "@juicesharp/rpiv-btw"
    - "@gotgenes/pi-session-tools"
```

## Behavior

Facet installs every declared extension with:

```sh
pi extension install <name>
```

Facet removes only extensions that were previously managed by Facet and are no
longer declared in the resolved config:

```sh
pi extension remove <name>
```

Manually installed Pi extensions are left untouched.

## Stage

Pi extensions run in the `pi` apply stage, after `packages` and before
`post_apply`:

```text
configs → pre_apply → packages → pi → post_apply → ai
```

Use `--stages pi` to run only Pi extension reconciliation.
```

- [ ] **Step 2: Register docs topic**

In `internal/docs/docs.go`, add a topic entry:

```go
{Name: "pi", Description: "Pi extension management"},
```

- [ ] **Step 3: Update config docs**

In `internal/docs/topics/config.md`, add `pi` to the schema table:

```markdown
| `pi` | PiConfig | Pi coding-agent extension configuration. See `facet docs pi`. |
```

Add a short `base.yaml` example under the existing example:

```yaml
pi:
  extensions:
    - pi-lens
    - pi-subagents
```

- [ ] **Step 4: Update commands docs**

In `internal/docs/topics/commands.md`, update stage order text and valid stages table to include:

```markdown
| `pi` | Install/remove managed Pi extensions |
```

The final numbered apply flow should place Pi after packages and before post-apply.

- [ ] **Step 5: Update README**

In `README.md`, add Pi extensions to the feature list/table if applicable and add a section near AI configuration:

```markdown
## Pi Extensions

facet can manage Pi coding-agent extensions as first-class state:

```yaml
pi:
  extensions:
    - pi-interactive-shell
    - pi-lens
    - pi-subagents
```

During `facet apply`, Facet installs declared extensions and removes only those
previously managed by Facet that are no longer declared. Manually installed Pi
extensions are not touched.
```

- [ ] **Step 6: Update architecture spec**

In `docs/architecture/v1-design-spec.md`, add a concise section documenting:

- top-level `pi.extensions`
- stage order including `pi`
- state-scoped removal semantics
- `pi extension install/remove` commands

- [ ] **Step 7: Verify docs build/tests**

Run:

```bash
go test ./internal/docs -count=1
go build ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/docs README.md docs/architecture/v1-design-spec.md
git commit -m "docs: document pi extension management"
```

---

### Task 5: E2E tests and mock Pi CLI

**Files:**
- Modify: `e2e/fixtures/mock-tools.sh`
- Create: `e2e/suites/18-pi-extensions.sh`

- [ ] **Step 1: Add mock `pi` CLI**

In `e2e/fixtures/mock-tools.sh`, add after the `npx` mock:

```bash
# ── Mock pi (for Pi extension management) ──
MOCK_PI_LOG="$HOME/.mock-pi"
touch "$MOCK_PI_LOG"

cat > "$HOME/mock-bin/pi" << 'PIEOF'
#!/bin/bash
MOCK_PI_LOG="$HOME/.mock-pi"
MOCK_PI_EXTENSIONS="$HOME/.mock-pi-extensions"
touch "$MOCK_PI_EXTENSIONS"

echo "pi $*" >> "$MOCK_PI_LOG"

if [ "$1" = "extension" ] && [ "$2" = "install" ]; then
    name="$3"
    grep -qx "$name" "$MOCK_PI_EXTENSIONS" 2>/dev/null || echo "$name" >> "$MOCK_PI_EXTENSIONS"
    echo "mock-pi: extension install $name"
    exit 0
fi

if [ "$1" = "extension" ] && [ "$2" = "remove" ]; then
    name="$3"
    grep -vx "$name" "$MOCK_PI_EXTENSIONS" > "$MOCK_PI_EXTENSIONS.tmp" || true
    mv "$MOCK_PI_EXTENSIONS.tmp" "$MOCK_PI_EXTENSIONS"
    echo "mock-pi: extension remove $name"
    exit 0
fi

if [ "$1" = "extension" ] && [ "$2" = "list" ]; then
    cat "$MOCK_PI_EXTENSIONS"
    exit 0
fi

echo "mock-pi: $*"
exit 0
PIEOF
chmod +x "$HOME/mock-bin/pi"
```

- [ ] **Step 2: Add E2E suite**

Create `e2e/suites/18-pi-extensions.sh`:

```bash
#!/bin/bash
# e2e/suites/18-pi-extensions.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  pi_extra: "@gotgenes/pi-session-tools"

packages:
  - name: pi-agent
    install: echo install-pi-agent

pi:
  extensions:
    - pi-lens
    - pi-subagents
    - "${facet:pi_extra}"
YAML

cat > "$HOME/dotfiles/profiles/work.yaml" << 'YAML'
extends: base

pi:
  extensions:
    - pi-subagents
    - pi-interactive-shell
YAML

facet_apply work
assert_file_exists "$HOME/.mock-pi"
assert_file_contains "$HOME/.mock-pi" "pi extension install pi-lens"
assert_file_contains "$HOME/.mock-pi" "pi extension install pi-subagents"
assert_file_contains "$HOME/.mock-pi" "pi extension install @gotgenes/pi-session-tools"
assert_file_contains "$HOME/.mock-pi" "pi extension install pi-interactive-shell"
assert_json_field "$HOME/.facet/.state.json" '.pi.extensions[0]' '@gotgenes/pi-session-tools'
echo "  pi extensions installed and recorded"

: > "$HOME/.mock-pi"
cat > "$HOME/dotfiles/profiles/work.yaml" << 'YAML'
extends: base

pi:
  extensions:
    - pi-lens
YAML
facet_apply work
assert_file_contains "$HOME/.mock-pi" "pi extension remove pi-interactive-shell"
assert_file_contains "$HOME/.mock-pi" "pi extension remove pi-subagents"
assert_file_contains "$HOME/.mock-pi" "pi extension install pi-lens"
echo "  removed only previously managed undeclared extensions"

: > "$HOME/.mock-pi"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work --stages packages
if [ -s "$HOME/.mock-pi" ]; then
    echo "  ASSERT FAIL: --stages packages should not run pi extension commands"
    cat "$HOME/.mock-pi"
    exit 1
fi
echo "  --stages packages skips pi extensions"

output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --dry-run work 2>&1)
echo "$output" | grep -q "Pi extensions" || { echo "  ASSERT FAIL: dry-run should show Pi extensions"; echo "$output"; exit 1; }
echo "  dry-run shows pi extension preview"
```

- [ ] **Step 3: Run E2E suite**

Run:

```bash
bash e2e/harness.sh e2e/suites/18-pi-extensions.sh
```

Expected: PASS.

- [ ] **Step 4: Run full E2E if feasible**

Run:

```bash
bash e2e/harness.sh
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add e2e/fixtures/mock-tools.sh e2e/suites/18-pi-extensions.sh
git commit -m "test(e2e): cover pi extension management"
```

---

### Task 6: Apply config repo update and final verification

**Files:**
- Modify: `/Users/bytedance/aec/src/github/facet-profiles/base.yaml`

- [ ] **Step 1: Update Facet profiles base config**

In `/Users/bytedance/aec/src/github/facet-profiles/base.yaml`, add after the `pi-agent` package or after the package list:

```yaml
pi:
  extensions:
    - pi-interactive-shell
    - pi-lens
    - pi-subagents
    - "@juicesharp/rpiv-btw"
    - "@juicesharp/rpiv-ask-user-question"
    - "@gotgenes/pi-session-tools"
```

Do not remove the existing `ai:` block; `pi:` is separate.

- [ ] **Step 2: Validate YAML and profile loading**

Run from `/Users/bytedance/aec/src/github/facet`:

```bash
go test ./internal/profile -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full unit tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run required pre-commit verification**

Run:

```bash
make pre-commit
```

Expected: PASS. If Docker is unavailable, record the exact failure and run at minimum:

```bash
go test ./...
bash e2e/harness.sh
```

- [ ] **Step 5: Commit profile config update if it belongs in this repo branch**

If the profile repo is tracked separately, commit there separately:

```bash
cd /Users/bytedance/aec/src/github/facet-profiles
git status --short
git add base.yaml
git commit -m "feat: manage pi extensions"
```

For the `facet` repo, commit remaining verification/doc changes:

```bash
cd /Users/bytedance/aec/src/github/facet
git status --short
git add .
git commit -m "feat: manage pi extensions"
```

- [ ] **Step 6: Create PR**

Push and create the GitHub PR from the `facet` repo:

```bash
cd /Users/bytedance/aec/src/github/facet
git push -u origin feat/pi-extensions
gh pr create --fill
```

If `gh` is not authenticated, use the URL printed by `git push` or run `gh auth login`.

---

## Self-Review

- Spec coverage: top-level `pi.extensions`, state-scoped removal, dedicated stage, dry-run/status/reporting, docs, unit tests, E2E tests, and profile update are covered.
- Placeholder scan: no TBD/TODO placeholders remain; all tasks have concrete files, code, commands, and expected outcomes.
- Type consistency: profile type is `profile.PiConfig`, domain config is `pi.Config`, persisted state is `pi.PiState`, app interface is `PiManager`.
