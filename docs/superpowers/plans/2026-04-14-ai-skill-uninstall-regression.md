# AI Skill Uninstall Regression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `facet apply` so removing an AI skill entry from a profile also uninstalls the previously installed skill set, including the `skills: omitted` ("all skills from source") case.

**Architecture:** Keep the existing apply/unapply flow and state model, but stop treating `Name == ""` as an uninstall blind spot. Use the global `npx skills` lock file (`~/.agents/.skill-lock.json`) to resolve a source into concrete skill names, then remove only the names that are no longer desired for that source+agent pair.

**Tech Stack:** Go, JSON, YAML, bash E2E harness, `npx skills`

---

## File Map

- `internal/ai/interfaces.go`
  Adds the source-resolution capability needed by the orchestrator without leaking lock-file details into callers.
- `internal/ai/skills_manager.go`
  Teaches `NPXSkillsManager` how to resolve installed skill names for a source from the global skill lock file.
- `internal/ai/skills_manager_test.go`
  Covers the new source-resolution behavior and failure paths.
- `internal/ai/orchestrator.go`
  Fixes all-source orphan removal during same-profile reapply and full unapply.
- `internal/ai/orchestrator_test.go`
  Adds regression coverage for all-to-nothing, all-to-specific, and unapply of all-source entries.
- `main.go`
  Wires the new skills-manager dependency explicitly.
- `e2e/fixtures/mock-tools.sh`
  Optionally teaches the mock environment how to maintain a minimal `.skill-lock.json`, if that is the cleanest way to keep the E2E suite hermetic.
- `e2e/suites/14-all-skills-from-source.sh`
  Adds the end-to-end regression cases for removing an all-source skill entry from the same profile.
- `README.md`
  Documents that facet also cleans up orphaned skills when an all-source entry is removed.
- `internal/docs/topics/ai.md`
  Updates the user-facing AI docs embedded in the binary.
- `docs/architecture/v1-design-spec.md`
  Records the intended uninstall behavior in the authoritative design document.

### Task 1: Lock The Regression In Tests First

**Files:**
- Modify: `internal/ai/orchestrator_test.go`
- Modify: `e2e/suites/14-all-skills-from-source.sh`

- [ ] **Step 1: Write failing unit tests for all-source removals**

Add focused tests to `internal/ai/orchestrator_test.go` that express the desired behavior:

```go
func TestOrchestrator_Apply_AllToNothing_RemovesResolvedSourceSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}

	_, err := orch.Apply(EffectiveAIConfig{"claude-code": {}}, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}

	require.Len(t, skillsMgr.removed, 1)
	assert.ElementsMatch(t, []string{"skill-a", "skill-b"}, skillsMgr.removed[0].skills)
	assert.Equal(t, []string{"claude-code"}, skillsMgr.removed[0].agents)
}

func TestOrchestrator_Apply_AllToSpecific_RemovesOnlyDroppedSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: "skill-a"}},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}

	require.Len(t, skillsMgr.removed, 1)
	assert.Equal(t, []string{"skill-b"}, skillsMgr.removed[0].skills)
}

func TestOrchestrator_Unapply_AllSource_RemovesResolvedSourceSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}

	require.NoError(t, orch.Unapply(previousState))
	require.Len(t, skillsMgr.removed, 1)
	assert.ElementsMatch(t, []string{"skill-a", "skill-b"}, skillsMgr.removed[0].skills)
}
```

- [ ] **Step 2: Run the unit regression tests and confirm they fail for the right reason**

Run: `go test ./internal/ai -run 'TestOrchestrator_(Apply_AllToNothing|Apply_AllToSpecific|Unapply_AllSource)' -v`
Expected: FAIL because the current orchestrator skips `Name == ""` during orphan removal and unapply.

- [ ] **Step 3: Add a hermetic E2E regression for same-profile removal**

Extend `e2e/suites/14-all-skills-from-source.sh` with a same-profile all-to-nothing case that seeds a minimal global skill lock file and verifies the remove command:

```bash
mkdir -p "$HOME/.agents"
cat > "$HOME/.agents/.skill-lock.json" <<'JSON'
{
  "version": 3,
  "skills": {
    "skill-a": {"source": "@vercel-labs/agent-skills"},
    "skill-b": {"source": "@vercel-labs/agent-skills"},
    "other-skill": {"source": "@org/other-skills"}
  }
}
JSON

cat > "$HOME/dotfiles/base.yaml" <<'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
YAML

: > "$HOME/.mock-ai"
facet_apply work

cat > "$HOME/dotfiles/base.yaml" <<'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
YAML

: > "$HOME/.mock-ai"
facet_apply work
assert_file_contains "$HOME/.mock-ai" "npx skills remove skill-a skill-b -a claude-code -g -y"
assert_file_not_contains "$HOME/.mock-ai" "other-skill"
```

- [ ] **Step 4: Run the E2E suite and confirm the new regression check fails**

Run: `bash e2e/harness.sh e2e/suites/14-all-skills-from-source.sh`
Expected: FAIL because the current implementation never removes an all-source entry.

- [ ] **Step 5: Commit the red tests**

```bash
git add internal/ai/orchestrator_test.go e2e/suites/14-all-skills-from-source.sh
git commit -m "test: cover all-source skill uninstall regression"
```

---

### Task 2: Add Source-To-Skill Resolution In The Skills Manager

**Files:**
- Modify: `internal/ai/interfaces.go`
- Modify: `internal/ai/skills_manager.go`
- Modify: `internal/ai/skills_manager_test.go`
- Modify: `main.go`

- [ ] **Step 1: Extend the skills-manager interface with source resolution**

Change `internal/ai/interfaces.go` so the orchestrator can ask for installed skills by source:

```go
type SkillsManager interface {
	Install(source string, skills []string, agents []string) error
	Remove(skills []string, agents []string) error
	InstalledForSource(source string) ([]string, error)
	Check() error
	Update() error
}
```

- [ ] **Step 2: Write failing tests for lock-file lookup**

Add tests to `internal/ai/skills_manager_test.go` like:

```go
func TestNPXSkillsManager_InstalledForSource(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), ".skill-lock.json")
	require.NoError(t, os.WriteFile(lockPath, []byte(`{
		"version": 3,
		"skills": {
			"skill-a": {"source": "@org/skills"},
			"skill-b": {"source": "@org/skills"},
			"other-skill": {"source": "@org/other"}
		}
	}`), 0o644))

	mgr := NewNPXSkillsManager(&mockRunner{}, lockPath)

	got, err := mgr.InstalledForSource("@org/skills")
	require.NoError(t, err)
	assert.Equal(t, []string{"skill-a", "skill-b"}, got)
}

func TestNPXSkillsManager_InstalledForSource_MissingLockFile(t *testing.T) {
	mgr := NewNPXSkillsManager(&mockRunner{}, filepath.Join(t.TempDir(), "missing.json"))

	got, err := mgr.InstalledForSource("@org/skills")
	require.NoError(t, err)
	assert.Nil(t, got)
}
```

- [ ] **Step 3: Implement lock-file parsing in the manager**

Update `internal/ai/skills_manager.go` so `NPXSkillsManager` stores a lock-file path and resolves source memberships from JSON:

```go
type NPXSkillsManager struct {
	runner       CommandRunner
	skillLockPath string
	npxOnce      sync.Once
	npxError     error
}

func NewNPXSkillsManager(runner CommandRunner, skillLockPath string) *NPXSkillsManager {
	return &NPXSkillsManager{
		runner:        runner,
		skillLockPath: skillLockPath,
	}
}

func (m *NPXSkillsManager) InstalledForSource(source string) ([]string, error) {
	data, err := os.ReadFile(m.skillLockPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill lock: %w", err)
	}

	var lock struct {
		Skills map[string]struct {
			Source string `json:"source"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse skill lock: %w", err)
	}

	var skills []string
	for name, entry := range lock.Skills {
		if entry.Source == source {
			skills = append(skills, name)
		}
	}
	sort.Strings(skills)
	return skills, nil
}
```

- [ ] **Step 4: Wire the manager in `main.go` with the global lock-file path**

Update the constructor call in `main.go` to pass:

```go
skillsMgr := ai.NewNPXSkillsManager(
	aiRunner,
	filepath.Join(homeDir, ".agents", ".skill-lock.json"),
)
```

- [ ] **Step 5: Run the skills-manager tests**

Run: `go test ./internal/ai -run 'TestNPXSkillsManager_(InstalledForSource|Install|Remove)' -v`
Expected: PASS

- [ ] **Step 6: Commit the lock-file support**

```bash
git add internal/ai/interfaces.go internal/ai/skills_manager.go internal/ai/skills_manager_test.go main.go
git commit -m "feat: resolve installed skills by source from skill lock"
```

---

### Task 3: Fix Orchestrator Removal Logic For All-Source Entries

**Files:**
- Modify: `internal/ai/orchestrator.go`
- Modify: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Replace the “skip empty name” behavior with source-aware removal**

Refactor `internal/ai/orchestrator.go` so the all-source case resolves installed names and removes only the names that are no longer desired:

```go
func (o *Orchestrator) sourceSkillsToRemove(
	source string,
	agent string,
	currentSkills map[skillID]map[string]struct{},
	currentAllSources map[string]map[string]struct{},
) ([]string, error) {
	if allAgents, hasAll := currentAllSources[source]; hasAll {
		if _, covered := allAgents[agent]; covered {
			return nil, nil
		}
	}

	installed, err := o.skillsManager.InstalledForSource(source)
	if err != nil {
		return nil, err
	}

	var keep map[string]struct{}
	for id, agents := range currentSkills {
		if id.source != source || id.name == "" {
			continue
		}
		if _, ok := agents[agent]; ok {
			if keep == nil {
				keep = make(map[string]struct{})
			}
			keep[id.name] = struct{}{}
		}
	}

	var remove []string
	for _, name := range installed {
		if _, ok := keep[name]; !ok {
			remove = append(remove, name)
		}
	}
	return remove, nil
}
```

- [ ] **Step 2: Use that helper in both `Apply` and `Unapply`**

Update the two removal paths to handle `Name == ""` instead of skipping it:

```go
if prevSkill.Name == "" {
	remove, err := o.sourceSkillsToRemove(prevSkill.Source, agent, currentSkills, currentAllSources)
	if err != nil {
		o.reporter.Warning(fmt.Sprintf("failed to resolve skills for source %q: %v", prevSkill.Source, err))
		continue
	}
	if len(remove) == 0 {
		continue
	}
	if err := o.skillsManager.Remove(remove, []string{agent}); err != nil {
		o.reporter.Warning(fmt.Sprintf("failed to remove orphan skills from %q for %q: %v", prevSkill.Source, agent, err))
	}
	continue
}
```

For `Unapply`, resolve all names for the source and remove them per agent rather than skipping the entry.

- [ ] **Step 3: Keep the behavior non-destructive outside the source**

Make sure the implementation preserves these invariants:

```go
// Same source, still all -> remove nothing.
// Same source, all -> specific -> remove only names not in current config.
// Same source, all -> nothing -> remove every name in the lock file for that source.
// Other sources remain untouched.
```

- [ ] **Step 4: Run the orchestrator regression tests**

Run: `go test ./internal/ai -run 'TestOrchestrator_(Apply_AllToNothing|Apply_AllToSpecific|Unapply_AllSource|Apply_SpecificToAllTransition)' -v`
Expected: PASS

- [ ] **Step 5: Commit the orchestrator fix**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "fix: uninstall all-source skills when removed from config"
```

---

### Task 4: Make The E2E Regression Hermetic And Green

