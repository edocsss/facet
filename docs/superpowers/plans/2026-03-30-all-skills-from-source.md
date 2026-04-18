# All Skills From Source — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow YAML config to omit the `skills` list in a `SkillEntry` to mean "install all skills from this source" via `--all` flag.

**Architecture:** When `SkillEntry.Skills` is nil/empty, the system propagates an "all" sentinel (empty string `""` for skill name) through resolve → orchestrate → install. `NPXSkillsManager.Install()` passes `--all` instead of individual `--skill` flags. The merge layer treats "all" as an atomic replacement for a source.

**Tech Stack:** Go, YAML, bash (E2E tests)

---

### Task 1: NPXSkillsManager.Install — pass `--all` when skills is empty

**Files:**
- Modify: `internal/ai/skills_manager.go:31-50`
- Test: `internal/ai/skills_manager_test.go`

- [ ] **Step 1: Write the failing test for `--all` flag**

Add to `internal/ai/skills_manager_test.go`:

```go
func TestNPXSkillsManager_Install_AllSkills(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Install("@my-org/skills", nil, []string{"claude-code", "cursor"})
	if err != nil {
		t.Fatalf("Install returned unexpected error: %v", err)
	}

	if len(runner.commands) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(runner.commands), runner.commands)
	}

	want := "npx skills add @my-org/skills --all -a claude-code -a cursor -y"
	if runner.commands[1] != want {
		t.Errorf("unexpected install command:\n  got:  %q\n  want: %q", runner.commands[1], want)
	}
}

func TestNPXSkillsManager_Install_AllSkillsEmptySlice(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Install("@my-org/skills", []string{}, []string{"claude-code"})
	if err != nil {
		t.Fatalf("Install returned unexpected error: %v", err)
	}

	want := "npx skills add @my-org/skills --all -a claude-code -y"
	if runner.commands[1] != want {
		t.Errorf("unexpected install command:\n  got:  %q\n  want: %q", runner.commands[1], want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestNPXSkillsManager_Install_AllSkills" -v`
Expected: FAIL — currently produces `npx skills add @my-org/skills -y` (no `--all` flag)

- [ ] **Step 3: Implement the `--all` branch in Install**

In `internal/ai/skills_manager.go`, replace the `Install` method body (lines 31–50) with:

```go
// Install runs: npx skills add <source> --skill <s1> --skill <s2> -a <a1> -a <a2> -y
// When skills is empty, passes --all instead of individual --skill flags.
func (m *NPXSkillsManager) Install(source string, skills []string, agents []string) error {
	if err := m.checkNPX(); err != nil {
		return err
	}

	var parts []string
	parts = append(parts, "npx", "skills", "add", source)
	if len(skills) == 0 {
		parts = append(parts, "--all")
	} else {
		for _, s := range skills {
			parts = append(parts, "--skill", s)
		}
	}
	for _, a := range agents {
		parts = append(parts, "-a", a)
	}
	parts = append(parts, "-y")

	if err := m.runner.Run(parts[0], parts[1:]...); err != nil {
		return fmt.Errorf("skills install: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run all skills_manager tests to verify**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestNPXSkillsManager" -v`
Expected: ALL PASS (new tests pass, existing tests still pass since they provide non-empty skills)

- [ ] **Step 5: Commit**

```bash
git add internal/ai/skills_manager.go internal/ai/skills_manager_test.go
git commit -m "feat: pass --all flag to npx skills add when skills list is empty"
```

---

### Task 2: mergeSkills — handle "all" entries

**Files:**
- Modify: `internal/profile/ai_merger.go:47-128`
- Test: `internal/profile/ai_merger_test.go`

- [ ] **Step 1: Write failing tests for "all" merge scenarios**

Add to `internal/profile/ai_merger_test.go`:

```go
func TestMergeSkills_AllFromSource(t *testing.T) {
	// "All" entry (empty Skills) passes through as-is.
	base := []SkillEntry{
		{Source: "source-a"},
	}
	result := mergeSkills(base, nil)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Nil(t, result[0].Skills)
}

func TestMergeSkills_AllInBaseSpecificInOverlay(t *testing.T) {
	// Overlay with specific skills replaces base "all" for same source.
	base := []SkillEntry{
		{Source: "source-a"},
	}
	overlay := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1"}, Agents: []string{"cursor"}},
	}

	result := mergeSkills(base, overlay)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Equal(t, []string{"skill-1"}, result[0].Skills)
	assert.Equal(t, []string{"cursor"}, result[0].Agents)
}

func TestMergeSkills_SpecificInBaseAllInOverlay(t *testing.T) {
	// "All" in overlay replaces all individual entries from base for same source.
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1", "skill-2"}, Agents: []string{"claude"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a"},
	}

	result := mergeSkills(base, overlay)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Nil(t, result[0].Skills)
	assert.Nil(t, result[0].Agents)
}

func TestMergeSkills_AllInBaseAllInOverlayDifferentAgents(t *testing.T) {
	// "All" in overlay replaces "all" in base for same source.
	base := []SkillEntry{
		{Source: "source-a", Agents: []string{"claude"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a", Agents: []string{"cursor"}},
	}

	result := mergeSkills(base, overlay)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Nil(t, result[0].Skills)
	assert.Equal(t, []string{"cursor"}, result[0].Agents)
}

func TestMergeSkills_AllForSourceADoesNotAffectSourceB(t *testing.T) {
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1"}},
		{Source: "source-b", Skills: []string{"skill-2"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a"},
	}

	result := mergeSkills(base, overlay)

	require.Len(t, result, 2)

	bySource := make(map[string]SkillEntry)
	for _, e := range result {
		bySource[e.Source] = e
	}

	assert.Nil(t, bySource["source-a"].Skills)
	assert.Equal(t, []string{"skill-2"}, bySource["source-b"].Skills)
}

func TestMergeSkills_ThreeLayer_BaseSpecific_OverlayAll_LocalSpecific(t *testing.T) {
	// Simulates base → profile → local merge.
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1", "skill-2"}},
	}
	profileLayer := []SkillEntry{
		{Source: "source-a"},
	}
	local := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-3"}, Agents: []string{"cursor"}},
	}

	merged := mergeSkills(base, profileLayer)
	result := mergeSkills(merged, local)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Equal(t, []string{"skill-3"}, result[0].Skills)
	assert.Equal(t, []string{"cursor"}, result[0].Agents)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestMergeSkills_All" -v`
Expected: FAIL — current mergeSkills ignores entries with empty Skills

- [ ] **Step 3: Implement "all" handling in mergeSkills**

Replace the `mergeSkills` function in `internal/profile/ai_merger.go` (lines 54–128) with:

