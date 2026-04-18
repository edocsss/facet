# Pre/Post Apply Scripts & Stages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the standalone `facet init` command, replace the `init` YAML field with `pre_apply` and `post_apply`, and add a `--stages` flag to `facet apply` for selective stage execution.

**Architecture:** The `init` field and command are replaced by two script fields (`pre_apply`, `post_apply`) that run automatically during `facet apply`. A `--stages` flag gates which of five stages run: `configs`, `pre_apply`, `packages`, `post_apply`, `ai`. Types are renamed (`InitScript` → `ScriptEntry`), and all merge/resolve/test/doc references are updated.

**Tech Stack:** Go, Cobra CLI, YAML (gopkg.in/yaml.v3), testify

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/profile/types.go` | Rename `InitScript` → `ScriptEntry`, rename `Init` → `PreApply`+`PostApply` on `FacetConfig` |
| Modify | `internal/profile/merger.go` | Replace `mergeInit` with `mergeScripts`, call for both fields |
| Modify | `internal/profile/merger_test.go` | Update all Init tests → PreApply/PostApply tests |
| Modify | `internal/profile/resolver.go` | Resolve both `PreApply` and `PostApply` fields |
| Modify | `internal/profile/resolver_test.go` | Update Init tests → PreApply/PostApply tests |
| Modify | `internal/app/apply.go` | Add `Stages` to `ApplyOpts`, add stages parsing/validation, add pre_apply/post_apply execution, gate all 5 stages |
| Modify | `internal/app/apply_test.go` | Add tests for stages filtering, pre/post apply script execution |
| Modify | `internal/app/report.go` | Remove `printInitReport`, add script results to `printApplyReport` |
| Modify | `internal/app/state.go` | No changes needed (scripts are stateless) |
| Modify | `internal/app/interfaces.go` | Update `ScriptRunner` doc comment |
| Modify | `internal/app/app.go` | No struct changes needed |
| Delete | `internal/app/init.go` | Remove init workflow |
| Delete | `internal/app/init_test.go` | Remove init tests |
| Modify | `cmd/apply.go` | Add `--stages` flag |
| Delete | `cmd/init_cmd.go` | Remove init command |
| Modify | `cmd/root.go` | Remove init command registration |
| Modify | `main.go` | No changes needed (shellScriptRunner stays) |
| Modify | `internal/docs/docs.go` | Replace `init` topic with `scripts` |
| Delete | `internal/docs/topics/init.md` | Remove init topic |
| Create | `internal/docs/topics/scripts.md` | New topic for pre_apply/post_apply |
| Modify | `internal/docs/topics/commands.md` | Remove init command, add `--stages` flag to apply |
| Modify | `internal/docs/topics/config.md` | Replace `init` field with `pre_apply` and `post_apply` |
| Modify | `internal/docs/topics/examples.md` | Update init examples to pre/post_apply |
| Modify | `e2e/suites/11-init-scripts.sh` | Rewrite as pre/post apply + stages tests |
| Modify | `e2e/fixtures/setup-basic.sh` | Replace `init:` with `pre_apply:` and `post_apply:` |
| Modify | `e2e/suites/helpers.sh` | Remove `facet_init` helper |

---

### Task 1: Rename `InitScript` → `ScriptEntry` and update fields in types

**Files:**
- Modify: `internal/profile/types.go:21-28`

- [ ] **Step 1: Update types.go**

Replace the `InitScript` type and the `Init` field on `FacetConfig`:

```go
// ScriptEntry is a named shell command run during facet apply.
type ScriptEntry struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}
```

On `FacetConfig`, replace:
```go
Init []InitScript `yaml:"init,omitempty"`
```
with:
```go
PreApply  []ScriptEntry `yaml:"pre_apply,omitempty"`
PostApply []ScriptEntry `yaml:"post_apply,omitempty"`
```

- [ ] **Step 2: Run build to find all compile errors**

Run: `go build ./...`
Expected: compile errors in merger.go, resolver.go, app/init.go, app/apply.go, tests — this is expected, we fix them in subsequent tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/profile/types.go
git commit -m "refactor: rename InitScript to ScriptEntry, add pre_apply/post_apply fields"
```

---

### Task 2: Update merger for pre_apply and post_apply

**Files:**
- Modify: `internal/profile/merger.go:122-138`
- Modify: `internal/profile/merger.go:28-29`

- [ ] **Step 1: Update merger.go**

Replace the `mergeInit` function with a generic `mergeScripts`:

```go
// mergeScripts concatenates base scripts followed by overlay scripts.
// No deduplication — if both layers define a script with the same name, both run.
func mergeScripts(base, overlay []ScriptEntry) []ScriptEntry {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	result := make([]ScriptEntry, 0, len(base)+len(overlay))
	for _, script := range base {
		result = append(result, ScriptEntry{Name: script.Name, Run: script.Run})
	}
	for _, script := range overlay {
		result = append(result, ScriptEntry{Name: script.Name, Run: script.Run})
	}

	return result
}
```

In the `Merge` function, replace:
```go
// Merge init scripts (concatenation — base first, then overlay)
result.Init = mergeInit(base.Init, overlay.Init)
```
with:
```go
// Merge pre_apply scripts (concatenation — base first, then overlay)
result.PreApply = mergeScripts(base.PreApply, overlay.PreApply)

// Merge post_apply scripts (concatenation — base first, then overlay)
result.PostApply = mergeScripts(base.PostApply, overlay.PostApply)
```

- [ ] **Step 2: Update merger_test.go**

Replace all `TestMerge_Init*` tests. Here is the complete replacement for the init test block (lines 187-265):

```go
func TestMerge_PreApplyConcatenation(t *testing.T) {
	base := &FacetConfig{
		PreApply: []ScriptEntry{
			{Name: "base-script-1", Run: "echo base1"},
			{Name: "base-script-2", Run: "echo base2"},
		},
	}
	overlay := &FacetConfig{
		PreApply: []ScriptEntry{
			{Name: "overlay-script-1", Run: "echo overlay1"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.PreApply, 3)
	assert.Equal(t, "base-script-1", result.PreApply[0].Name)
	assert.Equal(t, "echo base1", result.PreApply[0].Run)
	assert.Equal(t, "base-script-2", result.PreApply[1].Name)
	assert.Equal(t, "echo base2", result.PreApply[1].Run)
	assert.Equal(t, "overlay-script-1", result.PreApply[2].Name)
	assert.Equal(t, "echo overlay1", result.PreApply[2].Run)
}

func TestMerge_PostApplyConcatenation(t *testing.T) {
	base := &FacetConfig{
		PostApply: []ScriptEntry{
			{Name: "base-script", Run: "echo base"},
		},
	}
	overlay := &FacetConfig{
		PostApply: []ScriptEntry{
			{Name: "overlay-script", Run: "echo overlay"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.PostApply, 2)
	assert.Equal(t, "base-script", result.PostApply[0].Name)
	assert.Equal(t, "overlay-script", result.PostApply[1].Name)
}

func TestMerge_PreApplyBaseOnly(t *testing.T) {
	base := &FacetConfig{
		PreApply: []ScriptEntry{
			{Name: "base-only", Run: "echo base"},
		},
	}
	overlay := &FacetConfig{}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.PreApply, 1)
	assert.Equal(t, "base-only", result.PreApply[0].Name)
}

func TestMerge_PostApplyOverlayOnly(t *testing.T) {
	base := &FacetConfig{}
	overlay := &FacetConfig{
		PostApply: []ScriptEntry{
			{Name: "overlay-only", Run: "echo overlay"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	require.Len(t, result.PostApply, 1)
	assert.Equal(t, "overlay-only", result.PostApply[0].Name)
}

func TestMerge_ScriptsBothEmpty(t *testing.T) {
	base := &FacetConfig{}
	overlay := &FacetConfig{}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	assert.Nil(t, result.PreApply)
	assert.Nil(t, result.PostApply)
}

func TestMerge_ScriptsDeepCopy(t *testing.T) {
	base := &FacetConfig{
		PreApply: []ScriptEntry{
			{Name: "base-script", Run: "echo base"},
		},
	}
	overlay := &FacetConfig{
		PreApply: []ScriptEntry{
			{Name: "overlay-script", Run: "echo overlay"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)

	result.PreApply[0].Run = "mutated"
	assert.Equal(t, "echo base", base.PreApply[0].Run)
	assert.Equal(t, "echo overlay", overlay.PreApply[0].Run)
}
```

- [ ] **Step 3: Run merger tests**

Run: `go test ./internal/profile/ -run TestMerge -v`
Expected: all merger tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/profile/merger.go internal/profile/merger_test.go
git commit -m "refactor: update merger for pre_apply/post_apply scripts"
```

---

### Task 3: Update resolver for pre_apply and post_apply

**Files:**
- Modify: `internal/profile/resolver.go:44-56`
- Modify: `internal/profile/resolver_test.go:272-330`

- [ ] **Step 1: Update resolver.go**

Replace the `cfg.Init` resolution block (lines 44-56) with:

```go
	if cfg.PreApply != nil {
		result.PreApply = make([]ScriptEntry, len(cfg.PreApply))
		for i, script := range cfg.PreApply {
			resolvedRun, err := substituteVars(script.Run, cfg.Vars)
			if err != nil {
				return nil, fmt.Errorf("pre_apply[%d] %q: %w", i, script.Name, err)
			}
			result.PreApply[i] = ScriptEntry{
				Name: script.Name,
				Run:  resolvedRun,
			}
		}
	}

	if cfg.PostApply != nil {
		result.PostApply = make([]ScriptEntry, len(cfg.PostApply))
		for i, script := range cfg.PostApply {
			resolvedRun, err := substituteVars(script.Run, cfg.Vars)
			if err != nil {
				return nil, fmt.Errorf("post_apply[%d] %q: %w", i, script.Name, err)
			}
			result.PostApply[i] = ScriptEntry{
				Name: script.Name,
				Run:  resolvedRun,
			}
		}
	}
