# AI Skills Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `facet ai skills check` and `facet ai skills update` as thin passthroughs to `npx skills`, and document skill source format options.

**Architecture:** Extend `SkillsManager` interface with `Check()`/`Update()` methods that use a new `RunInteractive()` method on `CommandRunner` to stream output. Wire through `app.App` methods to new Cobra commands under `facet ai skills`. Pass the `SkillsManager` directly to `App` as a new dep field.

**Tech Stack:** Go, Cobra, os/exec

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `internal/ai/interfaces.go` | Add `RunInteractive` to `CommandRunner`, add `Check`/`Update` to `SkillsManager` |
| Modify | `internal/common/execrunner/execrunner.go` | Implement `RunInteractive` (stdout/stderr streaming) |
| Modify | `internal/ai/skills_manager.go` | Implement `Check()` and `Update()` |
| Modify | `internal/ai/test_helpers_test.go` | Add `RunInteractive` to mock runners |
| Create | `internal/common/execrunner/execrunner_test.go` | Test `RunInteractive` |
| Modify | `internal/ai/skills_manager_test.go` | Add tests for `Check()` and `Update()` |
| Modify | `internal/app/interfaces.go` | Add `SkillsManager` interface at app level |
| Modify | `internal/app/app.go` | Add `skillsManager` field to `App` and `Deps` |
| Create | `internal/app/ai_skills.go` | `AISkillsCheck()` and `AISkillsUpdate()` methods |
| Create | `internal/app/ai_skills_test.go` | Unit tests for app-layer methods |
| Modify | `main.go` | Pass `skillsMgr` to `App` via new `Deps.SkillsManager` field |
| Create | `cmd/ai.go` | `ai` parent command (help only) |
| Create | `cmd/ai_skills.go` | `skills` subcommand under `ai` (help only) |
| Create | `cmd/ai_skills_check.go` | Leaf command delegating to `app.App.AISkillsCheck()` |
| Create | `cmd/ai_skills_update.go` | Leaf command delegating to `app.App.AISkillsUpdate()` |
| Modify | `cmd/root.go` | Register `ai` command |
| Modify | `internal/docs/topics/ai.md` | Document source formats + new commands |
| Modify | `internal/docs/topics/commands.md` | Add new commands to CLI reference |
| Modify | `README.md` | Add AI configuration section |
| Create | `e2e/suites/13-ai-skills-commands.sh` | E2E tests for new commands |

---

### Task 1: Add `RunInteractive` to `CommandRunner` interface and implementation

**Files:**
- Modify: `internal/ai/interfaces.go`
- Modify: `internal/common/execrunner/execrunner.go`
- Create: `internal/common/execrunner/execrunner_test.go`
- Modify: `internal/ai/test_helpers_test.go`

- [ ] **Step 1: Write the failing test for `RunInteractive`**

Create `internal/common/execrunner/execrunner_test.go`:

```go
package execrunner

import (
	"testing"
)

func TestRunner_RunInteractive_Success(t *testing.T) {
	r := New()
	err := r.RunInteractive("echo", "hello")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunner_RunInteractive_Failure(t *testing.T) {
	r := New()
	err := r.RunInteractive("false")
	if err == nil {
		t.Fatal("expected error from false command, got nil")
	}
}

func TestRunner_RunInteractive_CommandNotFound(t *testing.T) {
	r := New()
	err := r.RunInteractive("nonexistent-binary-xyz")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/common/execrunner/ -run TestRunner_RunInteractive -v`
Expected: Compilation error — `RunInteractive` method does not exist.

- [ ] **Step 3: Add `RunInteractive` to `CommandRunner` interface**

Edit `internal/ai/interfaces.go` — add `RunInteractive` to the `CommandRunner` interface:

```go
// CommandRunner executes a command name with a stable argv vector. It is
// defined here (not imported from packages/) to avoid cross-domain coupling.
type CommandRunner interface {
	Run(name string, args ...string) error
	RunInteractive(name string, args ...string) error
}
```

- [ ] **Step 4: Implement `RunInteractive` on `execrunner.Runner`**

Edit `internal/common/execrunner/execrunner.go` — add the import for `"os"` and the new method:

```go
package execrunner

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner executes commands directly without going through a shell.
type Runner struct{}

// New constructs a Runner.
func New() *Runner {
	return &Runner{}
}

// Run executes the given command and argv, capturing output.
func (r *Runner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// RunInteractive executes the given command with stdout/stderr connected
// directly to the terminal so the user sees output in real time.
func (r *Runner) RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 5: Update mock runners in test helpers**

Edit `internal/ai/test_helpers_test.go` — add `RunInteractive` to both mocks:

```go
package ai