```go
// mergeSkills unions two SkillEntry slices by (source, skill_name) tuple key.
// An entry with an empty Skills list means "all skills from this source" and is
// treated as an atomic unit: it replaces all individual tuples for that source
// from the base, and is itself replaced by specific entries from the overlay.
// Overlay tuples overwrite base tuples on conflict. Results are re-grouped by source.
// Both nil → nil.
func mergeSkills(base, overlay []SkillEntry) []SkillEntry {
	if base == nil && overlay == nil {
		return nil
	}

	// allSources tracks sources where the latest layer specified "all".
	// key: source name, value: agents for the "all" entry.
	type allEntry struct {
		agents []string
		order  int // insertion order
	}
	allSources := make(map[string]allEntry)

	// Track insertion order by tuple key for specific skills.
	type entry struct {
		key   string // "source\x00skillName"
		tuple skillTuple
	}

	seen := make(map[string]int) // key → index in ordered
	ordered := []entry{}
	orderCounter := 0

	addSpecific := func(source, skillName string, agents []string) {
		key := source + "\x00" + skillName
		t := skillTuple{source: source, skillName: skillName, agents: agents}
		if idx, exists := seen[key]; exists {
			ordered[idx].tuple = t
		} else {
			seen[key] = len(ordered)
			ordered = append(ordered, entry{key: key, tuple: t})
		}
	}

	addAll := func(source string, agents []string) {
		// Remove any individual tuples for this source.
		for key, idx := range seen {
			if ordered[idx].tuple.source == source {
				ordered[idx].key = "" // mark for removal
				delete(seen, key)
			}
		}
		allSources[source] = allEntry{agents: agents, order: orderCounter}
		orderCounter++
	}

	addLayer := func(entries []SkillEntry) {
		for _, e := range entries {
			if len(e.Skills) == 0 {
				addAll(e.Source, e.Agents)
			} else {
				// Specific skills replace any "all" for this source.
				delete(allSources, e.Source)
				for _, skill := range e.Skills {
					addSpecific(e.Source, skill, e.Agents)
				}
			}
		}
	}

	addLayer(base)
	addLayer(overlay)

	// Build result: first collect "all" entries, then grouped specific entries.
	// Use a combined ordering approach to maintain insertion order.

	// Collect "all" entries sorted by their insertion order.
	type allResult struct {
		source string
		agents []string
		order  int
	}
	var allResults []allResult
	for source, ae := range allSources {
		allResults = append(allResults, allResult{source: source, agents: ae.agents, order: ae.order})
	}

	// Re-group specific tuples by (source, agents), preserving insertion order.
	type groupEntry struct {
		source string
		skills []string
		agents []string
	}
	groupIdx := make(map[string]int) // "source\x00sortedAgents" → index in groups
	groups := []groupEntry{}

	for _, oe := range ordered {
		if oe.key == "" {
			continue // marked for removal
		}
		t := oe.tuple
		key := t.source + "\x00" + sortedAgentsKey(t.agents)
		if idx, exists := groupIdx[key]; exists {
			groups[idx].skills = append(groups[idx].skills, t.skillName)
		} else {
			groupIdx[key] = len(groups)
			groups = append(groups, groupEntry{
				source: t.source,
				skills: []string{t.skillName},
				agents: t.agents,
			})
		}
	}

	result := make([]SkillEntry, 0, len(allResults)+len(groups))
	for _, ar := range allResults {
		result = append(result, SkillEntry{
			Source: ar.source,
			Agents: ar.agents,
		})
	}
	for _, g := range groups {
		result = append(result, SkillEntry{
			Source: g.source,
			Skills: g.skills,
			Agents: g.agents,
		})
	}
	return result
}
```

- [ ] **Step 4: Run all ai_merger tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: ALL PASS (new tests pass, existing tests still pass)

- [ ] **Step 5: Commit**

```bash
git add internal/profile/ai_merger.go internal/profile/ai_merger_test.go
git commit -m "feat: handle 'all skills' entries in mergeSkills"
```

---

### Task 3: Resolve — pass through "all" entries

**Files:**
- Modify: `internal/ai/resolve.go:23-33`
- Test: `internal/ai/resolve_test.go`

- [ ] **Step 1: Write failing tests for "all" resolution**

Add to `internal/ai/resolve_test.go`:

```go
func TestResolve_AllSkillsEntry(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/skills"},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)

	claudeCfg := result["claude-code"]
	require.Len(t, claudeCfg.Skills, 1)
	assert.Equal(t, "github.com/example/skills", claudeCfg.Skills[0].Source)
	assert.Equal(t, "", claudeCfg.Skills[0].Name)

	cursorCfg := result["cursor"]
	require.Len(t, cursorCfg.Skills, 1)
	assert.Equal(t, "github.com/example/skills", cursorCfg.Skills[0].Source)
	assert.Equal(t, "", cursorCfg.Skills[0].Name)
}

func TestResolve_AllSkillsWithAgentScoping(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor", "codex"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/all-skills"},
			{Source: "github.com/example/claude-only", Agents: []string{"claude-code"}},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)

	claudeCfg := result["claude-code"]
	require.Len(t, claudeCfg.Skills, 2)

	cursorCfg := result["cursor"]
	require.Len(t, cursorCfg.Skills, 1)
	assert.Equal(t, "github.com/example/all-skills", cursorCfg.Skills[0].Source)

	codexCfg := result["codex"]
	require.Len(t, codexCfg.Skills, 1)
	assert.Equal(t, "github.com/example/all-skills", codexCfg.Skills[0].Source)
}

func TestResolve_MixedAllAndSpecificSkills(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/all-skills"},
			{Source: "github.com/example/specific", Skills: []string{"skill-1", "skill-2"}},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)

	claudeCfg := result["claude-code"]
	require.Len(t, claudeCfg.Skills, 3)

	assert.Equal(t, "github.com/example/all-skills", claudeCfg.Skills[0].Source)
	assert.Equal(t, "", claudeCfg.Skills[0].Name)

	assert.Equal(t, "github.com/example/specific", claudeCfg.Skills[1].Source)
	assert.Equal(t, "skill-1", claudeCfg.Skills[1].Name)

	assert.Equal(t, "github.com/example/specific", claudeCfg.Skills[2].Source)
	assert.Equal(t, "skill-2", claudeCfg.Skills[2].Name)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestResolve_AllSkills\|TestResolve_MixedAll" -v`
