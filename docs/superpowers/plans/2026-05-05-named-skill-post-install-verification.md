# Named Skill Post-Install Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After installing named skills, verify them against the skill lock and warn (non-fatally) for any that weren't actually installed, recording only confirmed skills in state.

**Architecture:** Add an `else` branch to `applySkills` in `orchestrator.go` — after a successful named-skill `Install()` call, call `InstalledForSource()` and cross-check requested names against the lock. Warn for missing, record only confirmed. This mirrors the existing "all" path exactly. No interface changes needed.

**Tech Stack:** Go 1.21+, `internal/ai` package, bash E2E suites

---

## File Map

| File | Change |
|---|---|
| `internal/ai/orchestrator.go` | Add post-install lock verification for named skill groups |
| `internal/ai/orchestrator_test.go` | Add `capturingReporter`, `errInstalledForSourceSkillsMgr`, and 4 new tests |
| `e2e/suites/10-ai-config.sh` | Pre-populate skill lock so named-skill assertions still pass |
| `e2e/suites/17-named-skill-verification.sh` | New suite: confirmed skill vs ghost skill |
| `internal/docs/topics/ai.md` | Document lock verification behaviour |
| `docs/architecture/v1-design-spec.md` | Update section 11 to describe verification |

---

## Task 1: Write failing unit tests

**Files:**
- Modify: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Add `strings` to the import block**

Open `internal/ai/orchestrator_test.go`. The existing imports are:

```go
import (
	"errors"
	"fmt"
	"sort"
	"testing"
)
```

Replace with:

```go
import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Add `capturingReporter` after the existing `mockReporter` struct (around line 97)**

Insert after the closing `}` of `mockReporter`:

```go
type capturingReporter struct {
	warnings  []string
	successes []string
}

func (c *capturingReporter) Success(msg string)       { c.successes = append(c.successes, msg) }
func (c *capturingReporter) Warning(msg string)       { c.warnings = append(c.warnings, msg) }
func (c *capturingReporter) Error(_ string)           {}
func (c *capturingReporter) Header(_ string)          {}
func (c *capturingReporter) PrintLine(_ string)       {}
func (c *capturingReporter) Dim(text string) string   { return text }
```

- [ ] **Step 3: Add `errInstalledForSourceSkillsMgr` after `capturingReporter`**

```go
// errInstalledForSourceSkillsMgr always succeeds Install but returns an error
// from InstalledForSource, simulating an unreadable skill lock.
type errInstalledForSourceSkillsMgr struct {
	installed []struct {
		source string
		skills []string
		agents []string
	}
	installedForSourceErr error
}

func (m *errInstalledForSourceSkillsMgr) Install(source string, skills []string, agents []string) error {
	m.installed = append(m.installed, struct {
		source string
		skills []string
		agents []string
	}{source: source, skills: skills, agents: agents})
	return nil
}
func (m *errInstalledForSourceSkillsMgr) Remove(_ []string, _ []string) error { return nil }
func (m *errInstalledForSourceSkillsMgr) InstalledForSource(_ string) ([]string, error) {
	return nil, m.installedForSourceErr
}
func (m *errInstalledForSourceSkillsMgr) Check() error  { return nil }
func (m *errInstalledForSourceSkillsMgr) Update() error { return nil }
```

- [ ] **Step 4: Append the 4 new test functions at the bottom of `orchestrator_test.go`**

```go
func TestOrchestrator_Apply_NamedSkillVerification_AllConfirmed(t *testing.T) {
	rep := &capturingReporter{}
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		rep,
	)
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-a"},
				{Source: "@org/skills", Name: "skill-b"},
			},
		},
	}
	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 2 {
		t.Fatalf("expected 2 skills in state, got %+v", state.Skills)
	}
	names := []string{state.Skills[0].Name, state.Skills[1].Name}
	sort.Strings(names)
	if names[0] != "skill-a" || names[1] != "skill-b" {
		t.Fatalf("expected skill-a and skill-b in state, got %v", names)
	}
	if len(rep.warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", rep.warnings)
	}
}

func TestOrchestrator_Apply_NamedSkillVerification_PartialMiss(t *testing.T) {
	rep := &capturingReporter{}
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		rep,
	)
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-a"},
				{Source: "@org/skills", Name: "skill-b"},
				{Source: "@org/skills", Name: "skill-c"},
			},
		},
	}
	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 2 {
		t.Fatalf("expected 2 skills in state (skill-c missing), got %+v", state.Skills)
	}
	names := []string{state.Skills[0].Name, state.Skills[1].Name}
	sort.Strings(names)
	if names[0] != "skill-a" || names[1] != "skill-b" {
		t.Fatalf("expected skill-a and skill-b in state, got %v", names)
	}
	if len(rep.warnings) == 0 {
		t.Fatal("expected a warning for missing skill-c, got none")
	}
	found := false
	for _, w := range rep.warnings {
		if strings.Contains(w, "skill-c") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning mentioning skill-c, got: %v", rep.warnings)
	}
}