import "strings"

// mockRunner records commands and optionally returns an error.
type mockRunner struct {
	commands []string
	err      error
}

func (m *mockRunner) Run(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	return m.err
}

func (m *mockRunner) RunInteractive(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	return m.err
}

// sequentialMockRunner records commands and returns errors from a pre-defined
// sequence, one per call. If the call index exceeds the errors slice, nil is returned.
type sequentialMockRunner struct {
	commands []string
	errors   []error
	callIdx  int
}

func (m *sequentialMockRunner) Run(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	var err error
	if m.callIdx < len(m.errors) {
		err = m.errors[m.callIdx]
	}
	m.callIdx++
	return err
}

func (m *sequentialMockRunner) RunInteractive(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	var err error
	if m.callIdx < len(m.errors) {
		err = m.errors[m.callIdx]
	}
	m.callIdx++
	return err
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/common/execrunner/ -run TestRunner_RunInteractive -v && go test ./internal/ai/ -v`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/ai/interfaces.go internal/common/execrunner/execrunner.go internal/common/execrunner/execrunner_test.go internal/ai/test_helpers_test.go
git commit -m "feat: add RunInteractive to CommandRunner for streaming output"
```

---

### Task 2: Add `Check` and `Update` to `SkillsManager`

**Files:**
- Modify: `internal/ai/interfaces.go`
- Modify: `internal/ai/skills_manager.go`
- Modify: `internal/ai/skills_manager_test.go`

- [ ] **Step 1: Write failing tests for `Check` and `Update`**

Append to `internal/ai/skills_manager_test.go`:

```go
func TestNPXSkillsManager_Check(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Check()
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if len(runner.commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(runner.commands), runner.commands)
	}
	if runner.commands[0] != "npx --version" {
		t.Errorf("expected npx --version check, got %q", runner.commands[0])
	}
	if runner.commands[1] != "npx skills check" {
		t.Errorf("unexpected check command: got %q, want %q", runner.commands[1], "npx skills check")
	}
}

func TestNPXSkillsManager_Check_NPXNotFound(t *testing.T) {
	runner := &mockRunner{err: errors.New("command not found")}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Check()
	if err == nil {
		t.Fatal("expected error when npx not found, got nil")
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command (npx check only), got %d: %v", len(runner.commands), runner.commands)
	}
}

func TestNPXSkillsManager_Update(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Update()
	if err != nil {
		t.Fatalf("Update returned unexpected error: %v", err)
	}

	if len(runner.commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(runner.commands), runner.commands)
	}
	if runner.commands[0] != "npx --version" {
		t.Errorf("expected npx --version check, got %q", runner.commands[0])
	}
	if runner.commands[1] != "npx skills update" {
		t.Errorf("unexpected update command: got %q, want %q", runner.commands[1], "npx skills update")
	}
}

func TestNPXSkillsManager_Update_NPXNotFound(t *testing.T) {
	runner := &mockRunner{err: errors.New("command not found")}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Update()
	if err == nil {
		t.Fatal("expected error when npx not found, got nil")
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command (npx check only), got %d: %v", len(runner.commands), runner.commands)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestNPXSkillsManager_(Check|Update)" -v`
Expected: Compilation error — `Check` and `Update` methods do not exist.

- [ ] **Step 3: Add `Check` and `Update` to `SkillsManager` interface**

Edit `internal/ai/interfaces.go` — extend the `SkillsManager` interface:

```go
// SkillsManager manages skill installation/removal via external CLI.
type SkillsManager interface {
	Install(source string, skills []string, agents []string) error
	Remove(skills []string, agents []string) error
	Check() error
	Update() error
}
```

- [ ] **Step 4: Implement `Check` and `Update` on `NPXSkillsManager`**

Add to `internal/ai/skills_manager.go`:

```go
// Check runs: npx skills check
// Output streams directly to the terminal via RunInteractive.
func (m *NPXSkillsManager) Check() error {
	if err := m.checkNPX(); err != nil {
		return err
	}
	if err := m.runner.RunInteractive("npx", "skills", "check"); err != nil {
		return fmt.Errorf("skills check: %w", err)
	}
	return nil
}

// Update runs: npx skills update
// Output streams directly to the terminal via RunInteractive.
func (m *NPXSkillsManager) Update() error {
	if err := m.checkNPX(); err != nil {
		return err
	}
	if err := m.runner.RunInteractive("npx", "skills", "update"); err != nil {
		return fmt.Errorf("skills update: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -v`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ai/interfaces.go internal/ai/skills_manager.go internal/ai/skills_manager_test.go
git commit -m "feat: add Check and Update methods to SkillsManager"
```

---

### Task 3: Wire `SkillsManager` into `App` and add app-layer methods

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/ai_skills.go`
- Create: `internal/app/ai_skills_test.go`

- [ ] **Step 1: Write failing tests for `AISkillsCheck` and `AISkillsUpdate`**

Create `internal/app/ai_skills_test.go`:

```go
package app

import (
	"errors"
	"testing"
)

type mockSkillsManager struct {
	checkCalled bool
	updateCalled bool
	checkErr    error
	updateErr   error
}

func (m *mockSkillsManager) Check() error {
	m.checkCalled = true
	return m.checkErr
}

func (m *mockSkillsManager) Update() error {
	m.updateCalled = true
	return m.updateErr
}

func TestAISkillsCheck_Success(t *testing.T) {
	sm := &mockSkillsManager{}
	a := &App{skillsManager: sm}

	err := a.AISkillsCheck()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !sm.checkCalled {
		t.Error("expected Check to be called")
	}
}

func TestAISkillsCheck_Error(t *testing.T) {
	sm := &mockSkillsManager{checkErr: errors.New("npx not found")}
	a := &App{skillsManager: sm}

	err := a.AISkillsCheck()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAISkillsCheck_NilManager(t *testing.T) {
	a := &App{}

	err := a.AISkillsCheck()
	if err == nil {
		t.Fatal("expected error for nil skills manager, got nil")
	}
}

func TestAISkillsUpdate_Success(t *testing.T) {
	sm := &mockSkillsManager{}
	a := &App{skillsManager: sm}

	err := a.AISkillsUpdate()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !sm.updateCalled {
		t.Error("expected Update to be called")
	}
}

func TestAISkillsUpdate_Error(t *testing.T) {
	sm := &mockSkillsManager{updateErr: errors.New("update failed")}
	a := &App{skillsManager: sm}

	err := a.AISkillsUpdate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAISkillsUpdate_NilManager(t *testing.T) {
	a := &App{}

	err := a.AISkillsUpdate()
	if err == nil {
		t.Fatal("expected error for nil skills manager, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run "TestAISkills" -v`
Expected: Compilation errors — `skillsManager` field, `AISkillsCheck`, `AISkillsUpdate` do not exist.

- [ ] **Step 3: Add `SkillsManager` interface to app layer**

Edit `internal/app/interfaces.go` — add the interface:

```go
// SkillsManager manages AI skill check and update operations.
type SkillsManager interface {
	Check() error
	Update() error
}
```

- [ ] **Step 4: Add `skillsManager` field to `App` and `Deps`**

Edit `internal/app/app.go` — add the field to both structs and the constructor:

```go
// Deps holds all dependencies for the App.
type Deps struct {
	Loader          ProfileLoader
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

// App is the application service layer that orchestrates all facet operations.
type App struct {
	loader          ProfileLoader
	installer       Installer
	reporter        Reporter
	stateStore      StateStore
	deployerFactory DeployerFactory
	aiOrchestrator  AIOrchestrator
	scriptRunner    ScriptRunner
	skillsManager   SkillsManager
	version         string
	osName          string
}

// New creates a new App with the given dependencies.
func New(deps Deps) *App {
	return &App{
		loader:          deps.Loader,
		installer:       deps.Installer,
		reporter:        deps.Reporter,
		stateStore:      deps.StateStore,
		deployerFactory: deps.DeployerFactory,
		aiOrchestrator:  deps.AIOrchestrator,
		scriptRunner:    deps.ScriptRunner,
		skillsManager:   deps.SkillsManager,
		version:         deps.Version,
		osName:          deps.OSName,
	}
}
```

- [ ] **Step 5: Implement `AISkillsCheck` and `AISkillsUpdate`**

Create `internal/app/ai_skills.go`:

```go
package app

import "fmt"

// AISkillsCheck delegates to the SkillsManager to check for available updates.
func (a *App) AISkillsCheck() error {
	if a.skillsManager == nil {
		return fmt.Errorf("skills manager not available (is npx installed?)")
	}
	return a.skillsManager.Check()
}

// AISkillsUpdate delegates to the SkillsManager to update installed skills.
func (a *App) AISkillsUpdate() error {
	if a.skillsManager == nil {
		return fmt.Errorf("skills manager not available (is npx installed?)")
	}
	return a.skillsManager.Update()
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run "TestAISkills" -v`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/app/interfaces.go internal/app/app.go internal/app/ai_skills.go internal/app/ai_skills_test.go
git commit -m "feat: add AISkillsCheck and AISkillsUpdate to App"
```

---

### Task 4: Add Cobra commands and wire in `main.go`

**Files:**
- Create: `cmd/ai.go`
- Create: `cmd/ai_skills.go`
- Create: `cmd/ai_skills_check.go`
- Create: `cmd/ai_skills_update.go`
- Modify: `cmd/root.go`
- Modify: `main.go`

- [ ] **Step 1: Create `cmd/ai.go`**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

func newAICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ai",
		Short: "AI agent management commands",
	}
}
```

- [ ] **Step 2: Create `cmd/ai_skills.go`**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

func newAISkillsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skills",
		Short: "Manage AI agent skills",
	}
}
```

- [ ] **Step 3: Create `cmd/ai_skills_check.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newAISkillsCheckCmd(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for available skill updates",
		Long:  "Checks all globally installed skills for available updates via npx skills check.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return application.AISkillsCheck()
		},
	}
}
```

- [ ] **Step 4: Create `cmd/ai_skills_update.go`**

```go
package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newAISkillsUpdateCmd(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update all installed skills to latest versions",
		Long:  "Updates all globally installed skills to their latest versions via npx skills update.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return application.AISkillsUpdate()
		},
	}
}
```

- [ ] **Step 5: Register the command tree in `cmd/root.go`**

Edit `cmd/root.go` — add the `ai` command with its subcommands after the existing `rootCmd.AddCommand` calls:

```go
	aiCmd := newAICmd()
	aiSkillsCmd := newAISkillsCmd()
	aiSkillsCmd.AddCommand(newAISkillsCheckCmd(application))
	aiSkillsCmd.AddCommand(newAISkillsUpdateCmd(application))
	aiCmd.AddCommand(aiSkillsCmd)
	rootCmd.AddCommand(aiCmd)