Expected: FAIL — current Resolve produces 0 skills for entries with empty Skills list

- [ ] **Step 3: Implement "all" pass-through in Resolve**

In `internal/ai/resolve.go`, replace the skills filtering block (lines 24–33) with:

```go
		// Step 2: filter skills and flatten each SkillEntry into ResolvedSkills.
		// An entry with empty Skills means "all from source" — emit a single
		// ResolvedSkill with an empty Name as a sentinel.
		var skills []ResolvedSkill
		for _, entry := range cfg.Skills {
			if agentIncluded(agent, entry.Agents) {
				if len(entry.Skills) == 0 {
					skills = append(skills, ResolvedSkill{
						Source: entry.Source,
						Name:   "",
					})
				} else {
					for _, skillName := range entry.Skills {
						skills = append(skills, ResolvedSkill{
							Source: entry.Source,
							Name:   skillName,
						})
					}
				}
			}
		}
```

- [ ] **Step 4: Run all resolve tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestResolve" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/resolve.go internal/ai/resolve_test.go
git commit -m "feat: resolve 'all skills' entries as empty-name sentinel"
```

---

### Task 4: Orchestrator — handle "all" entries in grouping, state, and orphan removal

**Files:**
- Modify: `internal/ai/orchestrator.go:169-239`
- Test: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Write failing tests for "all" orchestration**

Add to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_Apply_AllSkillsFromSource(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: ""}},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 1 {
		t.Fatalf("expected 1 install call, got %d: %+v", len(skillsMgr.installed), skillsMgr.installed)
	}
	inst := skillsMgr.installed[0]
	if inst.source != "@org/skills" {
		t.Errorf("expected source @org/skills, got %q", inst.source)
	}
	if inst.skills != nil {
		t.Errorf("expected nil skills (all), got %v", inst.skills)
	}
	if len(inst.agents) != 1 || inst.agents[0] != "claude-code" {
		t.Errorf("unexpected agents: %v", inst.agents)
	}
	if len(state.Skills) != 1 || state.Skills[0].Name != "" || state.Skills[0].Source != "@org/skills" {
		t.Fatalf("unexpected state: %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_MixedAllAndSpecificSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/all-skills", Name: ""},
				{Source: "@org/specific", Name: "skill-a"},
				{Source: "@org/specific", Name: "skill-b"},
			},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 2 {
		t.Fatalf("expected 2 install calls, got %d: %+v", len(skillsMgr.installed), skillsMgr.installed)
	}

	// Sort by source for deterministic comparison.
	sort.Slice(skillsMgr.installed, func(i, j int) bool {
		return skillsMgr.installed[i].source < skillsMgr.installed[j].source
	})

	allInst := skillsMgr.installed[0]
	if allInst.source != "@org/all-skills" || allInst.skills != nil {
		t.Errorf("expected all-skills with nil skills, got %+v", allInst)
	}

	specInst := skillsMgr.installed[1]
	sort.Strings(specInst.skills)
	if specInst.source != "@org/specific" || len(specInst.skills) != 2 || specInst.skills[0] != "skill-a" || specInst.skills[1] != "skill-b" {
		t.Errorf("expected specific skills, got %+v", specInst)
	}

	if len(state.Skills) != 3 {
		t.Fatalf("expected 3 state entries, got %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_SpecificToAllTransition_SkipsOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "skill-1", Agents: []string{"claude-code"}},
			{Source: "@org/skills", Name: "skill-2", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: ""}},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	// Should NOT remove skill-1 or skill-2 — "all" from same source covers them.
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals (all covers previous), got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_AllToSpecificTransition_SkipsOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
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
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-1"},
			},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	// Previous "all" entry (name="") should not trigger Remove — meaningless to remove "".
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals for all-to-specific transition, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_AllToNothing_SkipsOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
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
		"claude-code": {},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	// Previous "all" entry should not trigger Remove — we can't remove "all from source" by name.
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals for all-to-nothing transition, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Unapply_SkipsAllEntries(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
			{Source: "@org/other", Name: "skill-1", Agents: []string{"claude-code"}},
		},
	}

	if err := orch.Unapply(previousState); err != nil {
		t.Fatalf("Unapply returned unexpected error: %v", err)
	}
	// Should only remove the named skill, not the "all" entry.
	if len(skillsMgr.removed) != 1 {
		t.Fatalf("expected 1 removal, got %+v", skillsMgr.removed)
	}
	if skillsMgr.removed[0].skills[0] != "skill-1" {
		t.Errorf("expected skill-1 removal, got %+v", skillsMgr.removed[0])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestOrchestrator_Apply_AllSkills\|TestOrchestrator_Apply_MixedAll\|TestOrchestrator_Apply_SpecificToAll\|TestOrchestrator_Apply_AllToSpecific\|TestOrchestrator_Apply_AllToNothing\|TestOrchestrator_Unapply_SkipsAll" -v`