**Files:**
- Modify: `e2e/fixtures/mock-tools.sh` (only if the suite needs richer lock-file simulation)
- Modify: `e2e/suites/14-all-skills-from-source.sh`

- [ ] **Step 1: Keep the E2E test self-contained**

If the suite can seed the lock file directly, prefer that over expanding the mock `npx` implementation:

```bash
mkdir -p "$HOME/.agents"
cat > "$HOME/.agents/.skill-lock.json" <<'JSON'
{
  "version": 3,
  "skills": {
    "skill-a": {"source": "@vercel-labs/agent-skills"},
    "skill-b": {"source": "@vercel-labs/agent-skills"},
    "other-skill": {"source": "@org/other-skills"}
  }
}
JSON
```

- [ ] **Step 2: Add an all-to-specific regression too**

Add a second E2E case proving we only remove dropped skills:

```bash
cat > "$HOME/dotfiles/base.yaml" <<'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      skills: [skill-a]
YAML

: > "$HOME/.mock-ai"
facet_apply work
assert_file_contains "$HOME/.mock-ai" "npx skills remove skill-b -a claude-code -g -y"
assert_file_not_contains "$HOME/.mock-ai" "other-skill"
```

- [ ] **Step 3: Run the targeted E2E suite**

Run: `bash e2e/harness.sh e2e/suites/14-all-skills-from-source.sh`
Expected: PASS

- [ ] **Step 4: Commit the E2E regression coverage**

```bash
git add e2e/fixtures/mock-tools.sh e2e/suites/14-all-skills-from-source.sh
git commit -m "test: cover all-source skill cleanup end to end"
```

---

### Task 5: Update User-Facing Docs And The Design Spec

**Files:**
- Modify: `README.md`
- Modify: `internal/docs/topics/ai.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Update the README behavior summary**

Add a short note near the AI skills section:

```md
When a skill entry is removed from the effective profile, facet also removes the
previously installed skill links for that source/agent scope during the next
`facet apply`, including entries that were installed with `--all`.
```

- [ ] **Step 2: Update the embedded docs**

Add the same behavior to `internal/docs/topics/ai.md` in the Skills section:

```md
facet tracks installed skills by source and removes orphaned skills on later
applies. This includes "all skills from source" entries when the source is
removed or narrowed to a smaller set of skills.
```

- [ ] **Step 3: Update the authoritative design spec**

Add a short rule in `docs/architecture/v1-design-spec.md`:

```md
AI skills are reconciled during apply. If a previously managed skill source is
removed or narrowed, facet removes only the no-longer-declared skills for the
affected agents before recording the new state.
```

- [ ] **Step 4: Review the doc changes for consistency**

Run: `rg -n "all skills|orphaned skills|facet apply" README.md internal/docs/topics/ai.md docs/architecture/v1-design-spec.md`
Expected: the three files all describe the same cleanup behavior.

- [ ] **Step 5: Commit the docs**

```bash
git add README.md internal/docs/topics/ai.md docs/architecture/v1-design-spec.md
git commit -m "docs: describe AI skill cleanup for removed sources"
```

---

### Task 6: Verify End To End Before Declaring Success

**Files:**
- Verify only

- [ ] **Step 1: Run the focused unit tests**

Run:

```bash
go test ./internal/ai -run 'TestNPXSkillsManager|TestOrchestrator_' -v
```

Expected: PASS

- [ ] **Step 2: Run the focused E2E suites**

Run:

```bash
bash e2e/harness.sh e2e/suites/10-ai-config.sh e2e/suites/14-all-skills-from-source.sh
```

Expected: PASS

- [ ] **Step 3: Run the full required gate**

Run:

```bash
make pre-commit
```

Expected: PASS, including native and Docker E2E coverage.

- [ ] **Step 4: Prepare the branch for review**

Run:

```bash
git status --short
git log --oneline --decorate -5
```

Expected: clean working tree, commits grouped by tests / implementation / docs.