```

- [ ] **Step 6: Pass `skillsMgr` to `App` in `main.go`**

Edit `main.go` — add `SkillsManager: skillsMgr` to the `app.Deps` struct:

```go
	application := app.New(app.Deps{
		Loader:          loader,
		Installer:       installer,
		Reporter:        r,
		StateStore:      stateStore,
		DeployerFactory: deployerFactory,
		AIOrchestrator:  aiOrchestrator,
		ScriptRunner:    scriptRunner,
		SkillsManager:   skillsMgr,
		Version:         "0.1.0",
		OSName:          osName,
	})
```

- [ ] **Step 7: Build and verify the CLI works**

Run: `cd /Users/edocsss/aec/src/facet && go build -o /tmp/facet-test . && /tmp/facet-test ai --help && /tmp/facet-test ai skills --help && /tmp/facet-test ai skills check --help && /tmp/facet-test ai skills update --help && rm /tmp/facet-test`

Expected: Each prints usage text with the correct command descriptions.

- [ ] **Step 8: Run all tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./...`
Expected: All pass.

- [ ] **Step 9: Commit**

```bash
git add cmd/ai.go cmd/ai_skills.go cmd/ai_skills_check.go cmd/ai_skills_update.go cmd/root.go main.go
git commit -m "feat: add facet ai skills check/update commands"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `internal/docs/topics/ai.md`
- Modify: `internal/docs/topics/commands.md`
- Modify: `README.md`

- [ ] **Step 1: Update `internal/docs/topics/ai.md`**

Add a "Skill Source Formats" subsection after the existing Skills section, and a "Skills Management" subsection after that. The full Skills section should read:

```markdown
## Skills