Expected: FAIL

- [ ] **Step 3: Implement "all" handling in applySkills**

Replace `applySkills` in `internal/ai/orchestrator.go` (lines 169–239) with:

```go
// applySkills installs current skills and removes orphans from previousState.
func (o *Orchestrator) applySkills(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	currentSkills := make(map[skillID]map[string]struct{})
	// Track which sources have an "all" entry per agent.
	currentAllSources := make(map[string]map[string]struct{}) // source → set of agents

	for agent, agentCfg := range config {
		for _, skill := range agentCfg.Skills {
			key := skillID{source: skill.Source, name: skill.Name}
			if _, exists := currentSkills[key]; !exists {
				currentSkills[key] = make(map[string]struct{})
			}
			currentSkills[key][agent] = struct{}{}

			if skill.Name == "" {
				if _, exists := currentAllSources[skill.Source]; !exists {
					currentAllSources[skill.Source] = make(map[string]struct{})
				}
				currentAllSources[skill.Source][agent] = struct{}{}
			}
		}
	}

	if previousState != nil {
		for _, prevSkill := range previousState.Skills {
			// Skip orphan removal for "all" entries — can't remove by empty name.
			if prevSkill.Name == "" {
				continue
			}
			currentAgents := currentSkills[skillID{source: prevSkill.Source, name: prevSkill.Name}]
			for _, agent := range prevSkill.Agents {
				if _, exists := currentAgents[agent]; exists {
					continue
				}
				// Skip removal if current config has "all" for this source+agent.
				if allAgents, hasAll := currentAllSources[prevSkill.Source]; hasAll {
					if _, covered := allAgents[agent]; covered {
						continue
					}
				}
				if err := o.skillsManager.Remove([]string{prevSkill.Name}, []string{agent}); err != nil {
					o.reporter.Warning(fmt.Sprintf("failed to remove orphan skill %q from %q: %v", prevSkill.Name, agent, err))
				} else {
					o.reporter.Success(fmt.Sprintf("removed orphan skill %q from %s", prevSkill.Name, agent))
				}
			}
		}
	}

	// Group skills by (source, sorted agents) for batched Install calls.
	groups := make(map[skillGroupKey][]string)
	// Track which groups are "all" (contain the empty-name sentinel).
	allGroups := make(map[skillGroupKey]bool)

	for id, agentSet := range currentSkills {
		agents := sortedSetKeys(agentSet)
		key := skillGroupKey{
			source: id.source,
			agents: strings.Join(agents, ","),
		}
		groups[key] = append(groups[key], id.name)
		if id.name == "" {
			allGroups[key] = true
		}
	}

	groupKeys := make([]skillGroupKey, 0, len(groups))
	for gk := range groups {
		groupKeys = append(groupKeys, gk)
	}
	sort.Slice(groupKeys, func(i, j int) bool {
		if groupKeys[i].source != groupKeys[j].source {
			return groupKeys[i].source < groupKeys[j].source
		}
		return groupKeys[i].agents < groupKeys[j].agents
	})

	for _, gk := range groupKeys {
		skills := groups[gk]
		agents := strings.Split(gk.agents, ",")

		var installSkills []string
		if allGroups[gk] {
			// "All" group — pass nil to trigger --all flag.
			installSkills = nil
		} else {
			sort.Strings(skills)
			installSkills = skills
		}

		if err := o.skillsManager.Install(gk.source, installSkills, agents); err != nil {
			o.reporter.Warning(fmt.Sprintf("failed to install skills from %q: %v", gk.source, err))
			continue
		}

		// Record each skill in state.
		for _, skillName := range skills {
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
	}
}
```

