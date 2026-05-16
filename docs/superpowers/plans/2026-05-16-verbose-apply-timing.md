# Verbose Apply Timing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add complete `--verbose` timing diagnostics for the full `facet apply` pipeline.

**Architecture:** Add timing primitives to the existing reporter boundary, then instrument the app, package installer, and AI orchestrator through dependency injection. Timing is terminal-only, no-op when verbose is disabled, and must not alter apply semantics.

**Tech Stack:** Go, Cobra CLI, existing reporter/app/packages/ai packages, shell E2E suites.

---

## File Structure

- Modify `internal/common/reporter/reporter.go`: add duration formatting and verbose-only timing methods.
- Modify `internal/common/reporter/reporter_test.go`: cover timing output and quiet mode.
- Modify `internal/app/interfaces.go`: extend the app reporter interface with timing methods.
- Modify `internal/app/test_helpers_test.go`: update mock reporter timing methods.
- Modify `internal/app/apply.go`: wrap full apply pipeline and stage items in timing calls.
- Modify `internal/app/apply_test.go`: update existing verbose assertions and add timing coverage.
- Modify `internal/packages/installer.go`: add injected progress reporter and time package checks/installs.
- Modify `internal/packages/installer_test.go`: add timing mock and coverage for package outcomes.
- Modify `main.go`: pass reporter to package installer.
- Modify `internal/ai/orchestrator.go`: time permissions, skills, verification, removals, and MCP operations.
- Modify `internal/ai/orchestrator_test.go`: update mock reporter and add AI timing test.
- Modify `e2e/suites/16-verbose-flag.sh`: assert timing lines exist only with verbose.
- Modify `internal/docs/topics/commands.md`, `README.md`, and `docs/architecture/v1-design-spec.md`: document verbose timing.

---

### Task 1: Reporter Timing Primitives

**Files:**
- Modify: `internal/common/reporter/reporter.go`
- Modify: `internal/common/reporter/reporter_test.go`

- [ ] **Step 1: Add failing reporter tests**

Append these tests to `internal/common/reporter/reporter_test.go`:

```go
func TestReporter_ProgressDuration_Silent_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.ProgressDuration("Loading profile", "ok", 12*time.Millisecond, nil)

	assert.Empty(t, buf.String())
}

func TestReporter_ProgressDuration_PrintsOutcomeAndDuration(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("Loading profile", "ok", 12*time.Millisecond, nil)

	out := buf.String()
	assert.Contains(t, out, "Loading profile ... ok 12ms")
}

func TestReporter_ProgressDuration_PrintsSecondsForLongDurations(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("Installing packages", "done", 1500*time.Millisecond, nil)

	assert.Contains(t, buf.String(), "Installing packages ... done 1.5s")
}

func TestReporter_ProgressDuration_PrintsErrorLine(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("  -> node install", "failed", 21*time.Millisecond, errors.New("exit status 1"))

	out := buf.String()
	assert.Contains(t, out, "  -> node install ... failed 21ms")
	assert.Contains(t, out, "     error: exit status 1")
}

func TestReporter_ProgressStart_PrintsStartAndDone(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	done := r.ProgressStart("Deploying configs")
	done("done", nil)

	out := buf.String()
	assert.Contains(t, out, "Deploying configs ... start")
	assert.Regexp(t, `Deploying configs \.\.\. done [0-9]+ms`, out)
}

func TestReporter_ProgressStep_PrintsFailureAndReturnsError(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)
	expectedErr := errors.New("boom")

	err := r.ProgressStep("Resolving extends", func() error {
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
	out := buf.String()
	assert.Regexp(t, `Resolving extends \.\.\. failed [0-9]+ms`, out)
	assert.Contains(t, out, "     error: boom")
}
```

Also add these imports:

```go
import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: Run reporter tests and verify they fail**

Run:

```bash
go test ./internal/common/reporter
```

Expected: FAIL because `ProgressDuration`, `ProgressStart`, and `ProgressStep` do not exist.

- [ ] **Step 3: Implement reporter timing API**

In `internal/common/reporter/reporter.go`, add `time` to imports:

```go
import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)
```

Add these methods below `Progress`:

```go
// ProgressDuration prints a completed timed operation when verbose mode is enabled.
func (r *Reporter) ProgressDuration(label, outcome string, elapsed time.Duration, err error) {
	if !r.verbose {
		return
	}
	fmt.Fprintf(r.w, "%s ... %s %s\n", label, outcome, formatDuration(elapsed))
	if err != nil {
		fmt.Fprintf(r.w, "     error: %v\n", err)
	}
}

// ProgressStart prints a grouped operation start line and returns a completion function.
func (r *Reporter) ProgressStart(label string) func(outcome string, err error) {
	if !r.verbose {
		return func(string, error) {}
	}
	start := time.Now()
	fmt.Fprintf(r.w, "%s ... start\n", label)
	return func(outcome string, err error) {
		r.ProgressDuration(label, outcome, time.Since(start), err)
	}
}

// ProgressStep runs fn and prints the operation outcome and duration.
func (r *Reporter) ProgressStep(label string, fn func() error) error {
	start := time.Now()
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	r.ProgressDuration(label, outcome, time.Since(start), err)
	return err
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		ms := d.Round(time.Millisecond).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
```

- [ ] **Step 4: Run reporter tests and verify they pass**

Run:

```bash
go test ./internal/common/reporter
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/common/reporter/reporter.go internal/common/reporter/reporter_test.go
git commit -m "feat: add verbose timing reporter"
```

---

### Task 2: App Pipeline Timing

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/test_helpers_test.go`
- Modify: `internal/app/apply.go`
- Modify: `internal/app/apply_test.go`

- [ ] **Step 1: Extend app reporter tests/mocks**

In `internal/app/interfaces.go`, add `time` to imports and extend `Reporter`:

```go
import (
	"time"

	"facet/internal/ai"
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)
```

Add these methods to the `Reporter` interface:

```go
	ProgressDuration(label, outcome string, elapsed time.Duration, err error)
	ProgressStart(label string) func(outcome string, err error)
	ProgressStep(label string, fn func() error) error
```

In `internal/app/test_helpers_test.go`, add `time` to imports and implement:

```go
func (m *mockReporter) ProgressDuration(label, outcome string, _ time.Duration, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... "+outcome)
	if err != nil {
		m.messages = append(m.messages, "progress-error: "+err.Error())
	}
}

func (m *mockReporter) ProgressStart(label string) func(outcome string, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... start")
	return func(outcome string, err error) {
		m.ProgressDuration(label, outcome, 0, err)
	}
}

func (m *mockReporter) ProgressStep(label string, fn func() error) error {
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	m.ProgressDuration(label, outcome, 0, err)
	return err
}
```

- [ ] **Step 2: Add failing app timing assertions**

Update `TestApply_EmitsVerboseProgress_ForStagesAndItems` in `internal/app/apply_test.go` so assertions use substring matching instead of exact slice membership:

```go
progressOutput := strings.Join(progressMessages, "\n")
assert.Contains(t, progressOutput, "progress: Loading profile ... ok")
assert.Contains(t, progressOutput, "progress: Resolving extends ... ok")
assert.Contains(t, progressOutput, "progress: Merging layers ... ok")
assert.Contains(t, progressOutput, "progress: Deploying configs ... start")
assert.Contains(t, progressOutput, "progress:   -> "+targetPath+" ... ok")
assert.Contains(t, progressOutput, "progress: Installing packages ... start")
assert.Contains(t, progressOutput, "progress:   -> git")
assert.Contains(t, progressOutput, "progress: Running pre_apply scripts ... start")
assert.Contains(t, progressOutput, "progress:   -> setup ... ok")
assert.Contains(t, progressOutput, "progress: Running post_apply scripts ... start")
assert.Contains(t, progressOutput, "progress:   -> teardown ... ok")
assert.Contains(t, progressOutput, "progress: Writing state ... ok")
assert.Contains(t, progressOutput, "progress: facet apply work ... done")
```

Run:

```bash
go test ./internal/app -run TestApply_EmitsVerboseProgress_ForStagesAndItems
```

Expected: FAIL because app still emits old untimed progress lines.

- [ ] **Step 3: Instrument top-level apply steps**

In `internal/app/apply.go`, replace direct calls for single-step operations with `ProgressStep`. Use this exact pattern:

```go
applyDone := a.reporter.ProgressStart(fmt.Sprintf("facet apply %s", profileName))
applyOutcome := "done"
var applyErr error
defer func() {
	if applyErr != nil {
		applyOutcome = "failed"
	}
	applyDone(applyOutcome, applyErr)
}()
```

At each early return in `Apply`, assign `applyErr` before returning:

```go
applyErr = err
return applyErr
```

Wrap these operations:

```go
var metaErr error
metaErr = a.reporter.ProgressStep("Loading metadata", func() error {
	_, err = a.loader.LoadMeta(opts.ConfigDir)
	return err
})
if metaErr != nil {
	applyErr = fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory, or run facet scaffold to create one.\n  detail: %w", metaErr)
	return applyErr
}
```

Use the same pattern for:

```go
"Loading profile"
"Validating profile"
"Resolving extends"
"Loading local config"
"Merging base and profile"
"Merging local config"
"Validating merged config"
"Resolving variables"
"Creating state directory"
"Writing state canary"
"Reading previous state"
"Writing state"
"Rendering apply report"
```

For operations that return values, declare the value outside the closure and assign inside the closure.

- [ ] **Step 4: Instrument grouped stages and items**

In `internal/app/apply.go`, convert grouped stages to `ProgressStart`:

```go
if stages["configs"] && len(resolved.Configs) > 0 {
	done := a.reporter.ProgressStart("Deploying configs")
	var deployErr error
	// existing config loop
	if deployErr != nil {
		done("failed", deployErr)
	} else {
		done("done", nil)
	}
}
```

For each config target, wrap source resolution, target expansion, and deployment:

```go
var sourceSpec deploy.SourceSpec
err := a.reporter.ProgressStep("  -> "+target+" source", func() error {
	var stepErr error
	sourceSpec, stepErr = deploy.ResolveSourcePath(source, resolved.ConfigMeta[target], opts.ConfigDir)
	return stepErr
})
```

Then:

```go
var expandedTarget string
err = a.reporter.ProgressStep("  -> "+target+" expand", func() error {
	var stepErr error
	expandedTarget, stepErr = deploy.ExpandPath(target)
	return stepErr
})
```

Then:

```go
err = a.reporter.ProgressStep("  -> "+target, func() error {
	_, stepErr := deployer.DeployOne(expandedTarget, sourceSpec, opts.Force)
	return stepErr
})
```

In `runScripts`, replace the old item progress line with:

```go
if err := a.reporter.ProgressStep("  -> "+script.Name, func() error {
	return a.scriptRunner.Run(script.Run, dir)
}); err != nil {
	return fmt.Errorf("%s script %q failed: %w", stageName, script.Name, err)
}
```

Wrap pre/post stages with `ProgressStart("Running pre_apply scripts")` and `ProgressStart("Running post_apply scripts")`. Call the returned completion function immediately after `runScripts` returns so the stage duration does not include later stages.

- [ ] **Step 5: Run app tests**

Run:

```bash
go test ./internal/app
```

Expected: PASS after updating any remaining exact progress assertions to substring assertions.

- [ ] **Step 6: Commit**

```bash
git add internal/app/interfaces.go internal/app/test_helpers_test.go internal/app/apply.go internal/app/apply_test.go
git commit -m "feat: time apply pipeline steps"
```

---

### Task 3: Package Stage Item Timing

**Files:**
- Modify: `internal/packages/installer.go`
- Modify: `internal/packages/installer_test.go`
- Modify: `main.go`

- [ ] **Step 1: Add package timing test double**

In `internal/packages/installer_test.go`, add:

```go
type timingCall struct {
	label   string
	outcome string
}

type packageTimingReporter struct {
	calls []timingCall
}

func (r *packageTimingReporter) ProgressStep(label string, fn func() error) error {
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	r.calls = append(r.calls, timingCall{label: label, outcome: outcome})
	return err
}

func (r *packageTimingReporter) ProgressDuration(label, outcome string, _ time.Duration, _ error) {
	r.calls = append(r.calls, timingCall{label: label, outcome: outcome})
}
```

Add `time` to imports.

- [ ] **Step 2: Add failing package timing tests**

Add these tests:

```go
func TestInstallAll_ReportsCheckAndInstallTiming(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["which rg"] = errors.New("exit status 1")
	timing := &packageTimingReporter{}
	inst := NewInstallerWithProgress(runner, "macos", timing)

	pkgs := []profile.PackageEntry{{
		Name:    "ripgrep",
		Check:   profile.InstallCmd{Command: "which rg"},
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}}

	results := inst.InstallAll(pkgs)

	require.Len(t, results, 1)
	assert.Equal(t, StatusOK, results[0].Status)
	assert.Equal(t, []timingCall{
		{label: "  -> ripgrep check", outcome: "failed"},
		{label: "  -> ripgrep install", outcome: "ok"},
	}, timing.calls)
}

func TestInstallAll_ReportsPackageSkipTiming(t *testing.T) {
	runner := newMockRunner()
	timing := &packageTimingReporter{}
	inst := NewInstallerWithProgress(runner, "linux", timing)

	pkgs := []profile.PackageEntry{{
		Name: "mac-only",
		Install: profile.InstallCmd{PerOS: map[string]string{
			"macos": "brew install mac-only",
		}},
	}}

	results := inst.InstallAll(pkgs)

	require.Len(t, results, 1)
	assert.Equal(t, StatusSkipped, results[0].Status)
	assert.Equal(t, []timingCall{{label: "  -> mac-only skip", outcome: "skipped"}}, timing.calls)
}
```

Run:

```bash
go test ./internal/packages
```

Expected: FAIL because `NewInstallerWithProgress` does not exist.

- [ ] **Step 3: Implement package timing injection**

In `internal/packages/installer.go`, add `time` to imports and add these types:

```go
type ProgressReporter interface {
	ProgressStep(label string, fn func() error) error
	ProgressDuration(label, outcome string, elapsed time.Duration, err error)
}

type noopProgressReporter struct{}

func (noopProgressReporter) ProgressStep(_ string, fn func() error) error { return fn() }
func (noopProgressReporter) ProgressDuration(string, string, time.Duration, error) {}
```

Update `Installer`:

```go
type Installer struct {
	runner   CommandRunner
	osName   string
	progress ProgressReporter
}
```

Keep the existing constructor and add a new one:

```go
func NewInstaller(runner CommandRunner, osName string) *Installer {
	return NewInstallerWithProgress(runner, osName, nil)
}

func NewInstallerWithProgress(runner CommandRunner, osName string, progress ProgressReporter) *Installer {
	if progress == nil {
		progress = noopProgressReporter{}
	}
	return &Installer{runner: runner, osName: osName, progress: progress}
}
```

In `InstallAll`, report skips:

```go
if skip {
	pr.Status = StatusSkipped
	pr.Error = fmt.Sprintf("no install command for OS %q", inst.osName)
	inst.progress.ProgressDuration("  -> "+pkg.Name+" skip", "skipped", 0, nil)
	results = append(results, pr)
	continue
}
```

Wrap checks and installs:

```go
if hasCheck {
	checkErr := inst.progress.ProgressStep("  -> "+pkg.Name+" check", func() error {
		return inst.runner.Run(checkCmd)
	})
	if checkErr == nil {
		pr.Status = StatusAlreadyInstalled
		results = append(results, pr)
		continue
	}
}

err := inst.progress.ProgressStep("  -> "+pkg.Name+" install", func() error {
	return inst.runner.Run(cmd)
})
```

- [ ] **Step 4: Wire package timing in main**

In `main.go`, change:

```go
installer := packages.NewInstaller(runner, osName)
```

to:

```go
installer := packages.NewInstallerWithProgress(runner, osName, r)
```

- [ ] **Step 5: Run package tests**

Run:

```bash
go test ./internal/packages
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/packages/installer.go internal/packages/installer_test.go main.go
git commit -m "feat: time package checks and installs"
```

---

### Task 4: AI Operation Timing

**Files:**
- Modify: `internal/ai/orchestrator.go`
- Modify: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Extend AI reporter interface and mock**

In `internal/ai/orchestrator.go`, add `time` to imports and add timing methods to the `Reporter` interface:

```go
	ProgressDuration(label, outcome string, elapsed time.Duration, err error)
	ProgressStart(label string) func(outcome string, err error)
	ProgressStep(label string, fn func() error) error
```

In `internal/ai/orchestrator_test.go`, replace `type mockReporter struct{}` with:

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
func (m *mockReporter) ProgressDuration(label, outcome string, _ time.Duration, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... "+outcome)
	if err != nil {
		m.messages = append(m.messages, "progress-error: "+err.Error())
	}
}
func (m *mockReporter) ProgressStart(label string) func(outcome string, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... start")
	return func(outcome string, err error) {
		m.ProgressDuration(label, outcome, 0, err)
	}
}
func (m *mockReporter) ProgressStep(label string, fn func() error) error {
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	m.ProgressDuration(label, outcome, 0, err)
	return err
}
```

Add `time` to test imports.

- [ ] **Step 2: Add failing AI timing test**

Add this test to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_EmitsTimingForAIItems(t *testing.T) {
	provider := &mockProvider{}
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"owner/repo": {"code-review"},
		},
	}
	reporter := &mockReporter{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		skillsMgr,
		reporter,
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Permissions: ResolvedPermissions{Allow: []string{"Read"}},
			Skills: []ResolvedSkill{{
				Source: "owner/repo",
				Name:   "code-review",
			}},
			MCPs: []ResolvedMCP{{
				Name:    "filesystem",
				Command: "true",
			}},
		},
	}

	_, err := orch.Apply(config, nil)
	require.NoError(t, err)

	out := strings.Join(reporter.messages, "\n")
	assert.Contains(t, out, "progress: AI permissions ... start")
	assert.Contains(t, out, "progress:   -> permissions claude-code ... ok")
	assert.Contains(t, out, "progress: AI skills ... start")
	assert.Contains(t, out, "progress:   -> skills install owner/repo [claude-code] ... ok")
	assert.Contains(t, out, "progress:   -> skills verify owner/repo ... ok")
	assert.Contains(t, out, "progress: AI MCPs ... start")
	assert.Contains(t, out, "progress:   -> mcp register filesystem claude-code ... ok")
}
```

Run:

```bash
go test ./internal/ai -run TestOrchestrator_EmitsTimingForAIItems
```

Expected: FAIL because AI timings are not emitted.

- [ ] **Step 3: Instrument AI permissions**

In `applyPermissions`, add:

```go
done := o.reporter.ProgressStart("AI permissions")
outcome := "done"
defer func() { done(outcome, nil) }()
```

Wrap provider calls:

```go
if err := o.reporter.ProgressStep("  -> permissions remove "+agent, func() error {
	return provider.RemovePermissions(perms)
}); err != nil {
	o.reporter.Warning(fmt.Sprintf("failed to remove permissions for dropped agent %q: %v", agent, err))
} else {
	o.reporter.Success(fmt.Sprintf("removed permissions for %s", agent))
}
```

And:

```go
if err := o.reporter.ProgressStep("  -> permissions "+agent, func() error {
	return provider.ApplyPermissions(perms)
}); err != nil {
	o.reporter.Error(fmt.Sprintf("failed to apply permissions for %q: %v", agent, err))
	continue
}
```

- [ ] **Step 4: Instrument AI skills**

At the start of `applySkills`, add:

```go
done := o.reporter.ProgressStart("AI skills")
defer done("done", nil)
```

Wrap orphan installed-skill reads:

```go
var skillsToRemove []string
err := o.reporter.ProgressStep("  -> skills detect orphans "+prevSkill.Source+" "+agent, func() error {
	var stepErr error
	skillsToRemove, stepErr = o.sourceSkillsToRemove(prevSkill.Source, agent, currentSkills, currentAllSources)
	return stepErr
})
```

Wrap removes:

```go
if err := o.reporter.ProgressStep("  -> skills remove "+prevSkill.Name+" "+agent, func() error {
	return o.skillsManager.Remove([]string{prevSkill.Name}, []string{agent})
}); err != nil {
	o.reporter.Warning(fmt.Sprintf("failed to remove orphan skill %q from %q: %v", prevSkill.Name, agent, err))
} else {
	o.reporter.Success(fmt.Sprintf("removed orphan skill %q from %s", prevSkill.Name, agent))
}
```

Wrap installs:

```go
label := fmt.Sprintf("  -> skills install %s [%s]", gk.source, strings.Join(agents, ","))
if err := o.reporter.ProgressStep(label, func() error {
	return o.skillsManager.Install(gk.source, installSkills, agents)
}); err != nil {
	o.reporter.Warning(fmt.Sprintf("failed to install skills from %q: %v", gk.source, err))
	continue
}
```

Wrap verification:

```go
var verified []string
verifyErr := o.reporter.ProgressStep("  -> skills verify "+gk.source, func() error {
	var stepErr error
	verified, stepErr = o.skillsManager.InstalledForSource(gk.source)
	return stepErr
})
```

Use `verifyErr` in the existing verification branches instead of calling `InstalledForSource` directly.

- [ ] **Step 5: Instrument AI MCPs**

At the start of `applyMCPs`, add:

```go
done := o.reporter.ProgressStart("AI MCPs")
defer done("done", nil)
```

Wrap removals:

```go
if err := o.reporter.ProgressStep("  -> mcp remove "+prevMCP.Name+" "+agent, func() error {
	return provider.RemoveMCP(prevMCP.Name)
}); err != nil {
	o.reporter.Warning(fmt.Sprintf("failed to remove orphan MCP %q from %q: %v", prevMCP.Name, agent, err))
} else {
	o.reporter.Success(fmt.Sprintf("removed orphan MCP %q from %s", prevMCP.Name, agent))
}
```

Wrap registrations:

```go
if err := o.reporter.ProgressStep("  -> mcp register "+mcpName+" "+agent, func() error {
	return provider.RegisterMCP(mcpCfg)
}); err != nil {
	o.reporter.Warning(fmt.Sprintf("failed to register MCP %q for %q: %v", mcpName, agent, err))
	continue
}
```

- [ ] **Step 6: Run AI tests**

Run:

```bash
go test ./internal/ai
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "feat: time ai apply operations"
```

---

### Task 5: E2E and Documentation

**Files:**
- Modify: `e2e/suites/16-verbose-flag.sh`
- Modify: `internal/docs/topics/commands.md`
- Modify: `README.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Update verbose E2E timing assertions**

In `e2e/suites/16-verbose-flag.sh`, replace the first verbose assertions with:

```bash
echo "$output" | grep -Eq "facet apply work \.\.\. start" || { echo "  FAIL: missing total apply start timing"; exit 1; }
echo "$output" | grep -Eq "Loading profile \.\.\. ok [0-9]+ms|Loading profile \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed 'Loading profile'"; exit 1; }
echo "$output" | grep -Eq "Resolving extends \.\.\. ok [0-9]+ms|Resolving extends \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed 'Resolving extends'"; exit 1; }
echo "$output" | grep -Eq "Merging base and profile \.\.\. ok [0-9]+ms|Merging base and profile \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed merge"; exit 1; }
echo "$output" | grep -Eq "Deploying configs \.\.\. start" || { echo "  FAIL: missing config timing start"; exit 1; }
echo "$output" | grep -Eq "Installing packages \.\.\. start" || { echo "  FAIL: missing package timing start"; exit 1; }
echo "$output" | grep -Eq "Running pre_apply scripts \.\.\. start" || { echo "  FAIL: missing pre_apply timing start"; exit 1; }
echo "$output" | grep -Eq "Running post_apply scripts \.\.\. start" || { echo "  FAIL: missing post_apply timing start"; exit 1; }
echo "$output" | grep -Eq "Writing state \.\.\. ok [0-9]+ms|Writing state \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing state write timing"; exit 1; }
echo "$output" | grep -Eq "facet apply work \.\.\. done [0-9]+ms|facet apply work \.\.\. done [0-9]+\.[0-9]s" || { echo "  FAIL: missing total apply done timing"; exit 1; }
echo "  --verbose shows timed stage progress lines"
```

Update item assertions:

```bash
echo "$output" | grep -Eq -- "-> ripgrep (check|install|skip) \.\.\. (ok|failed|skipped)" || { echo "  FAIL: missing package item timing for 'ripgrep'"; exit 1; }
echo "$output" | grep -Eq -- "-> create-pre-marker \.\.\. ok" || { echo "  FAIL: missing pre_apply item timing for 'create-pre-marker'"; exit 1; }
echo "  --verbose shows timed item-level detail"
```

Update quiet assertions so they reject timing:

```bash
if echo "$output_quiet" | grep -q "\.\.\. ok [0-9]"; then
	echo "  FAIL: timing appeared without --verbose flag"
	exit 1