```

- [ ] **Step 2: Update resolver_test.go**

Replace the four `TestResolve_InitScript*` tests (lines 272-330) with:

```go
func TestResolve_PreApplyScriptVars(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"email": "sarah@acme.com",
			},
		},
		PreApply: []ScriptEntry{
			{Name: "configure git", Run: `git config --global user.email "${facet:git.email}"`},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	require.Len(t, resolved.PreApply, 1)
	assert.Equal(t, "configure git", resolved.PreApply[0].Name)
	assert.Equal(t, `git config --global user.email "sarah@acme.com"`, resolved.PreApply[0].Run)
}

func TestResolve_PostApplyScriptVars(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"name": "Sarah"},
		PostApply: []ScriptEntry{
			{Name: "greet", Run: "echo ${facet:name}"},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	require.Len(t, resolved.PostApply, 1)
	assert.Equal(t, "echo Sarah", resolved.PostApply[0].Run)
}

func TestResolve_ScriptNameNotResolved(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"tool": "git"},
		PreApply: []ScriptEntry{
			{Name: "setup ${facet:tool}", Run: "echo hello"},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "setup ${facet:tool}", resolved.PreApply[0].Name)
}

func TestResolve_ScriptUndefinedVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{},
		PostApply: []ScriptEntry{
			{Name: "broken", Run: "echo ${facet:missing}"},
		},
	}

	_, err := Resolve(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestResolve_ScriptDeepCopy(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"name": "Sarah"},
		PreApply: []ScriptEntry{
			{Name: "test", Run: "echo ${facet:name}"},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)

	resolved.PreApply[0].Run = "mutated"
	assert.Equal(t, "echo ${facet:name}", cfg.PreApply[0].Run)
}
```

- [ ] **Step 3: Run resolver tests**

Run: `go test ./internal/profile/ -run TestResolve -v`
Expected: all resolver tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/profile/resolver.go internal/profile/resolver_test.go
git commit -m "refactor: update resolver for pre_apply/post_apply scripts"
```

---

### Task 4: Delete init command and init workflow

**Files:**
- Delete: `internal/app/init.go`
- Delete: `internal/app/init_test.go`
- Delete: `cmd/init_cmd.go`
- Modify: `cmd/root.go:27`
- Modify: `internal/app/interfaces.go:39` (doc comment only)
- Modify: `internal/app/report.go:97-134`

- [ ] **Step 1: Delete init files**

```bash
rm internal/app/init.go internal/app/init_test.go cmd/init_cmd.go
```

- [ ] **Step 2: Remove init command registration from root.go**

In `cmd/root.go`, remove this line:
```go
rootCmd.AddCommand(newInitCmd(application, &configDir, &stateDir))
```

- [ ] **Step 3: Update ScriptRunner doc comment in interfaces.go**

Change:
```go
// ScriptRunner executes shell commands for init scripts.
```
to:
```go
// ScriptRunner executes shell commands for pre_apply and post_apply scripts.
```

- [ ] **Step 4: Remove printInitReport from report.go**

Delete the entire `printInitReport` function (lines 97-134 of report.go).

- [ ] **Step 5: Run build**

Run: `go build ./...`
Expected: compile errors only from apply.go referencing `Init` field — fixed in next task.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove init command and init workflow"
```

---

### Task 5: Add stages to ApplyOpts and update apply.go

**Files:**
- Modify: `internal/app/apply.go`

- [ ] **Step 1: Add Stages field to ApplyOpts and stage constants**

At the top of `apply.go`, after the imports, add:

```go
// ValidStages is the ordered list of stage names that --stages accepts.
var ValidStages = []string{"configs", "pre_apply", "packages", "post_apply", "ai"}