- [ ] **Step 4: Update Unapply to skip "all" entries**

In `internal/ai/orchestrator.go`, in the `Unapply` method, replace the skills removal block (lines 82–86) with:

```go
	// 2. Remove skills (skip "all" entries — can't remove by empty name).
	for _, skill := range previousState.Skills {
		if skill.Name == "" {
			continue
		}
		if err := o.skillsManager.Remove([]string{skill.Name}, skill.Agents); err != nil {
			o.reporter.Warning(fmt.Sprintf("failed to remove skill %q: %v", skill.Name, err))
		}
	}
```

- [ ] **Step 5: Run all orchestrator tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestOrchestrator" -v`
Expected: ALL PASS

- [ ] **Step 6: Run full unit test suite**

Run: `cd /Users/edocsss/aec/src/facet && go test ./...`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "feat: handle 'all skills' entries in orchestrator and unapply"
```

---

### Task 5: Documentation updates

**Files:**
- Modify: `internal/docs/topics/ai.md`
- Modify: `internal/docs/topics/merge.md`
- Modify: `README.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Update `internal/docs/topics/ai.md`**

In `internal/docs/topics/ai.md`, replace the Skills section (lines 45–84) with:

```markdown
## Skills

Install skills from a source, optionally scoped to specific agents:

```yaml
ai:
  agents:
    - claude-code
    - cursor
  skills:
    # All skills from this source (omit skills list)
    - source: "@anthropic/claude-code-skills"
    # Specific skills from a source
    - source: "@my-org/custom-skills"
      skills:
        - deploy-helper
      agents:
        - claude-code
```

Each skill entry has:

- `source`: package or path passed to the skills installer (see formats below)
- `skills`: optional list of skill names from that source. **If omitted, all skills
  from the source are installed** (equivalent to `npx skills add <source> --all`).
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

    facet ai skills check

Update all installed skills to their latest versions:

    facet ai skills update

These commands pass through to the underlying skills CLI (`npx skills`) and
operate on all globally installed skills, not just those managed by facet.
```

- [ ] **Step 2: Update `internal/docs/topics/merge.md`**

In `internal/docs/topics/merge.md`, replace the `ai.skills` section (lines 67–69) with:

```markdown
## `ai.skills`

Skills are unioned by the tuple `(source, skill name)`. If the same pair appears in
multiple layers, the later layer wins, including its agent scoping.

An entry with an empty `skills` list means "all skills from this source." It is
treated as an atomic unit during merge:

- "All" in a later layer replaces all individual skill entries for the same source
  from earlier layers.
- Specific skills in a later layer replace an "all" entry for the same source from
  earlier layers.
```

- [ ] **Step 3: Update `README.md`**