func TestOrchestrator_Apply_NamedSkillVerification_TotalMiss(t *testing.T) {
	rep := &capturingReporter{}
	skillsMgr := &mockSkillsMgr{} // no sourceSkills — InstalledForSource returns empty
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		rep,
	)
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-a"},
				{Source: "@org/skills", Name: "skill-b"},
			},
		},
	}
	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 0 {
		t.Fatalf("expected no skills in state (total miss), got %+v", state.Skills)
	}
	if len(rep.warnings) == 0 {
		t.Fatal("expected warning for missing skills, got none")
	}
	found := false
	for _, w := range rep.warnings {
		if strings.Contains(w, "skill-a") && strings.Contains(w, "skill-b") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning mentioning skill-a and skill-b, got: %v", rep.warnings)
	}
}

func TestOrchestrator_Apply_NamedSkillVerification_LockReadError(t *testing.T) {
	rep := &capturingReporter{}
	skillsMgr := &errInstalledForSourceSkillsMgr{
		installedForSourceErr: fmt.Errorf("lock file unreadable"),
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		rep,
	)
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-a"},
				{Source: "@org/skills", Name: "skill-b"},
			},
		},
	}
	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 2 {
		t.Fatalf("expected fallback to record both skills, got %+v", state.Skills)
	}
	names := []string{state.Skills[0].Name, state.Skills[1].Name}
	sort.Strings(names)
	if names[0] != "skill-a" || names[1] != "skill-b" {
		t.Fatalf("expected skill-a and skill-b in state (fallback), got %v", names)
	}
	if len(rep.warnings) == 0 {
		t.Fatal("expected warning for lock read error, got none")
	}
	found := false
	for _, w := range rep.warnings {
		if strings.Contains(w, "could not verify") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning about lock verification failure, got: %v", rep.warnings)
	}
}
```

- [ ] **Step 5: Run the new tests to verify RED**

```bash
cd /Users/bytedance/aec/src/github/facet
go test ./internal/ai/... -run "TestOrchestrator_Apply_NamedSkillVerification" -v
```

Expected: `AllConfirmed` PASSES (the happy path already works), `PartialMiss` FAILS with "expected 2 skills in state (skill-c missing), got 3", `TotalMiss` FAILS with "expected no skills in state (total miss), got 2", `LockReadError` FAILS with "expected warning about lock verification failure".

- [ ] **Step 6: Commit the failing tests**

```bash
git add internal/ai/orchestrator_test.go
git commit -m "test: add failing tests for named skill post-install verification"
```

---

## Task 2: Implement post-install verification in `orchestrator.go`

**Files:**
- Modify: `internal/ai/orchestrator.go:288-315`

- [ ] **Step 1: Replace the recording block in `applySkills`**

Find this block (lines 288–315):

```go
		recordedSkills := skills
		if allGroups[gk] {
			resolvedSkills, err := o.skillsManager.InstalledForSource(gk.source)
			if err != nil {
				o.reporter.Warning(fmt.Sprintf("installed all skills from %q, but failed to resolve concrete skill names: %v", gk.source, err))
				recordedSkills = nil
			} else if len(resolvedSkills) == 0 {
				o.reporter.Warning(fmt.Sprintf("installed all skills from %q, but could not resolve concrete skill names from the skill lock; future cleanup will be skipped until the source is re-applied with a readable lock", gk.source))
				recordedSkills = nil
			} else {
				recordedSkills = resolvedSkills
			}
		}

		// Record each managed skill in state.
		for _, skillName := range recordedSkills {
			state.Skills = append(state.Skills, SkillState{
				Source: gk.source,
				Name:   skillName,
				Agents: agents,
			})
		}
		if allGroups[gk] {
			o.reporter.Success(fmt.Sprintf("installed all skills from %s", gk.source))
		} else {
			sort.Strings(skills)
			o.reporter.Success(fmt.Sprintf("installed skills %v from %s", skills, gk.source))
		}
