# Remote Extends Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a local profile inherit `base.yaml` from an HTTPS/SSH git repo, local directory, or local file, while keeping the selected profile local, materializing remote-base configs, and preserving hermetic docs/E2E coverage.

**Architecture:** Parse `extends` into an internal spec, resolve it into a concrete base layer before merge, and carry per-layer provenance through merge and apply. Remote-base configs use their own source root and are always materialized so cleanup can delete the fresh clone at the end of `apply`; local profile configs keep today’s symlink/template behavior.

**Tech Stack:** Go, YAML, git CLI, bash E2E harness, Docker-backed `make pre-commit`

---

## File Map

- `internal/profile/extends.go`
  New parser for raw `extends` strings and small helpers for classifying git/local directory/local file inputs.
- `internal/profile/extends_test.go`
  Parser coverage for HTTPS, SSH, local path, refs, and malformed inputs.
- `internal/profile/types.go`
  Internal-only provenance fields so merged configs can remember source roots and remote/local materialization intent.
- `internal/profile/loader.go`
  Replace the hard-coded `extends: base` validation with locator parsing.
- `internal/profile/loader_test.go`
  Update validation tests for the new `extends` semantics.
- `internal/profile/merger.go`
  Merge provenance maps and script working directories alongside existing config data.
- `internal/profile/merger_test.go`
  Regression coverage for provenance merge and deep-copy behavior.
- `internal/profile/resolver.go`
  Preserve provenance metadata through variable substitution.
- `internal/profile/resolver_test.go`
  Cover config-source substitution without losing source-root information.
- `internal/profile/base_resolver.go`
  Resolve `extends` into a loaded base config plus cleanup, including fresh git clone flow.
- `internal/profile/base_resolver_test.go`
  Hermetic local-git tests for branch/tag/commit/default-branch and cleanup.
- `internal/app/interfaces.go`
  Add the base-resolver dependency where it is consumed.
- `internal/app/app.go`
  Store the new dependency on `App`.
- `internal/app/apply.go`
  Resolve the base before load/merge, defer cleanup, and use provenance-aware source/script handling.
- `internal/app/report.go`
  Dry-run output must understand remote materialization and resolved source roots.
- `internal/app/apply_test.go`
  App-level tests for merge order, cleanup, script directories, and remote/local config behavior.
- `internal/app/status.go`
  Keep status checks correct for symlinked local configs and materialized remote configs.
- `internal/app/status_test.go`
  Coverage for the new status semantics.
- `internal/deploy/deployer.go`
  Accept resolved source paths and remote-only materialization strategy.
- `internal/deploy/deployer_test.go`
  Copy/materialize tests plus local symlink regression coverage.
- `internal/deploy/pathexpand.go`
  Replace “relative-only” validation with provenance-aware source resolution.
- `internal/deploy/pathexpand_test.go`
  Tests for relative local sources and absolute remote-resolved sources.
- `main.go`
  Wire the concrete base resolver and git command runner explicitly.
- `e2e/suites/15-remote-extends.sh`
  Full hermetic end-to-end coverage for local dir/file and full git repo flows without network.
- `README.md`
  Update the public contract for `extends`, merge order, and remote materialization behavior.
- `internal/docs/topics/config.md`
  Update the embedded config reference.
- `internal/docs/topics/merge.md`
  Update layer order and remote/local source semantics.
- `internal/docs/topics/quickstart.md`
  Mention non-`base` extends forms in the user flow.
- `internal/docs/topics/commands.md`
  Update `apply` description to reflect external-base resolution.
- `internal/docs/topics/examples.md`
  Add examples for git/local extends and remote-base config behavior.
- `internal/docs/topics/variables.md`
  Clarify that config source values still undergo `${facet:...}` substitution before provenance-aware resolution.
- `docs/architecture/v1-design-spec.md`
  Bring the authoritative spec in line with the shipped behavior.

### Repository Rule For Commits

This repository does not allow committing incomplete feature work before docs and E2E are updated. Do not create intermediate feature commits before the documentation and E2E tasks are done. Use targeted tests for feedback during implementation, then run `make pre-commit` and commit once the whole slice is complete.

---

### Task 1: Lock The New `extends` Contract In Tests