In `README.md`, replace the AI Configuration YAML example (lines 187–201) with:

```yaml
# base.yaml
ai:
  agents: [claude-code, cursor, codex]
  permissions:
    claude-code:
      allow: [Read, Edit, Bash]
  skills:
    - source: "owner/repo"              # all skills from this source
    - source: "other/repo"
      skills: [code-review]             # specific skills only
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
```

- [ ] **Step 4: Update `docs/architecture/v1-design-spec.md`**

The v1-design-spec defers AI configuration as a separate v2 pass (line 18). No changes needed — the AI design lives in the separate AI design spec and in `internal/docs/topics/ai.md`.

- [ ] **Step 5: Run `go build` to verify embedded docs compile**

Run: `cd /Users/edocsss/aec/src/facet && go build ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/docs/topics/ai.md internal/docs/topics/merge.md README.md
git commit -m "docs: document 'all skills from source' feature"
```

---

### Task 6: E2E tests

**Files:**
- Create: `e2e/suites/14-all-skills-from-source.sh`

- [ ] **Step 1: Write the E2E test suite**

Create `e2e/suites/14-all-skills-from-source.sh`:

```bash
#!/bin/bash
# e2e/suites/14-all-skills-from-source.sh — "all skills from source" E2E tests
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
bash "$FIXTURE_DIR/setup-ai.sh"

# Test 1: Apply with "all skills" entry (no skills list) — should pass --all
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
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
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills --all"
assert_file_not_contains "$HOME/.mock-ai" "--skill"
echo "  all-skills entry passes --all flag"

# Test 2: Mixed all and specific entries — correct commands for each
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
    - source: "@org/specific-skills"
      skills: [my-skill]
YAML

: > "$HOME/.mock-ai"
facet_apply work
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills --all"
assert_file_contains "$HOME/.mock-ai" "npx skills add @org/specific-skills --skill my-skill"
echo "  mixed all and specific entries produce correct commands"

# Test 3: "all" scoped to specific agents
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code, cursor]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
    cursor:
      allow: ["Read(**)"]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      agents: [claude-code]
YAML

: > "$HOME/.mock-ai"
facet_apply work
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills --all -a claude-code"
assert_file_not_contains "$HOME/.mock-ai" "-a cursor"
echo "  all-skills respects agent scoping"

# Test 4: Transition from specific to all — no orphan removal
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      skills: [frontend-design]
YAML

: > "$HOME/.mock-ai"
facet_apply work
echo "  applied specific skills"

# Now switch to "all"
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
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
assert_file_not_contains "$HOME/.mock-ai" "npx skills remove"
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills --all"
echo "  specific to all transition: no orphan removal, installs with --all"

# Test 5: Transition from all to specific
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      skills: [frontend-design]
YAML

: > "$HOME/.mock-ai"
facet_apply work
assert_file_not_contains "$HOME/.mock-ai" "npx skills remove"
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills --skill frontend-design"
echo "  all to specific transition: no orphan removal, installs specific skills"

# Test 6: State tracking for "all" entries
assert_file_exists "$HOME/.facet/.state.json"
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].source' '@vercel-labs/agent-skills'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'frontend-design'
echo "  state tracks skills correctly after transitions"
```

- [ ] **Step 2: Run E2E tests locally**

Run: `cd /Users/edocsss/aec/src/facet && make e2e-native`
Expected: ALL PASS including new suite 14

- [ ] **Step 3: Commit**

```bash
git add e2e/suites/14-all-skills-from-source.sh
git commit -m "test: add E2E tests for all-skills-from-source feature"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run `make pre-commit`**

Run: `cd /Users/edocsss/aec/src/facet && make pre-commit`
Expected: ALL PASS — unit tests, native E2E, Docker E2E

- [ ] **Step 2: Review state.json schema**

Verify that `.state.json` correctly stores "all" entries with `"name": ""` by reading the state file after an E2E run. The existing `SkillState` struct already supports empty string names — no schema change needed.

- [ ] **Step 3: Final commit if any fixes needed**

Only if previous steps revealed issues that needed fixing.