fi
```

- [ ] **Step 2: Update docs**

In `README.md`, change the `--verbose` apply example comment to:

```text
facet apply work --verbose         # stream stage, item, and duration diagnostics
```

In the flags table row for `--verbose`, use:

```markdown
| `--verbose` | `-v` | false | Stream stage, item, and duration diagnostics during apply |
```

In `internal/docs/topics/commands.md`, update the same wording for the global flag and apply examples.

In `docs/architecture/v1-design-spec.md`, update the CLI flag description and add one sentence near the apply workflow:

```markdown
With `--verbose`, each apply step and selected stage item reports an outcome and elapsed duration; non-verbose output remains the final summary only.
```

- [ ] **Step 3: Run focused E2E**

Run:

```bash
go test -tags e2e ./e2e -run TestE2E_Native
```

Expected: PASS. This runs the native E2E harness, including `16-verbose-flag.sh`.

- [ ] **Step 4: Run doc search check**

Run:

```bash
rg -n -- "--verbose|verbose" README.md internal/docs/topics docs/architecture/v1-design-spec.md
```

Expected: all `--verbose` descriptions mention timing or duration diagnostics where they describe apply progress.

- [ ] **Step 5: Commit**

```bash
git add e2e/suites/16-verbose-flag.sh internal/docs/topics/commands.md README.md docs/architecture/v1-design-spec.md
git commit -m "docs: document verbose apply timing"
```

---

### Task 6: Final Verification

**Files:**
- No source edits expected.

- [ ] **Step 1: Run unit tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run full pre-commit**

Run:

```bash
make pre-commit
```

Expected: PASS. This includes unit tests, native E2E, and Docker Linux E2E. Docker must be running locally.

- [ ] **Step 3: Inspect git status**

Run:

```bash
git status --short
```

Expected: clean working tree.