**Files:**
- Create: `internal/profile/extends_test.go`
- Modify: `internal/profile/loader_test.go`

- [ ] **Step 1: Write failing parser tests for the supported locator shapes**

Create `internal/profile/extends_test.go` with focused parser coverage:

```go
package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExtends_HTTPSDefaultBranch(t *testing.T) {
	spec, err := ParseExtends("https://github.com/me/personal-dotfiles.git")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "https://github.com/me/personal-dotfiles.git", spec.Locator)
	assert.Empty(t, spec.Ref)
}

func TestParseExtends_HTTPSWithBranch(t *testing.T) {
	spec, err := ParseExtends("https://github.com/me/personal-dotfiles.git@main")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "main", spec.Ref)
}

func TestParseExtends_SSHWithTag(t *testing.T) {
	spec, err := ParseExtends("git@github.com:me/personal-dotfiles.git@v1.2.0")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "git@github.com:me/personal-dotfiles.git", spec.Locator)
	assert.Equal(t, "v1.2.0", spec.Ref)
}

func TestParseExtends_LocalDirectory(t *testing.T) {
	spec, err := ParseExtends("./personal-dotfiles")
	require.NoError(t, err)
	assert.Equal(t, ExtendsDir, spec.Kind)
	assert.Equal(t, "./personal-dotfiles", spec.Locator)
}

func TestParseExtends_LocalFile(t *testing.T) {
	spec, err := ParseExtends("profiles/shared/base.yaml")
	require.NoError(t, err)
	assert.Equal(t, ExtendsFile, spec.Kind)
	assert.Equal(t, "profiles/shared/base.yaml", spec.Locator)
}

func TestParseExtends_RejectsEmpty(t *testing.T) {
	_, err := ParseExtends("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extends")
}
```

- [ ] **Step 2: Run the parser tests and confirm they fail because the parser does not exist yet**

Run: `go test ./internal/profile -run 'TestParseExtends_' -v`

Expected: FAIL with compile errors such as `undefined: ParseExtends` and `undefined: ExtendsGit`.

- [ ] **Step 3: Replace the old validation tests with the new accepted/rejected values**

Update `internal/profile/loader_test.go` so validation reflects the locator contract:

```go
func TestValidateProfile_ValidGitExtends(t *testing.T) {
	cfg := &FacetConfig{Extends: "git@github.com:me/personal-dotfiles.git@main"}
	assert.NoError(t, ValidateProfile(cfg))
}

func TestValidateProfile_ValidLocalFileExtends(t *testing.T) {
	cfg := &FacetConfig{Extends: "shared/base.yaml"}
	assert.NoError(t, ValidateProfile(cfg))
}

func TestValidateProfile_InvalidExtends(t *testing.T) {
	cfg := &FacetConfig{Extends: "@not-a-locator"}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extends")
}
```

- [ ] **Step 4: Run the validation tests and confirm the current `extends: base` logic fails them**

Run: `go test ./internal/profile -run 'TestValidateProfile_' -v`

Expected: FAIL because `ValidateProfile` still hard-codes `base`.

- [ ] **Step 5: Leave the workspace red and move to implementation**

No commit yet. Repository policy forbids committing before docs and E2E updates are part of the slice.

---

### Task 2: Add Provenance Metadata And Base Resolution

**Files:**
- Create: `internal/profile/extends.go`
- Create: `internal/profile/base_resolver.go`
- Create: `internal/profile/base_resolver_test.go`
- Modify: `internal/profile/types.go`
- Modify: `internal/profile/loader.go`
- Modify: `internal/profile/merger.go`
- Modify: `internal/profile/merger_test.go`
- Modify: `internal/profile/resolver.go`
- Modify: `internal/profile/resolver_test.go`

- [ ] **Step 1: Add the parser and internal provenance types**

Implement the parser in `internal/profile/extends.go` and add internal-only metadata in `internal/profile/types.go`:

```go
package profile

type ExtendsKind string

const (
	ExtendsGit  ExtendsKind = "git"
	ExtendsDir  ExtendsKind = "dir"
	ExtendsFile ExtendsKind = "file"
)

type ExtendsSpec struct {
	Raw     string
	Kind    ExtendsKind
	Locator string
	Ref     string
}

type ConfigProvenance struct {
	SourceRoot  string `yaml:"-"`
	Materialize bool   `yaml:"-"`
}

type FacetConfig struct {
	Extends          string                      `yaml:"extends,omitempty"`
	Vars             map[string]any             `yaml:"vars,omitempty"`
	Packages         []PackageEntry             `yaml:"packages,omitempty"`
	Configs          map[string]string          `yaml:"configs,omitempty"`
	ConfigMeta       map[string]ConfigProvenance `yaml:"-"`
	AI               *AIConfig                  `yaml:"ai,omitempty"`
	PreApply         []ScriptEntry              `yaml:"pre_apply,omitempty"`
	PostApply        []ScriptEntry              `yaml:"post_apply,omitempty"`
}

type ScriptEntry struct {
	Name    string `yaml:"name"`
	Run     string `yaml:"run"`
	WorkDir string `yaml:"-"`
}
```

- [ ] **Step 2: Add layer annotation helpers and update merge/resolve behavior**

Teach the profile package to carry provenance forward:

```go
func AnnotateLayer(cfg *FacetConfig, sourceRoot string, materialize bool) {
	if cfg == nil {
		return
	}
	if cfg.ConfigMeta == nil {
		cfg.ConfigMeta = make(map[string]ConfigProvenance, len(cfg.Configs))
	}
	for target := range cfg.Configs {
		cfg.ConfigMeta[target] = ConfigProvenance{
			SourceRoot:  sourceRoot,
			Materialize: materialize,
		}
	}
	for i := range cfg.PreApply {
		cfg.PreApply[i].WorkDir = sourceRoot
	}
	for i := range cfg.PostApply {
		cfg.PostApply[i].WorkDir = sourceRoot
	}
}
```

In `merger.go`, merge `ConfigMeta` by target using the same overlay-wins rule as `Configs`, and preserve `WorkDir` when copying scripts. In `resolver.go`, copy `ConfigMeta` straight through and preserve `WorkDir` when rebuilding script entries.

- [ ] **Step 3: Implement the base resolver with fresh clone + cleanup**

Create `internal/profile/base_resolver.go`:

```go
type CommandRunner interface {
	Run(name string, args ...string) error
}

type ResolvedBase struct {
	Config  *FacetConfig
	Cleanup func() error
}

type BaseResolver struct {
	loader *Loader
	runner CommandRunner
}

func NewBaseResolver(loader *Loader, runner CommandRunner) *BaseResolver {
	return &BaseResolver{loader: loader, runner: runner}
}

func (r *BaseResolver) Resolve(rawExtends, localConfigDir string) (*ResolvedBase, error) {
	spec, err := ParseExtends(rawExtends)
	if err != nil {
		return nil, err
	}

	switch spec.Kind {
	case ExtendsFile:
		path := resolveLocalPath(localConfigDir, spec.Locator)
		cfg, err := r.loader.LoadConfig(path)
		if err != nil {
			return nil, err
		}
		AnnotateLayer(cfg, filepath.Dir(path), false)
		return &ResolvedBase{Config: cfg, Cleanup: func() error { return nil }}, nil
	case ExtendsDir:
		root := resolveLocalPath(localConfigDir, spec.Locator)
		cfg, err := r.loader.LoadConfig(filepath.Join(root, "base.yaml"))
		if err != nil {
			return nil, err
		}
		AnnotateLayer(cfg, root, false)
		return &ResolvedBase{Config: cfg, Cleanup: func() error { return nil }}, nil
	case ExtendsGit:
		tmpDir, err := os.MkdirTemp("", "facet-extends-*")
		if err != nil {
			return nil, err
		}
		cleanup := func() error { return os.RemoveAll(tmpDir) }
		if err := r.runner.Run("git", "clone", "--depth", "1", spec.Locator, tmpDir); err != nil {
			_ = cleanup()
			return nil, err
		}
		if spec.Ref != "" {
			if err := r.runner.Run("git", "-C", tmpDir, "checkout", spec.Ref); err != nil {
				_ = cleanup()
				return nil, err
			}
		}
		cfg, err := r.loader.LoadConfig(filepath.Join(tmpDir, "base.yaml"))
		if err != nil {
			_ = cleanup()
			return nil, err
		}
		AnnotateLayer(cfg, tmpDir, true)
		return &ResolvedBase{Config: cfg, Cleanup: cleanup}, nil
	default:
		return nil, fmt.Errorf("unsupported extends kind %q", spec.Kind)
	}
}
```