```

Replace with:

```go
		recordedSkills := skills
		if allGroups[gk] {
			resolvedSkills, err := o.skillsManager.InstalledForSource(gk.source)
			if err != nil {
				o.reporter.Warning(fmt.Sprintf("installed all skills from %q, but failed to resolve concrete skill names: %v", gk.source, err))
				recordedSkills = nil
			} else if len(resolvedSkills) == 0 {
				o.reporter.Warning(fmt.Sprintf("installed all skills from %q, but could not resolve concrete skill names from the skill lock; future cleanup will be skipped until the source is re-applied with a readable lock", gk.source))
				recordedSkills = nil
			} else {
				recordedSkills = resolvedSkills
			}
		} else {
			verified, err := o.skillsManager.InstalledForSource(gk.source)
			if err != nil {
				o.reporter.Warning(fmt.Sprintf("installed skills from %q but could not verify via skill lock: %v — recording requested names", gk.source, err))
			} else {
				verifiedSet := make(map[string]struct{}, len(verified))
				for _, s := range verified {
					verifiedSet[s] = struct{}{}
				}
				var confirmed, missing []string
				for _, s := range skills {
					if _, ok := verifiedSet[s]; ok {
						confirmed = append(confirmed, s)
					} else {
						missing = append(missing, s)
					}
				}
				if len(missing) > 0 {
					o.reporter.Warning(fmt.Sprintf("skills %v from %q were not found in the skill lock after install — they may not exist in the source", missing, gk.source))
				}
				recordedSkills = confirmed
			}
		}

		// Record each managed skill in state.
		for _, skillName := range recordedSkills {
			state.Skills = append(state.Skills, SkillState{
				Source: gk.source,
				Name:   skillName,
				Agents: agents,
			})
		}
		if allGroups[gk] {
			o.reporter.Success(fmt.Sprintf("installed all skills from %s", gk.source))
		} else if len(recordedSkills) > 0 {
			sort.Strings(recordedSkills)
			o.reporter.Success(fmt.Sprintf("installed skills %v from %s", recordedSkills, gk.source))
		}
```

- [ ] **Step 2: Run unit tests to verify GREEN**

```bash
cd /Users/bytedance/aec/src/github/facet
go test ./internal/ai/... -v
```

Expected: all tests pass, including the 4 new ones.

- [ ] **Step 3: Commit**

```bash
git add internal/ai/orchestrator.go
git commit -m "feat: verify named skills against skill lock after install"
```

---

## Task 3: Fix `10-ai-config.sh` (broken by Task 2)

**Context:** The existing `10-ai-config.sh` installs `frontend-design` as a named skill. After Task 2's code change, `InstalledForSource` is now called post-install. The mock `npx` never writes to the skill lock, so the lock is empty → `frontend-design` won't be recorded in state → Tests 3 and 4 (which assert orphan removal of `frontend-design`) will break.

**Fix:** Pre-populate `~/.agents/.skill-lock.json` with `frontend-design` before Test 1.

**Files:**
- Modify: `e2e/suites/10-ai-config.sh`

- [ ] **Step 1: Insert skill lock setup after the `bash "$FIXTURE_DIR/setup-ai.sh"` line**

Find these two lines near the top of `10-ai-config.sh`:

```bash
setup_basic
bash "$FIXTURE_DIR/setup-ai.sh"
```

Replace with:

```bash
setup_basic
bash "$FIXTURE_DIR/setup-ai.sh"

mkdir -p "$HOME/.agents"
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {
    "frontend-design": {"source": "@vercel-labs/agent-skills", "sourceUrl": "https://github.com/vercel-labs/agent-skills.git"}
  }
}
JSON
```

- [ ] **Step 2: Run the E2E suite to verify it passes**

```bash
cd /Users/bytedance/aec/src/github/facet
go test -v -tags e2e ./e2e/... -run TestE2E_Native 2>&1 | grep -A5 "10-ai-config"
```

Expected: `✓ PASS`

- [ ] **Step 3: Commit**

```bash
git add e2e/suites/10-ai-config.sh
git commit -m "fix(e2e): pre-populate skill lock in 10-ai-config to match new verification"
```

---

## Task 4: Add `17-named-skill-verification.sh` E2E suite

**Files:**
- Create: `e2e/suites/17-named-skill-verification.sh`

**Context:** The harness auto-discovers suite files matching `[0-9]*.sh` — no registration needed.

- [ ] **Step 1: Create the suite file**

```bash
cat > /Users/bytedance/aec/src/github/facet/e2e/suites/17-named-skill-verification.sh << 'EOF'
#!/bin/bash
# e2e/suites/17-named-skill-verification.sh — named skill post-install verification
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Set up a minimal base.yaml (no existing AI config from setup-ai.sh needed)
mkdir -p "$HOME/dotfiles/profiles"
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
YAML

cat > "$HOME/dotfiles/profiles/work.yaml" << 'YAML'
extends: base
YAML

mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.agents"

# Test 1: Named skill that IS in the lock — recorded in state, no warning
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {
    "real-skill": {"source": "@org/skills"}
  }
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@org/skills"
      skills: [real-skill]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'real-skill'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].source' '@org/skills'
assert_file_not_contains "$HOME/.apply-out" "not found in the skill lock"
echo "  confirmed skill: recorded in state, no warning"

# Test 2: Named skill NOT in the lock — absent from state, warning in output
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {}
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@org/skills"
      skills: [ghost-skill]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_json_field "$HOME/.facet/.state.json" '.ai.skills' 'null'
assert_file_contains "$HOME/.apply-out" "not found in the skill lock"
echo "  ghost skill: absent from state, warning emitted"
EOF
chmod +x /Users/bytedance/aec/src/github/facet/e2e/suites/17-named-skill-verification.sh
```

- [ ] **Step 2: Run all native E2E tests**

```bash
cd /Users/bytedance/aec/src/github/facet
go test -v -tags e2e ./e2e/... -run TestE2E_Native 2>&1 | tail -20
```

Expected: all suites pass including `17-named-skill-verification`.

- [ ] **Step 3: Commit**

```bash
git add e2e/suites/17-named-skill-verification.sh
git commit -m "test(e2e): add named skill post-install verification suite"
```

---

## Task 5: Update documentation

**Files:**
- Modify: `internal/docs/topics/ai.md`
- Modify: `docs/architecture/v1-design-spec.md`
- Check (no change expected): `README.md`

- [ ] **Step 1: Update `internal/docs/topics/ai.md`**

Find this paragraph (around line 73):

```markdown
facet reconciles skills on every `facet apply`. If a previously managed source is
removed or narrowed to fewer skills, facet removes the no-longer-declared skills
for the affected agents before writing the new state. This includes entries that
were previously installed as "all skills from this source."
```

Replace with:

```markdown
facet reconciles skills on every `facet apply`. If a previously managed source is
removed or narrowed to fewer skills, facet removes the no-longer-declared skills
for the affected agents before writing the new state. This includes entries that
were previously installed as "all skills from this source."

After installing named skills, facet verifies each one against the skill lock
(`~/.agents/.skill-lock.json`). Skills that are absent from the lock after
install are not recorded in state and trigger a warning — they may not exist in
the source. If the lock file is unreadable, facet records the requested names as
a fallback and warns.
```

- [ ] **Step 2: Update `docs/architecture/v1-design-spec.md`**

Find this paragraph in section 11 (around line 485):

```markdown
AI skill reconciliation is stateful: when a previously managed source is removed
or narrowed on a later apply, facet removes only the no-longer-declared skills
for the affected agents before recording the new state.
```

Replace with:

```markdown
AI skill reconciliation is stateful: when a previously managed source is removed
or narrowed on a later apply, facet removes only the no-longer-declared skills
for the affected agents before recording the new state.

After installing named skills, facet post-verifies each requested name against
the skill lock (`~/.agents/.skill-lock.json`). Only names confirmed present in
the lock are recorded in state. Missing names produce a non-fatal warning and are
excluded from state (preventing phantom orphan-removal entries on future applies).
If the lock is unreadable, facet falls back to recording all requested names and
warns. The "all skills" path already followed this pattern; named installs now
behave consistently.
```

- [ ] **Step 3: Verify README.md needs no changes**

```bash
grep -n "skill\|Skill" /Users/bytedance/aec/src/github/facet/README.md | head -20
```

Expected: no detail about named-skill install behaviour that needs updating.

- [ ] **Step 4: Commit**

```bash
git add internal/docs/topics/ai.md docs/architecture/v1-design-spec.md
git commit -m "docs: document named skill post-install lock verification"
```

---

## Task 6: Final verification and wrap-up

- [ ] **Step 1: Run `make pre-commit`**

```bash
cd /Users/bytedance/aec/src/github/facet
make pre-commit
```

Expected: all unit tests pass, native E2E passes, Docker Linux E2E passes. If Docker is not running, start it first.

- [ ] **Step 2: Verify the fix end-to-end manually (optional smoke test)**

The real scenario from the bug report: `wagent-task-dev` and `wagent-task-workspace` were claimed as installed but weren't. After this fix:

- `npx skills add ... --skill wagent-task-dev --skill wagent-task-workspace` exits 0
- `InstalledForSource` reads lock, finds neither name
- `⚠ skills [wagent-task-dev wagent-task-workspace] from "..." were not found in the skill lock after install — they may not exist in the source`
- Neither appears in `.state.json` under `ai.skills`
- Neither appears in the `✓` lines of the apply report