// parseStages parses a comma-separated stages string into a set.
// An empty string means all stages. Returns an error for invalid stage names.
func parseStages(raw string) (map[string]bool, error) {
	if raw == "" {
		result := make(map[string]bool, len(ValidStages))
		for _, s := range ValidStages {
			result[s] = true
		}
		return result, nil
	}

	result := make(map[string]bool)
	valid := make(map[string]bool, len(ValidStages))
	for _, s := range ValidStages {
		valid[s] = true
	}

	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		if !valid[s] {
			return nil, fmt.Errorf("unknown stage %q; valid stages: %s", s, strings.Join(ValidStages, ", "))
		}
		result[s] = true
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid stages specified; valid stages: %s", strings.Join(ValidStages, ", "))
	}

	return result, nil
}
```

Add `Stages string` to `ApplyOpts`:

```go
type ApplyOpts struct {
	ConfigDir   string
	StateDir    string
	Force       bool
	SkipFailure bool
	DryRun      bool
	Stages      string
}
```

Add `"strings"` to the imports.

- [ ] **Step 2: Update Apply method with stages gating and script execution**

Replace the Apply method body from the stages parsing through the end. The full Apply method should be:

```go
func (a *App) Apply(profileName string, opts ApplyOpts) error {
	stages, err := parseStages(opts.Stages)
	if err != nil {
		return err
	}

	// Step 1: Load facet.yaml
	_, err = a.loader.LoadMeta(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory, or run facet scaffold to create one.\n  detail: %w", err)
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
	if err := profile.ValidateMergedConfig(merged); err != nil {
		return err
	}

	// Step 6: Resolve variables
	resolved, err := profile.Resolve(merged)
	if err != nil {
		return err
	}

	// Dry-run: preview what would happen without side effects
	if opts.DryRun {
		return a.printDryRun(profileName, resolved, opts)
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
	var prevAIState *ai.AIState
	if prevState != nil {
		prevConfigs = prevState.Configs
		prevAIState = prevState.AI
	}

	// Unapply previous state if needed (always runs regardless of stages)
	if prevState != nil {
		shouldUnapply := opts.Force || prevState.Profile != profileName
		if shouldUnapply {
			if a.aiOrchestrator != nil && prevState.AI != nil {
				if err := a.aiOrchestrator.Unapply(prevState.AI); err != nil {
					a.reporter.Error(fmt.Sprintf("AI unapply failed: %v", err))
				}
			}
			unapplyDeployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, nil)
			if err := unapplyDeployer.Unapply(prevState.Configs); err != nil {
				a.reporter.Warning(fmt.Sprintf("Unapply warning: %v", err))
			}
			prevConfigs = nil
			prevAIState = nil
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

	// Stage: configs — deploy symlinks/templates
	deployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, prevConfigs)
	if stages["configs"] {
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
				os.Remove(filepath.Join(opts.StateDir, stateFile))
			}
			return fmt.Errorf("config deployment failed (rolled back): %w", deployErr)
		}
	}

	// Stage: pre_apply — run pre_apply scripts
	if stages["pre_apply"] {
		if err := a.runScripts(resolved.PreApply, opts.ConfigDir, "pre_apply"); err != nil {
			return err
		}
	}

	// Stage: packages — install packages
	var pkgResults []packages.PackageResult
	if stages["packages"] {
		pkgResults = a.installer.InstallAll(resolved.Packages)
	}

	// Stage: post_apply — run post_apply scripts
	if stages["post_apply"] {
		if err := a.runScripts(resolved.PostApply, opts.ConfigDir, "post_apply"); err != nil {
			return err
		}
	}

	// Stage: ai — apply AI configuration
	var aiState *ai.AIState
	if stages["ai"] {
		if a.aiOrchestrator != nil {
			effectiveAI := ai.Resolve(resolved.AI)
			if effectiveAI != nil || prevAIState != nil {
				var aiErr error
				aiState, aiErr = a.aiOrchestrator.Apply(effectiveAI, prevAIState)
				if aiErr != nil {
					a.reporter.Error(fmt.Sprintf("AI configuration failed: %v", aiErr))
				}
				if isEmptyAIState(aiState) {
					aiState = nil
				}
			}
		}
	}

	// Write final state
	applyState := &ApplyState{
		Profile:      profileName,
		AppliedAt:    time.Now().UTC(),
		FacetVersion: a.version,
		Packages:     pkgResults,
		Configs:      deployer.Deployed(),
		AI:           aiState,
	}

	if err := a.stateStore.Write(opts.StateDir, applyState); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	// Report
	a.printApplyReport(applyState)

	return nil
}
```

- [ ] **Step 3: Add runScripts helper method**

Add this method to `apply.go`, after the `Apply` method:

```go
// runScripts executes a list of scripts sequentially. Fails fast on first error.
func (a *App) runScripts(scripts []profile.ScriptEntry, configDir, stageName string) error {
	if len(scripts) == 0 {
		return nil
	}
	if a.scriptRunner == nil {
		return fmt.Errorf("%s script runner is not configured", stageName)
	}

	for _, script := range scripts {
		if err := a.scriptRunner.Run(script.Run, configDir); err != nil {
			return fmt.Errorf("%s script %q failed: %w", stageName, script.Name, err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Run build**

Run: `go build ./...`
Expected: compiles successfully

- [ ] **Step 5: Commit**

```bash
git add internal/app/apply.go
git commit -m "feat: add stages gating and pre_apply/post_apply execution to apply"
```

---

### Task 6: Add --stages flag to apply command

**Files:**
- Modify: `cmd/apply.go`

- [ ] **Step 1: Add stages flag**

Add a `stages` variable and bind it:

```go
func newApplyCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	var force, skipFailure, dryRun bool
	var stages string

	cmd := &cobra.Command{
		Use:   "apply <profile>",
		Short: "Apply a configuration profile",
		Long: `Loads, merges, and applies a configuration profile to this machine.

Stages run in this order: configs, pre_apply, packages, post_apply, ai.
Use --stages to run only specific stages (comma-separated).`,
		Args: cobra.ExactArgs(1),
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
				DryRun:      dryRun,
				Stages:      stages,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Unapply + apply, skip prompts for conflicting files")
	cmd.Flags().BoolVar(&skipFailure, "skip-failure", false, "Warn on config deploy failure instead of rolling back")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would happen without making changes")
	cmd.Flags().StringVar(&stages, "stages", "", "Comma-separated list of stages to run (default: all). Valid: configs, pre_apply, packages, post_apply, ai")

	return cmd
}
```

- [ ] **Step 2: Run build**

Run: `go build ./...`
Expected: compiles successfully

- [ ] **Step 3: Commit**

```bash
git add cmd/apply.go
git commit -m "feat: add --stages flag to apply command"
```

---

### Task 7: Update apply tests

**Files:**
- Modify: `internal/app/apply_test.go`

- [ ] **Step 1: Add test for pre_apply and post_apply scripts running in order**

Add to `apply_test.go`:

```go
func TestApply_RunsPreAndPostApplyScripts(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	mockDep := &mockDeployer{}
	stateStore := &mockStateStore{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "base-pre", Run: "echo base-pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "base-post", Run: "echo base-post"},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				PreApply: []profile.ScriptEntry{
					{Name: "profile-pre", Run: "echo profile-pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "profile-post", Run: "echo profile-post"},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &mockInstaller{},
		StateStore:   stateStore,
		ScriptRunner: runner,
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

	// Pre-apply scripts (base then profile) run before post-apply scripts
	require.Len(t, runner.commands, 4)
	assert.Equal(t, "echo base-pre", runner.commands[0])
	assert.Equal(t, "echo profile-pre", runner.commands[1])
	assert.Equal(t, "echo base-post", runner.commands[2])
	assert.Equal(t, "echo profile-post", runner.commands[3])
}
```

- [ ] **Step 2: Add test for pre_apply script failure halts apply**

```go
func TestApply_PreApplyScriptFailureHaltsApply(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{
		failOn: map[string]error{
			"echo fail": fmt.Errorf("exit status 1"),
		},
	}
	installerCalled := false
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "will-fail", Run: "echo fail"},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &trackingInstaller{called: &installerCalled},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "will-fail")
	assert.False(t, installerCalled, "installer should not run after pre_apply failure")
}
```

- [ ] **Step 3: Add test for --stages filtering**

```go
func TestApply_StagesFiltering(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	installerCalled := false
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "pre", Run: "echo pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "post", Run: "echo post"},
				},
				Packages: []profile.PackageEntry{
					{Name: "git", Install: profile.InstallCmd{Command: "brew install git"}},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &trackingInstaller{called: &installerCalled},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	// Only run packages stage — scripts should not run
	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		Stages:    "packages",
	})
	require.NoError(t, err)

	assert.True(t, installerCalled, "installer should run when packages stage is selected")
	assert.Empty(t, runner.commands, "scripts should not run when not in stages")
	assert.Empty(t, mockDep.deployed, "configs should not deploy when not in stages")
}
```

- [ ] **Step 4: Add test for invalid stage name**

```go
func TestApply_InvalidStageReturnsError(t *testing.T) {
	a := New(Deps{
		Reporter: &mockReporter{},
		Loader: &mockLoader{
			meta: &profile.FacetMeta{},
		},
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: "/fake",
		StateDir:  "/fake",
		Stages:    "invalid_stage",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stage")
}
```

- [ ] **Step 5: Add test for variable resolution in scripts**

```go
func TestApply_ScriptsResolveVariables(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				Vars: map[string]any{
					"git": map[string]any{"email": "sarah@acme.com"},
				},
				PreApply: []profile.ScriptEntry{
					{Name: "setup", Run: `echo "${facet:git.email}"`},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &mockInstaller{},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
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

	require.Len(t, runner.commands, 1)
	assert.Equal(t, `echo "sarah@acme.com"`, runner.commands[0])
}
```

- [ ] **Step 6: Run all apply tests**

Run: `go test ./internal/app/ -run TestApply -v`
Expected: all tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/app/apply_test.go
git commit -m "test: add apply tests for pre/post apply scripts and stages filtering"
```

---

### Task 8: Update documentation topics

**Files:**
- Delete: `internal/docs/topics/init.md`
- Create: `internal/docs/topics/scripts.md`
- Modify: `internal/docs/docs.go:24`
- Modify: `internal/docs/topics/commands.md`
- Modify: `internal/docs/topics/config.md:102`
- Modify: `internal/docs/topics/examples.md`

- [ ] **Step 1: Delete init.md and create scripts.md**

```bash
rm internal/docs/topics/init.md
```

Create `internal/docs/topics/scripts.md`:

```markdown
# Apply Scripts

Pre-apply and post-apply scripts run shell commands during `facet apply`.

- `pre_apply`: runs after config deployment, before package installation
- `post_apply`: runs after package installation

## YAML Format

Define scripts in `base.yaml` and profile YAML files:

` ` `yaml
pre_apply:
  - name: setup ssh keys
    run: ssh-keygen -t ed25519 -C "${facet:git.email}"

post_apply:
  - name: configure git
    run: git config --global user.email "${facet:git.email}"
  - name: setup editor plugins
    run: |
      export EDITOR_HOME="${facet:editor.home}"
      ./scripts/setup-editor.sh
` ` `

Each entry has:

- `name`: human-readable label shown in the output
- `run`: shell command or multi-line script executed via `sh -c`

## Variable Resolution

`${facet:var.name}` references are resolved in the `run` field using the full
merged variable set (base + profile + `.local.yaml`). The `name` field is not resolved.

For external scripts, pass variables explicitly:

` ` `yaml
pre_apply:
  - name: run python setup
    run: |
      export DB_URL="${facet:db.url}"
      python3 ./scripts/setup.py
` ` `

## Merge Rules

Base scripts run first, then profile scripts are appended. No deduplication: if both
layers define a script with the same name, both run. `.local.yaml` can also contribute
scripts (merged as the third layer).

## Execution

Scripts run sequentially in order. If any script exits with a non-zero code, execution
stops immediately and `facet apply` returns an error.

Scripts run with the config directory as the working directory, so relative paths like
`./scripts/setup.sh` resolve against the config repo.

## Interactive Scripts

Scripts inherit the terminal's stdin, stdout, and stderr. This means scripts can
prompt for user input, display real-time progress, and work with tools that require a
TTY (like `ssh-keygen` or `gpg --gen-key`).

` ` `yaml
pre_apply:
  - name: generate ssh key
    run: ssh-keygen -t ed25519 -C "${facet:git.email}"

post_apply:
  - name: login to registry
    run: docker login registry.acme.com
` ` `

Output from scripts streams directly to the terminal as the script runs. If a script
fails, the exit code is reported — any error output will already be visible above the
facet summary.

## Stages

Scripts are part of the apply stage pipeline. Use `--stages` to control which stages
run:

` ` `bash
facet apply work --stages pre_apply,packages
facet apply work --stages configs,post_apply
` ` `

Valid stages (in execution order):

| Stage | Description |
|-------|-------------|
| `configs` | Deploy symlinks and templates |
| `pre_apply` | Run pre_apply scripts |
| `packages` | Install packages |
| `post_apply` | Run post_apply scripts |
| `ai` | Apply AI configuration |

Default (no `--stages` flag): all stages run.
```

Note: the triple backticks in the code above should have no spaces — the spaces are only to avoid escaping issues in this plan document.

- [ ] **Step 2: Update docs.go topic list**

In `internal/docs/docs.go`, replace:
```go
{Name: "init", Description: "Post-install initialization scripts"},
```
with:
```go
{Name: "scripts", Description: "Pre-apply and post-apply scripts"},
```

- [ ] **Step 3: Update commands.md**

Replace the `## facet init <profile>` section and update the `## facet apply <profile>` section.

In the apply section, add the `--stages` flag and update the "What it does" list:

```markdown
## `facet apply <profile>`

Apply a configuration profile.

` ` `bash
facet apply work
facet apply work --dry-run
facet apply work --force
facet apply work --skip-failure
facet apply work --stages configs,packages
` ` `

Flags:

- `--dry-run`: preview the resolved actions without writing changes
- `--force`: replace conflicting non-facet files and unapply previous state first when needed
- `--skip-failure`: warn on deploy failures instead of rolling back immediately
- `--stages`: comma-separated list of stages to run (default: all)

Valid stages (in execution order):

| Stage | Description |
|-------|-------------|
| `configs` | Deploy symlinks and templates |
| `pre_apply` | Run pre_apply scripts |
| `packages` | Install packages |
| `post_apply` | Run post_apply scripts |
| `ai` | Apply AI configuration |

What it does:

1. Loads `facet.yaml`, `base.yaml`, the selected profile, and `.local.yaml`
2. Merges the three layers
3. Resolves `${facet:...}` variables
4. Unapplies previous state if switching profiles or using `--force`
5. Deploys config files (if `configs` stage)
6. Runs pre_apply scripts (if `pre_apply` stage)
7. Installs packages (if `packages` stage)
8. Runs post_apply scripts (if `post_apply` stage)
9. Applies AI configuration (if `ai` stage)
10. Writes `.state.json`
```

Remove the entire `## facet init <profile>` section (lines 48-70 of commands.md).

- [ ] **Step 4: Update config.md field reference**

Replace the `init` row in the field reference table:
```markdown
| `init` | list of InitScript | Init scripts run by `facet init`. See `facet docs init`. |
```
with:
```markdown
| `pre_apply` | list of ScriptEntry | Scripts run before package install. See `facet docs scripts`. |
| `post_apply` | list of ScriptEntry | Scripts run after package install. See `facet docs scripts`. |
```

- [ ] **Step 5: Update examples.md**

Replace the "Init scripts in base.yaml" and "Init scripts in profiles/work.yaml" sections and the "What Happens On `facet init work`" section.

Replace:
```markdown
## Init scripts in `base.yaml`

` ` `yaml
init:
  - name: configure git credentials
    run: git config --global credential.helper store
` ` `

## Init scripts in `profiles/work.yaml`

` ` `yaml
init:
  - name: authenticate gcloud
    run: |
      export PROJECT="${facet:gcp_project}"
      gcloud config set project "$PROJECT"
` ` `
```

with:

```markdown
## Pre-apply scripts in `base.yaml`

` ` `yaml
pre_apply:
  - name: setup ssh keys
    run: ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
` ` `

## Post-apply scripts in `profiles/work.yaml`

` ` `yaml
post_apply:
  - name: authenticate gcloud
    run: |
      export PROJECT="${facet:gcp_project}"
      gcloud config set project "$PROJECT"
` ` `
```

Replace the "What Happens On `facet init work`" section with:

```markdown
## What Happens During `facet apply work` (Scripts)

- Pre-apply scripts from base and profile run after configs are deployed
- Post-apply scripts run after packages are installed
- `${facet:...}` variables are resolved in the `run` fields
- Scripts run sequentially; any failure stops execution
- Use `--stages pre_apply,post_apply` to run only scripts
```

- [ ] **Step 6: Run docs tests**

Run: `go test ./internal/docs/ -v`
Expected: all docs tests pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "docs: replace init topic with scripts, update commands and config references"
```

---

### Task 9: Update E2E tests and fixtures

**Files:**
- Modify: `e2e/fixtures/setup-basic.sh`
- Modify: `e2e/suites/11-init-scripts.sh` (rewrite)
- Modify: `e2e/suites/helpers.sh`

- [ ] **Step 1: Update setup-basic.sh fixture**

In `e2e/fixtures/setup-basic.sh`, replace the `init:` blocks in base.yaml and profiles/work.yaml.

In the base.yaml heredoc, replace:
```yaml
init:
  - name: create-marker
    run: touch "$HOME/.facet-base-init-ran"
```
with:
```yaml
pre_apply:
  - name: create-pre-marker
    run: touch "$HOME/.facet-base-pre-ran"

post_apply:
  - name: create-post-marker
    run: touch "$HOME/.facet-base-post-ran"
```

In the profiles/work.yaml heredoc, replace:
```yaml
init:
  - name: create-work-marker
    run: touch "$HOME/.facet-work-init-ran"
  - name: write-resolved-var
    run: echo "${facet:git.email}" > "$HOME/.facet-init-email"
```
with:
```yaml
pre_apply:
  - name: create-work-pre-marker
    run: touch "$HOME/.facet-work-pre-ran"

post_apply:
  - name: create-work-post-marker
    run: touch "$HOME/.facet-work-post-ran"
  - name: write-resolved-var
    run: echo "${facet:git.email}" > "$HOME/.facet-post-email"
```

- [ ] **Step 2: Remove facet_init from helpers.sh**

In `e2e/suites/helpers.sh`, delete:
```bash
facet_init() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" init "$@"
}
```

- [ ] **Step 3: Rewrite 11-init-scripts.sh as 11-apply-scripts.sh**

Rename and rewrite the test suite:

```bash
mv e2e/suites/11-init-scripts.sh e2e/suites/11-apply-scripts.sh
```

Write the new content:

```bash
#!/bin/bash
# e2e/suites/11-apply-scripts.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Test: pre_apply and post_apply scripts run during apply
facet_apply work

assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-work-pre-ran"
assert_file_exists "$HOME/.facet-base-post-ran"
assert_file_exists "$HOME/.facet-work-post-ran"
echo "  pre_apply and post_apply scripts run during apply"

# Test: variable resolution in post_apply scripts
assert_file_exists "$HOME/.facet-post-email"
assert_file_contains "$HOME/.facet-post-email" "sarah@acme.com"
echo "  scripts resolve variables in run strings"

# Test: scripts are re-runnable (apply again)
facet_apply work --force
assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-base-post-ran"
echo "  scripts are re-runnable on re-apply"

# Test: pre_apply script failure halts apply
cat > "$HOME/dotfiles/profiles/failing.yaml" << 'YAML'
extends: base

pre_apply:
  - name: will-succeed
    run: touch "$HOME/.facet-fail-first"
  - name: will-fail
    run: exit 1
  - name: should-be-skipped
    run: touch "$HOME/.facet-fail-skipped"
YAML

assert_exit_code 1 bash -c "facet -c $HOME/dotfiles -s $HOME/.facet apply failing"
assert_file_exists "$HOME/.facet-fail-first"
assert_file_not_exists "$HOME/.facet-fail-skipped"
echo "  pre_apply fails fast on non-zero exit, skips remaining"

# Test: --stages filters which stages run
setup_basic

# Remove marker files from previous test
rm -f "$HOME/.facet-base-pre-ran" "$HOME/.facet-work-pre-ran"
rm -f "$HOME/.facet-base-post-ran" "$HOME/.facet-work-post-ran"

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work --stages packages
assert_file_not_exists "$HOME/.facet-base-pre-ran"
assert_file_not_exists "$HOME/.facet-base-post-ran"
echo "  --stages packages skips scripts"

# Test: --stages runs only selected stages
rm -f "$HOME/.facet-base-pre-ran" "$HOME/.facet-work-pre-ran"

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work --force --stages pre_apply
assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-work-pre-ran"
assert_file_not_exists "$HOME/.facet-base-post-ran"
echo "  --stages pre_apply runs only pre_apply scripts"

# Test: apply with no scripts succeeds
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen
YAML

cat > "$HOME/dotfiles/profiles/noscripts.yaml" << 'YAML'
extends: base
YAML

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply noscripts
echo "  apply succeeds when no scripts are defined"

# Test: scripts can read from stdin (interactive support)
setup_basic

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen

post_apply:
  - name: read-stdin
    run: read -r line && echo "$line" > "$HOME/.facet-stdin-result"
YAML

cat > "$HOME/dotfiles/profiles/interactive.yaml" << 'YAML'
extends: base
YAML

echo "hello-from-stdin" | facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply interactive
assert_file_exists "$HOME/.facet-stdin-result"
assert_file_contains "$HOME/.facet-stdin-result" "hello-from-stdin"
echo "  scripts can read from stdin (interactive support)"

# Test: script output streams to stdout
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen

pre_apply:
  - name: echo-stdout
    run: echo "visible-output"
YAML

output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply interactive 2>&1)
echo "$output" | grep -q "visible-output"
echo "  script output streams to terminal"
```

- [ ] **Step 4: Run E2E tests**

Run: `go test ./e2e/ -v -run 11` (or the project's E2E runner)
Expected: all E2E tests pass

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: rewrite E2E tests for pre/post apply scripts and stages"
```

---

### Task 10: Run full test suite and verify

**Files:** none (verification only)

- [ ] **Step 1: Run all unit tests**

Run: `go test ./... -v`
Expected: all tests pass

- [ ] **Step 2: Run build**

Run: `go build -o /dev/null ./...`
Expected: compiles successfully

- [ ] **Step 3: Run E2E tests**

Run the full E2E suite per the project's runner.
Expected: all suites pass

- [ ] **Step 4: Manual smoke test**

```bash
go build -o facet .
./facet apply --help
```

Expected: `--stages` flag is visible with description listing valid stage names.

```bash
./facet docs scripts
```

Expected: shows the scripts documentation topic.

```bash
./facet docs
```

Expected: topic list shows `scripts` instead of `init`.

- [ ] **Step 5: Final commit if any fixes were needed**

Only if prior steps required fixes.