- [ ] **Step 4: Add hermetic local-git tests for branch/tag/commit/default-branch and cleanup**

Create `internal/profile/base_resolver_test.go` with real local git repos:

```go
func TestBaseResolver_GitDefaultBranch(t *testing.T) {
	repo := newGitRepoWithBase(t, "main", "vars:\n  source: remote-main\n")
	resolver := NewBaseResolver(NewLoader(), execrunner.New())

	result, err := resolver.Resolve(repo, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, result.Cleanup()) })

	assert.Equal(t, "remote-main", result.Config.Vars["source"])
	assert.True(t, result.Config.ConfigMeta["~/.gitconfig"].Materialize)
}

func TestBaseResolver_GitTag(t *testing.T) {
	repo, tagName := newTaggedGitRepoWithBase(t)
	resolver := NewBaseResolver(NewLoader(), execrunner.New())

	result, err := resolver.Resolve(repo+"@"+tagName, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, result.Cleanup()) })
	assert.Equal(t, "tagged", result.Config.Vars["source"])
}

func TestBaseResolver_CleanupRemovesClone(t *testing.T) {
	repo := newGitRepoWithBase(t, "main", "vars:\n  source: remote-main\n")
	resolver := NewBaseResolver(NewLoader(), execrunner.New())

	result, err := resolver.Resolve(repo, t.TempDir())
	require.NoError(t, err)

	root := result.Config.ConfigMeta["~/.gitconfig"].SourceRoot
	require.DirExists(t, root)
	require.NoError(t, result.Cleanup())
	assert.NoDirExists(t, root)
}
```

- [ ] **Step 5: Run the focused profile test suites until they pass**

Run:

```bash
go test ./internal/profile -run 'Test(ParseExtends_|ValidateProfile_|BaseResolver_|Merge_|Resolve_)' -v
```

Expected: PASS.

---

### Task 3: Integrate Base Resolution Into `facet apply`

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/apply.go`
- Modify: `internal/app/report.go`
- Modify: `internal/app/apply_test.go`
- Modify: `main.go`

- [ ] **Step 1: Add the base-resolver dependency to the app boundary**

Update `internal/app/interfaces.go` and `internal/app/app.go`:

```go
type BaseResolver interface {
	Resolve(rawExtends string, localConfigDir string) (*profile.ResolvedBase, error)
}

type Deps struct {
	Loader          ProfileLoader
	BaseResolver    BaseResolver
	Installer       Installer
	Reporter        Reporter
	StateStore      StateStore
	DeployerFactory DeployerFactory
	AIOrchestrator  AIOrchestrator
	ScriptRunner    ScriptRunner
	SkillsManager   SkillsManager
	Version         string
	OSName          string
}
```

Store `baseResolver` on `App` and wire it in `main.go` with:

```go
baseResolver := profile.NewBaseResolver(loader, execrunner.New())
application := app.New(app.Deps{
	Loader:       loader,
	BaseResolver: baseResolver,
	// ...
})
```

- [ ] **Step 2: Resolve the base before merge and always defer cleanup**

Update `internal/app/apply.go` so the profile drives base resolution:

```go
profileCfg, err := a.loader.LoadConfig(profilePath)
if err != nil {
	return fmt.Errorf("cannot load profile %q: %w", profileName, err)
}
if err := profile.ValidateProfile(profileCfg); err != nil {
	return err
}
profile.AnnotateLayer(profileCfg, opts.ConfigDir, false)

resolvedBase, err := a.baseResolver.Resolve(profileCfg.Extends, opts.ConfigDir)
if err != nil {
	return fmt.Errorf("cannot resolve extends %q: %w", profileCfg.Extends, err)
}
defer func() {
	if cleanupErr := resolvedBase.Cleanup(); cleanupErr != nil {
		a.reporter.Warning(fmt.Sprintf("Failed to clean extends clone: %v", cleanupErr))
	}
}()