Install skills from a source, optionally scoped to specific agents:

```yaml
ai:
  agents:
    - claude-code
    - cursor
  skills:
    - source: "@anthropic/claude-code-skills"
      skills:
        - code-review
        - testing
    - source: "@my-org/custom-skills"
      skills:
        - deploy-helper
      agents:
        - claude-code
```

Each skill entry has:

- `source`: package or path passed to the skills installer (see formats below)
- `skills`: list of skill names from that source
- `agents`: optional list limiting installation to specific agents

### Skill Source Formats

The `source` field supports any format accepted by the skills CLI:

| Format | Example |
|--------|---------|
| GitHub shorthand | `owner/repo` |
| HTTPS URL | `https://github.com/org/repo` |
| SSH URL | `git@github.com:org/private-repo.git` |
| Local path | `./my-local-skills` or `/absolute/path` |

Private repositories work via system-level git authentication. Ensure your SSH
keys are loaded (`ssh-agent`) or your git credentials are configured
(`gh auth login`) before running `facet apply`.

### Skills Management

Check for available skill updates:

```bash
facet ai skills check
```

Update all installed skills to their latest versions:

```bash
facet ai skills update
```

These commands pass through to the underlying skills CLI (`npx skills`) and
operate on all globally installed skills, not just those managed by facet.
```

- [ ] **Step 2: Update `internal/docs/topics/commands.md`**

Add a `## facet ai skills check` and `## facet ai skills update` section before the Exit Codes section:

```markdown
## `facet ai skills check`

Check for available skill updates.

```bash
facet ai skills check
```

Runs `npx skills check` and streams the output. Shows which installed skills
have newer versions available.

## `facet ai skills update`

Update all installed skills to their latest versions.

```bash
facet ai skills update
```

Runs `npx skills update` and streams the output. Re-installs any skills that
have updates available.
```

- [ ] **Step 3: Update `README.md`**

Add an "AI Configuration" section after the "Package Installation" section and before the "Commands" section:

```markdown
## AI Configuration

facet configures AI coding agents — permissions, skills, and MCP servers.

```yaml
# base.yaml
ai:
  agents: [claude-code, cursor, codex]
  permissions:
    claude-code:
      allow: [Read, Edit, Bash]
  skills:
    - source: "owner/repo"
      skills: [code-review]
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
```

Skills support multiple source formats including GitHub repos, SSH URLs for private repos, and local paths. See `facet docs ai` for details.

Manage installed skills with:

```sh
facet ai skills check    # check for available updates
facet ai skills update   # update all skills to latest
```
```

- [ ] **Step 4: Commit**

```bash
git add internal/docs/topics/ai.md internal/docs/topics/commands.md README.md
git commit -m "docs: document skill source formats and ai skills commands"
```

---

### Task 6: Add E2E tests

**Files:**
- Create: `e2e/suites/13-ai-skills-commands.sh`

- [ ] **Step 1: Create E2E test suite**

Create `e2e/suites/13-ai-skills-commands.sh`:

```bash
#!/bin/bash
# e2e/suites/13-ai-skills-commands.sh — AI skills check/update commands
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Test 1: facet ai skills check — runs npx skills check
facet -c "$HOME/dotfiles" -s "$HOME/.facet" ai skills check
assert_file_contains "$HOME/.mock-ai" "npx skills check"
echo "  facet ai skills check invoked npx skills check"

# Test 2: facet ai skills update — runs npx skills update
: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" ai skills update
assert_file_contains "$HOME/.mock-ai" "npx skills update"
echo "  facet ai skills update invoked npx skills update"

# Test 3: help output for parent commands
facet ai --help > "$HOME/.help-output" 2>&1
assert_file_contains "$HOME/.help-output" "skills"
echo "  facet ai --help shows skills subcommand"

facet ai skills --help > "$HOME/.help-output" 2>&1
assert_file_contains "$HOME/.help-output" "check"
assert_file_contains "$HOME/.help-output" "update"
echo "  facet ai skills --help shows check and update subcommands"
```

- [ ] **Step 2: Make the test executable**

Run: `chmod +x /Users/edocsss/aec/src/facet/e2e/suites/13-ai-skills-commands.sh`

- [ ] **Step 3: Verify the mock npx handles skills check/update**

The existing mock npx in `e2e/fixtures/mock-tools.sh` already logs `npx skills *` commands to `$HOME/.mock-ai`. The `skills check` and `skills update` invocations use `RunInteractive` which connects stdout/stderr directly, but the mock npx logs via `echo "npx $*" >> "$MOCK_AI_LOG"` which writes to the log file regardless. This works because the mock binary writes to the log file independently of how its stdout is connected.

- [ ] **Step 4: Run E2E tests natively**

Run: `cd /Users/edocsss/aec/src/facet/e2e && bash harness.sh suites/13-ai-skills-commands.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add e2e/suites/13-ai-skills-commands.sh
git commit -m "test: add E2E tests for facet ai skills check/update"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full unit test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./...`
Expected: All pass.

- [ ] **Step 2: Run full E2E test suite (native + Docker)**

Run: `cd /Users/edocsss/aec/src/facet && make pre-commit`
Expected: All pass.

- [ ] **Step 3: Verify CLI end-to-end manually**

Run: `cd /Users/edocsss/aec/src/facet && go build -o /tmp/facet-test . && /tmp/facet-test ai skills check && rm /tmp/facet-test`
Expected: `npx skills check` output streams to terminal (or error if npx/skills not installed, which is fine).