baseCfg := resolvedBase.Config
localCfg, err := a.loader.LoadConfig(localPath)
if err != nil {
	return fmt.Errorf(".local.yaml is required in %s: %w", opts.StateDir, err)
}
profile.AnnotateLayer(localCfg, opts.ConfigDir, false)
```

- [ ] **Step 3: Run scripts from their annotated working directories**

Update `runScripts`:

```go
func (a *App) runScripts(scripts []profile.ScriptEntry, fallbackDir, stageName string) error {
	if len(scripts) == 0 {
		return nil
	}
	if a.scriptRunner == nil {
		return fmt.Errorf("%s script runner is not configured", stageName)
	}
	for _, script := range scripts {
		dir := script.WorkDir
		if dir == "" {
			dir = fallbackDir
		}
		if err := a.scriptRunner.Run(script.Run, dir); err != nil {
			return fmt.Errorf("%s script %q failed: %w", stageName, script.Name, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Add app tests for merge order, cleanup, and script directories**

Extend `internal/app/apply_test.go` with tests like:

```go
func TestApply_UsesResolvedBaseBeforeProfileAndLocal(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte("vars:\n  source: local\n"), 0o644))

	resolver := &mockBaseResolver{
		result: &profile.ResolvedBase{
			Config: &profile.FacetConfig{
				Vars: map[string]any{"source": "remote"},
			},
			Cleanup: func() error { return nil },
		},
	}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "git@github.com:me/personal-dotfiles.git@main",
				Vars:    map[string]any{"source": "profile"},
			},
			filepath.Join(stateDir, ".local.yaml"): {
				Vars: map[string]any{"source": "local"},
			},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		BaseResolver: resolver,
		Installer:    &mockInstaller{},
		StateStore:   &mockStateStore{},
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, owned []deploy.ConfigResult) deploy.Service {
			return &mockDeployer{}
		},
		OSName: "macos",
	})

	require.NoError(t, a.Apply("work", ApplyOpts{ConfigDir: cfgDir, StateDir: stateDir}))
	assert.True(t, resolver.cleanedUp)
}

func TestApply_RunsRemoteScriptsFromRemoteClone(t *testing.T) {
	// remote base pre/post scripts should use annotated WorkDir, profile scripts should use cfgDir
}
```

- [ ] **Step 5: Run the app-level tests for the new apply flow**

Run:

```bash
go test ./internal/app -run 'TestApply_(UsesResolvedBase|RunsRemoteScripts|DryRun|StagesFiltering)' -v
```

Expected: PASS.

---

### Task 4: Support Remote-Only Materialized Configs And Correct Status

**Files:**
- Modify: `internal/deploy/pathexpand.go`
- Modify: `internal/deploy/pathexpand_test.go`
- Modify: `internal/deploy/deployer.go`
- Modify: `internal/deploy/deployer_test.go`
- Modify: `internal/app/apply.go`
- Modify: `internal/app/report.go`
- Modify: `internal/app/status.go`
- Modify: `internal/app/status_test.go`

- [ ] **Step 1: Add provenance-aware source resolution helpers**

Replace the relative-only assumption in `internal/deploy/pathexpand.go` with helpers that can resolve either local relative sources or remote-resolved absolute sources:

```go
type SourceSpec struct {
	DisplaySource string
	ResolvedPath  string
	Materialize   bool
}

func ResolveSourcePath(source string, meta profile.ConfigProvenance, localConfigDir string) (SourceSpec, error) {
	root := meta.SourceRoot
	if root == "" {
		root = localConfigDir
	}

	if filepath.IsAbs(source) {
		return SourceSpec{
			DisplaySource: source,
			ResolvedPath:  source,
			Materialize:   meta.Materialize,
		}, nil
	}

	resolved := filepath.Join(root, source)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return SourceSpec{}, fmt.Errorf("cannot resolve source path %q: %w", source, err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return SourceSpec{}, fmt.Errorf("cannot resolve source root: %w", err)
	}

	if !strings.HasPrefix(absResolved, absRoot+string(filepath.Separator)) && absResolved != absRoot {
		return SourceSpec{}, fmt.Errorf("config source path %q escapes source root %s", source, absRoot)
	}

	return SourceSpec{
		DisplaySource: source,
		ResolvedPath:  absResolved,
		Materialize:   meta.Materialize,
	}, nil
}
```

- [ ] **Step 2: Teach the deployer a remote-only copy strategy**

Update `internal/deploy/deployer.go` so remote-base configs are always materialized:

```go
const (
	StrategySymlink  = "symlink"
	StrategyTemplate = "template"
	StrategyCopy     = "copy"
)

func DetectStrategy(sourcePath string, materialize bool) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}

	if info.IsDir() {
		if materialize {
			return StrategyCopy, nil
		}
		return StrategySymlink, nil
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot read source %q: %w", sourcePath, err)
	}

	if strings.Contains(string(content), "${facet:") {
		return StrategyTemplate, nil
	}
	if materialize {
		return StrategyCopy, nil
	}
	return StrategySymlink, nil
}
```

Adjust `DeployOne` to accept the resolved source path instead of blindly joining `configDir`, and implement recursive directory copy for `StrategyCopy`.

- [ ] **Step 3: Update apply/report to resolve sources using provenance metadata**

In `internal/app/apply.go` and `internal/app/report.go`, resolve each config entry like this:

```go
meta := resolved.ConfigMeta[target]
sourceSpec, err := deploy.ResolveSourcePath(source, meta, opts.ConfigDir)
if err != nil {
	// existing error/skip handling
}

_, err = deployer.DeployOne(expandedTarget, sourceSpec, opts.Force)
```

Dry-run should print the original display source plus the chosen strategy. Remote-base entries should show `copy` or `template`, never `symlink`.

- [ ] **Step 4: Update status checks so local symlinks still validate and remote materialized files do not expect a live clone**

Change `internal/app/status.go` and `internal/app/status_test.go` so:

```go
switch cfg.Strategy {
case deploy.StrategySymlink:
	expectedSource := cfg.SourcePath
	if expectedSource == "" && cfgDir != "" {
		expectedSource = filepath.Join(cfgDir, cfg.Source)
	}
	// compare symlink target to expectedSource
case deploy.StrategyTemplate, deploy.StrategyCopy:
	check.Valid = true
}
```

Add tests for:

- valid local symlink still passes
- copied remote config does not require a source clone to exist
- template output still reports valid

- [ ] **Step 5: Run deploy/status/app regression tests**

Run:

```bash
go test ./internal/deploy ./internal/app -run 'Test(DetectStrategy_|Deploy_|RunValidityChecks_|Status_)' -v
```

Expected: PASS.

---

### Task 5: Add Full Hermetic E2E Coverage For Local And Git Extends

**Files:**
- Create: `e2e/suites/15-remote-extends.sh`
- Modify: `e2e/suites/helpers.sh` (only if a helper reduces duplication cleanly)

- [ ] **Step 1: Write the hermetic E2E suite using only local filesystem git repos**

Create `e2e/suites/15-remote-extends.sh`:

```bash
#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

mkdir -p "$HOME/dotfiles/profiles" "$HOME/dotfiles/configs/local" "$HOME/.facet"
cat > "$HOME/.facet/.local.yaml" <<'YAML'
vars:
  git_email: user@example.com
YAML

REMOTE_REPO="$HOME/remote-base"
mkdir -p "$REMOTE_REPO/configs/remote"
git init -b main "$REMOTE_REPO" >/dev/null 2>&1
cat > "$REMOTE_REPO/base.yaml" <<'YAML'
vars:
  remote_name: remote-base
configs:
  ~/.remote-base-config: configs/remote/base.conf
pre_apply:
  - name: remote-pre
    run: printf '%s' "$PWD" > "$HOME/remote-pre-dir.txt"
YAML
cat > "$REMOTE_REPO/configs/remote/base.conf" <<'EOF'
remote-file
EOF
(cd "$REMOTE_REPO" && git add . && git commit -m "base" >/dev/null 2>&1 && git tag v1.0.0)

cat > "$HOME/dotfiles/profiles/work.yaml" <<YAML
extends: $REMOTE_REPO@main
configs:
  ~/.local-override: configs/local/override.conf
post_apply:
  - name: local-post
    run: printf '%s' "$PWD" > "$HOME/local-post-dir.txt"
YAML
cat > "$HOME/dotfiles/configs/local/override.conf" <<'EOF'
local-file
EOF

facet_apply work
assert_file_exists "$HOME/.remote-base-config"
assert_not_symlink "$HOME/.remote-base-config"
assert_file_contains "$HOME/.remote-base-config" "remote-file"
assert_symlink "$HOME/.local-override"
assert_file_contains "$HOME/remote-pre-dir.txt" "/facet-extends-"
assert_file_contains "$HOME/local-post-dir.txt" "$HOME/dotfiles"
echo "  remote git base and local profile sources resolve correctly"
```

- [ ] **Step 2: Extend the suite to cover tag, commit, default branch, local dir, local file, invalid ref, and missing remote `base.yaml`**

Add independent cases in the same suite:

```bash
# default branch with no @ref
cat > "$HOME/dotfiles/profiles/default-branch.yaml" <<YAML
extends: $REMOTE_REPO
YAML
facet_apply default-branch

# tag
cat > "$HOME/dotfiles/profiles/tagged.yaml" <<YAML
extends: $REMOTE_REPO@v1.0.0
YAML
facet_apply tagged

# commit
REMOTE_SHA=$(cd "$REMOTE_REPO" && git rev-parse HEAD)
cat > "$HOME/dotfiles/profiles/commit.yaml" <<YAML
extends: $REMOTE_REPO@$REMOTE_SHA
YAML
facet_apply commit

# local directory
cat > "$HOME/dotfiles/profiles/local-dir.yaml" <<YAML
extends: $REMOTE_REPO
YAML

# local file
cat > "$HOME/dotfiles/profiles/local-file.yaml" <<YAML
extends: $REMOTE_REPO/base.yaml
YAML

# invalid ref
cat > "$HOME/dotfiles/profiles/bad-ref.yaml" <<YAML
extends: $REMOTE_REPO@does-not-exist
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply bad-ref

# missing base
BROKEN_REPO="$HOME/broken-base"
git init -b main "$BROKEN_REPO" >/dev/null 2>&1
(cd "$BROKEN_REPO" && git commit --allow-empty -m "empty" >/dev/null 2>&1)
cat > "$HOME/dotfiles/profiles/no-base.yaml" <<YAML
extends: $BROKEN_REPO
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply no-base
```

- [ ] **Step 3: Run the new suite and make it pass natively**

Run: `bash e2e/harness.sh e2e/suites/15-remote-extends.sh`

Expected: PASS.

- [ ] **Step 4: Confirm the suite stays hermetic under Docker-backed Linux E2E**

Run: `go test ./e2e -tags e2e -run TestE2E_Docker -v`

Expected: PASS, with no network dependency because the suite only uses local git repositories.

- [ ] **Step 5: Keep the feature uncommitted until docs are updated**

No commit yet. The repository completeness rule requires docs + E2E before commit.

---

### Task 6: Update All Required User-Facing Docs

**Files:**
- Modify: `README.md`
- Modify: `internal/docs/topics/config.md`
- Modify: `internal/docs/topics/merge.md`
- Modify: `internal/docs/topics/quickstart.md`
- Modify: `internal/docs/topics/commands.md`
- Modify: `internal/docs/topics/examples.md`
- Modify: `internal/docs/topics/variables.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Search every required doc surface for the old `extends: base` assumption**

Run:

```bash
rg -n "extends: base|must be `base`|must be \"base\"|Three layers are merged|Load base.yaml|shared foundation" \
  README.md internal/docs/topics docs/architecture/v1-design-spec.md
```

Expected: a list of stale references in all three required doc surfaces.

- [ ] **Step 2: Update the README to describe the shipped user contract**

Edit `README.md` so it includes:

```md
Profiles now point to a base locator via `extends`.

Supported forms:

```yaml
extends: https://github.com/me/personal-dotfiles.git
extends: https://github.com/me/personal-dotfiles.git@main
extends: git@github.com:me/personal-dotfiles.git@v1.2.0
extends: /Users/me/personal-dotfiles
extends: /Users/me/personal-dotfiles/base.yaml
```

Merge order is:

1. resolved base from `extends`
2. selected local profile
3. `.local.yaml`

When the base comes from git, facet clones it fresh for each apply and cleans it up afterward. Config files inherited from that remote base are materialized into place rather than symlinked.
```

- [ ] **Step 3: Update the embedded docs in `internal/docs/topics/`**

Make the following concrete updates:

- `config.md`
  Replace “must be `base`” with the locator contract and examples.
- `merge.md`
  Change the layer-order wording from literal `base.yaml` to “resolved base from `extends`”.
- `quickstart.md`
  Keep the local `base.yaml` example, but mention git/local-path `extends` as an alternative.
- `commands.md`
  Update `apply` to say it resolves external bases before merge.
- `examples.md`
  Add one example profile that extends a git base and note that remote-base configs are materialized.
- `variables.md`
  Clarify that config source values are substituted first, then resolved relative to the owning source root.

- [ ] **Step 4: Update the authoritative design spec**

Edit `docs/architecture/v1-design-spec.md` to match the shipped behavior:

```md
- `extends` is a locator string, not only `base`
- supported forms: HTTPS git, SSH git, local directory, local base file
- optional `@ref` syntax for branch/tag/commit
- omitted ref uses remote default branch
- merge order is resolved base -> local profile -> .local.yaml
- remote-base configs are materialized, not symlinked
- git-based extends are cloned fresh per apply and cleaned up afterward
```

- [ ] **Step 5: Re-run the search to verify stale wording is gone**

Run:

```bash
rg -n "extends: base|must be `base`|must be \"base\"|only value allowed in v1" \
  README.md internal/docs/topics docs/architecture/v1-design-spec.md
```

Expected: no matches, or only intentional historical examples that have been rewritten explicitly.

---

### Task 7: Full Verification And Final Commit

**Files:**
- Modify: all staged feature files from Tasks 1-6

- [ ] **Step 1: Run the targeted Go tests one last time**

Run:

```bash
go test ./internal/profile ./internal/deploy ./internal/app -v
```

Expected: PASS.

- [ ] **Step 2: Run the dedicated E2E suite one last time**

Run: `bash e2e/harness.sh e2e/suites/15-remote-extends.sh`

Expected: PASS.

- [ ] **Step 3: Run the repository-required full verification**

Run: `make pre-commit`

Expected: PASS, including unit tests, native E2E, and Docker Linux E2E.

- [ ] **Step 4: Create a single complete feature commit after docs and E2E are included**

```bash
git add \
  internal/profile/extends.go \
  internal/profile/extends_test.go \
  internal/profile/base_resolver.go \
  internal/profile/base_resolver_test.go \
  internal/profile/types.go \
  internal/profile/loader.go \
  internal/profile/loader_test.go \
  internal/profile/merger.go \
  internal/profile/merger_test.go \
  internal/profile/resolver.go \
  internal/profile/resolver_test.go \
  internal/app/interfaces.go \
  internal/app/app.go \
  internal/app/apply.go \
  internal/app/report.go \
  internal/app/apply_test.go \
  internal/app/status.go \
  internal/app/status_test.go \
  internal/deploy/pathexpand.go \
  internal/deploy/pathexpand_test.go \
  internal/deploy/deployer.go \
  internal/deploy/deployer_test.go \
  main.go \
  e2e/suites/15-remote-extends.sh \
  README.md \
  internal/docs/topics/config.md \
  internal/docs/topics/merge.md \
  internal/docs/topics/quickstart.md \
  internal/docs/topics/commands.md \
  internal/docs/topics/examples.md \
  internal/docs/topics/variables.md \
  docs/architecture/v1-design-spec.md
git commit -m "feat: support remote profile extends"
```

- [ ] **Step 5: Capture the final implementation notes in the work summary**

Include:

- accepted `extends` forms
- remote-only materialization rule
- source-root/script-working-dir handling
- hermetic git E2E coverage
- docs updated across all required surfaces

---

## Self-Review Checklist

- Spec coverage:
  - supported locator formats: Task 1, Task 2, Task 6
  - fresh clone + cleanup: Task 2, Task 3, Task 5
  - remote relative source handling: Task 2, Task 3, Task 4, Task 5
  - remote-only materialized configs: Task 4, Task 5, Task 6
  - hermetic no-network git testing: Task 5
  - required docs updates: Task 6
- Placeholder scan:
  - no `TODO`, `TBD`, or “implement later” placeholders remain
- Type consistency:
  - parser type: `ExtendsSpec`
  - provenance type: `ConfigProvenance`
  - resolved base type: `ResolvedBase`
  - deploy resolution helper: `ResolveSourcePath`
