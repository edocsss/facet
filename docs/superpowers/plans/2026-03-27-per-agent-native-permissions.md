# Per-Agent Native Permissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the canonical permission vocabulary + mapper with per-agent native permission lists directly in YAML config.

**Architecture:** Remove the shared `permissions` field and `overrides` map from `AIConfig`. Replace with `permissions: map[agent_name] → {allow, deny}` where values are agent-native strings (e.g. `Read`, `Edit`, `Bash` for Claude Code; `Read(**)`, `Write(**)` for Cursor). Remove `PermissionMapper` interface and `DefaultPermissionMapper` entirely. The orchestrator passes permissions straight to providers without mapping.

**Tech Stack:** Go, YAML (gopkg.in/yaml.v3), testify

---

### Task 1: Update profile AI types — remove Overrides, make Permissions per-agent

**Files:**
- Modify: `internal/profile/ai_types.go`
- Test: `internal/profile/ai_types_test.go`

- [ ] **Step 1: Update `AIConfig` and remove `AIOverride`**

Replace the contents of `internal/profile/ai_types.go`:

```go
package profile

// AIConfig holds the AI tooling configuration for a facet profile.
type AIConfig struct {
	Agents      []string                       `yaml:"agents"`
	Permissions map[string]*PermissionsConfig  `yaml:"permissions,omitempty"`
	Skills      []SkillEntry                   `yaml:"skills,omitempty"`
	MCPs        []MCPEntry                     `yaml:"mcps,omitempty"`
}

// PermissionsConfig defines allow/deny lists for AI agent tool permissions.
type PermissionsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// SkillEntry describes a set of skills to load from a source, optionally
// scoped to specific agents.
type SkillEntry struct {
	Source string   `yaml:"source"`
	Skills []string `yaml:"skills"`
	Agents []string `yaml:"agents,omitempty"`
}

// MCPEntry describes a Model Context Protocol server to configure, optionally
// scoped to specific agents.
type MCPEntry struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Agents  []string          `yaml:"agents,omitempty"`
}
```

Key changes:
- `Permissions` changes from `*PermissionsConfig` to `map[string]*PermissionsConfig`
- `Overrides` field removed entirely
- `AIOverride` struct removed entirely

- [ ] **Step 2: Update YAML unmarshal tests**

Replace `internal/profile/ai_types_test.go`:

```go
package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAIConfig_UnmarshalYAML_Full(t *testing.T) {
	input := `
agents:
  - claude-code
  - cursor
permissions:
  claude-code:
    allow:
      - Read
      - Edit
      - Bash
    deny: []
  cursor:
    allow:
      - "Read(**)"
      - "Write(**)"
    deny:
      - "Shell(*)"
skills:
  - source: github.com/org/skills
    skills:
      - commit
      - review-pr
    agents:
      - claude-code
  - source: github.com/org/other-skills
    skills:
      - deploy
mcps:
  - name: playwright
    command: npx
    args:
      - "@playwright/mcp"
    env:
      DISPLAY: ":0"
    agents:
      - claude-code
  - name: filesystem
    command: npx
    args:
      - "@modelcontextprotocol/server-filesystem"
      - /tmp
`
	var cfg AIConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	// agents
	assert.Equal(t, []string{"claude-code", "cursor"}, cfg.Agents)

	// permissions — per-agent native terms
	require.Len(t, cfg.Permissions, 2)
	claude := cfg.Permissions["claude-code"]
	require.NotNil(t, claude)
	assert.Equal(t, []string{"Read", "Edit", "Bash"}, claude.Allow)
	assert.Equal(t, []string{}, claude.Deny)

	cursor := cfg.Permissions["cursor"]
	require.NotNil(t, cursor)
	assert.Equal(t, []string{"Read(**)", "Write(**)"}, cursor.Allow)
	assert.Equal(t, []string{"Shell(*)"}, cursor.Deny)

	// skills
	require.Len(t, cfg.Skills, 2)
	assert.Equal(t, "github.com/org/skills", cfg.Skills[0].Source)
	assert.Equal(t, []string{"commit", "review-pr"}, cfg.Skills[0].Skills)
	assert.Equal(t, []string{"claude-code"}, cfg.Skills[0].Agents)

	// mcps
	require.Len(t, cfg.MCPs, 2)
	assert.Equal(t, "playwright", cfg.MCPs[0].Name)
}

func TestAIConfig_UnmarshalYAML_Empty(t *testing.T) {
	input := `
agents:
  - claude-code
`
	var cfg AIConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"claude-code"}, cfg.Agents)
	assert.Nil(t, cfg.Permissions)
	assert.Nil(t, cfg.Skills)
	assert.Nil(t, cfg.MCPs)
}

func TestFacetConfig_WithAI(t *testing.T) {
	input := `
extends: base
vars:
  editor: nvim
packages:
  - name: ripgrep
    install: brew install ripgrep
configs:
  ~/.zshrc: configs/.zshrc
ai:
  agents:
    - claude-code
  permissions:
    claude-code:
      allow:
        - Bash
      deny: []
  mcps:
    - name: filesystem
      command: npx
      args:
        - "@modelcontextprotocol/server-filesystem"
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "base", cfg.Extends)
	require.NotNil(t, cfg.AI)
	assert.Equal(t, []string{"claude-code"}, cfg.AI.Agents)
	require.NotNil(t, cfg.AI.Permissions["claude-code"])
	assert.Equal(t, []string{"Bash"}, cfg.AI.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{}, cfg.AI.Permissions["claude-code"].Deny)
}

func TestFacetConfig_WithoutAI(t *testing.T) {
	input := `
extends: base
vars:
  editor: nvim
packages:
  - name: ripgrep
    install: brew install ripgrep
configs:
  ~/.zshrc: configs/.zshrc
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "base", cfg.Extends)
	assert.Nil(t, cfg.AI)
}
```

- [ ] **Step 3: Run tests to verify**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestAIConfig -v`
Expected: PASS (both type tests pass with new YAML structure)

- [ ] **Step 4: Commit**

```bash
git add internal/profile/ai_types.go internal/profile/ai_types_test.go
git commit -m "refactor: make AI permissions per-agent with native terms

Remove shared canonical permissions and AIOverride. Permissions are now
a map[string]*PermissionsConfig keyed by agent name, using each agent's
native permission strings directly."
```

---

### Task 2: Update merger — remove overrides, merge permissions per-agent

**Files:**
- Modify: `internal/profile/ai_merger.go`
- Modify: `internal/profile/ai_merger_test.go`

- [ ] **Step 1: Rewrite merger functions**

Replace `internal/profile/ai_merger.go`. Key changes:
- `mergePermissions` now merges `map[string]*PermissionsConfig` per-agent (last writer wins per agent key)
- Remove `mergeOverrides` entirely
- Remove override-related logic from `mergeAI`

```go
package profile

import (
	"sort"
	"strings"
)

// mergePermissions performs per-agent last-writer-wins on permissions maps.
// For each agent key, if overlay has it, overlay wins. Otherwise base is kept.
// Both nil → nil.
func mergePermissions(base, overlay map[string]*PermissionsConfig) map[string]*PermissionsConfig {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := make(map[string]*PermissionsConfig, len(base)+len(overlay))
	for agent, perms := range base {
		result[agent] = perms
	}
	for agent, perms := range overlay {
		result[agent] = perms
	}
	return result
}

// skillTuple holds a single normalized (source, skillName, agents) triple.
type skillTuple struct {
	source    string
	skillName string
	agents    []string
}

// mergeSkills unions two SkillEntry slices by (source, skill_name) tuple key.
// Overlay tuples overwrite base tuples on conflict. Results are re-grouped by source.
// Both nil → nil.
func mergeSkills(base, overlay []SkillEntry) []SkillEntry {
	if base == nil && overlay == nil {
		return nil
	}

	// Track insertion order by tuple key.
	type entry struct {
		key   string // "source\x00skillName"
		tuple skillTuple
	}

	seen := make(map[string]int) // key → index in ordered
	ordered := []entry{}

	add := func(source, skillName string, agents []string) {
		key := source + "\x00" + skillName
		t := skillTuple{source: source, skillName: skillName, agents: agents}
		if idx, exists := seen[key]; exists {
			ordered[idx].tuple = t
		} else {
			seen[key] = len(ordered)
			ordered = append(ordered, entry{key: key, tuple: t})
		}
	}

	for _, e := range base {
		for _, skill := range e.Skills {
			add(e.Source, skill, e.Agents)
		}
	}
	for _, e := range overlay {
		for _, skill := range e.Skills {
			add(e.Source, skill, e.Agents)
		}
	}

	// Re-group tuples by (source, agents), preserving insertion order. This keeps
	// per-skill agent targeting intact even when the same source contributes
	// differently scoped skills across layers.
	type groupEntry struct {
		source string
		skills []string
		agents []string
	}
	groupIdx := make(map[string]int) // "source\x00sortedAgents" → index in groups
	groups := []groupEntry{}

	for _, oe := range ordered {
		t := oe.tuple
		key := t.source + "\x00" + canonicalAgentsKey(t.agents)
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

	result := make([]SkillEntry, 0, len(groups))
	for _, g := range groups {
		result = append(result, SkillEntry{
			Source: g.source,
			Skills: g.skills,
			Agents: g.agents,
		})
	}
	return result
}

func canonicalAgentsKey(agents []string) string {
	if len(agents) == 0 {
		return ""
	}
	sorted := append([]string{}, agents...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

// mergeMCPs unions two MCPEntry slices by name. Overlay wins on name conflict.
// Both nil → nil.
func mergeMCPs(base, overlay []MCPEntry) []MCPEntry {
	if base == nil && overlay == nil {
		return nil
	}

	byName := make(map[string]MCPEntry)
	var order []string

	for _, m := range base {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range overlay {
		if _, exists := byName[m.Name]; !exists {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}

	result := make([]MCPEntry, 0, len(order))
	for _, name := range order {
		result = append(result, byName[name])
	}
	return result
}

// mergeAI orchestrates the merge of two AIConfig values.
// agents: last writer wins. Both nil → nil. One nil → return the other.
func mergeAI(base, overlay *AIConfig) *AIConfig {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := &AIConfig{}

	// agents: last writer wins
	if overlay.Agents != nil {
		result.Agents = overlay.Agents
	} else {
		result.Agents = base.Agents
	}

	// permissions: per-agent last writer wins
	result.Permissions = mergePermissions(base.Permissions, overlay.Permissions)

	// skills: union by (source, skill) tuple
	result.Skills = mergeSkills(base.Skills, overlay.Skills)

	// mcps: union by name
	result.MCPs = mergeMCPs(base.MCPs, overlay.MCPs)

	return result
}
```

- [ ] **Step 2: Rewrite merger tests**

Replace `internal/profile/ai_merger_test.go`:

```go
package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mergePermissions ---

func TestMergePermissions_PerAgentOverlayWins(t *testing.T) {
	base := map[string]*PermissionsConfig{
		"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{}},
		"cursor":      {Allow: []string{"Read(**)"}, Deny: []string{}},
	}
	overlay := map[string]*PermissionsConfig{
		"claude-code": {Allow: []string{"Read"}, Deny: []string{"Bash"}},
	}

	result := mergePermissions(base, overlay)

	require.NotNil(t, result)
	// claude-code: overlay wins
	require.NotNil(t, result["claude-code"])
	assert.Equal(t, []string{"Read"}, result["claude-code"].Allow)
	assert.Equal(t, []string{"Bash"}, result["claude-code"].Deny)
	// cursor: base preserved (overlay doesn't define cursor)
	require.NotNil(t, result["cursor"])
	assert.Equal(t, []string{"Read(**)"}, result["cursor"].Allow)
}

func TestMergePermissions_BaseOnly(t *testing.T) {
	base := map[string]*PermissionsConfig{
		"claude-code": {Allow: []string{"Read"}, Deny: []string{}},
	}

	result := mergePermissions(base, nil)

	require.NotNil(t, result)
	assert.Equal(t, base, result)
}

func TestMergePermissions_OverlayOnly(t *testing.T) {
	overlay := map[string]*PermissionsConfig{
		"cursor": {Allow: []string{"Read(**)"}, Deny: []string{}},
	}

	result := mergePermissions(nil, overlay)

	require.NotNil(t, result)
	assert.Equal(t, overlay, result)
}

func TestMergePermissions_BothNil(t *testing.T) {
	result := mergePermissions(nil, nil)
	assert.Nil(t, result)
}

// --- mergeSkills ---

func TestMergeSkills_UnionBySourceAndName(t *testing.T) {
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1", "skill-2"}, Agents: []string{"claude"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-2", "skill-3"}, Agents: []string{"cursor"}},
		{Source: "source-b", Skills: []string{"skill-x"}},
	}

	result := mergeSkills(base, overlay)

	require.NotNil(t, result)

	var sourceAEntries []SkillEntry
	var sourceBEntries []SkillEntry
	for _, e := range result {
		switch e.Source {
		case "source-a":
			sourceAEntries = append(sourceAEntries, e)
		case "source-b":
			sourceBEntries = append(sourceBEntries, e)
		}
	}

	require.Len(t, sourceAEntries, 2)
	require.Len(t, sourceBEntries, 1)

	assert.ElementsMatch(t, []string{"skill-1"}, sourceAEntries[0].Skills)
	assert.ElementsMatch(t, []string{"skill-2", "skill-3"}, sourceAEntries[1].Skills)
	assert.Equal(t, []string{"claude"}, sourceAEntries[0].Agents)
	assert.Equal(t, []string{"cursor"}, sourceAEntries[1].Agents)
	assert.Equal(t, []string{"skill-x"}, sourceBEntries[0].Skills)
}

func TestMergeSkills_OverlayAgentsWin(t *testing.T) {
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1"}, Agents: []string{"claude"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1"}, Agents: []string{"cursor"}},
	}

	result := mergeSkills(base, overlay)

	require.NotNil(t, result)
	require.Len(t, result, 1)
	assert.Equal(t, []string{"cursor"}, result[0].Agents)
}

func TestMergeSkills_PreservesPerSkillAgentsWithinSameSource(t *testing.T) {
	base := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-1"}, Agents: []string{"claude"}},
	}
	overlay := []SkillEntry{
		{Source: "source-a", Skills: []string{"skill-2"}, Agents: []string{"cursor"}},
	}

	result := mergeSkills(base, overlay)

	require.Len(t, result, 2)

	entries := make(map[string]SkillEntry)
	for _, entry := range result {
		require.Len(t, entry.Skills, 1)
		entries[entry.Skills[0]] = entry
	}

	require.Contains(t, entries, "skill-1")
	require.Contains(t, entries, "skill-2")
	assert.Equal(t, []string{"claude"}, entries["skill-1"].Agents)
	assert.Equal(t, []string{"cursor"}, entries["skill-2"].Agents)
}

func TestMergeSkills_BothNil(t *testing.T) {
	result := mergeSkills(nil, nil)
	assert.Nil(t, result)
}

// --- mergeMCPs ---

func TestMergeMCPs_UnionByName(t *testing.T) {
	base := []MCPEntry{
		{Name: "playwright", Command: "npx", Args: []string{"@playwright/mcp"}},
		{Name: "filesystem", Command: "npx", Args: []string{"@modelcontextprotocol/server-filesystem"}},
	}
	overlay := []MCPEntry{
		{Name: "filesystem", Command: "npx", Args: []string{"@modelcontextprotocol/server-filesystem", "/tmp"}, Agents: []string{"claude"}},
		{Name: "github", Command: "npx", Args: []string{"@github/mcp"}},
	}

	result := mergeMCPs(base, overlay)

	require.Len(t, result, 3)

	byName := make(map[string]MCPEntry)
	for _, m := range result {
		byName[m.Name] = m
	}

	require.Contains(t, byName, "playwright")
	require.Contains(t, byName, "filesystem")
	require.Contains(t, byName, "github")

	fs := byName["filesystem"]
	assert.Equal(t, []string{"@modelcontextprotocol/server-filesystem", "/tmp"}, fs.Args)
	assert.Equal(t, []string{"claude"}, fs.Agents)
}

func TestMergeMCPs_BothNil(t *testing.T) {
	result := mergeMCPs(nil, nil)
	assert.Nil(t, result)
}

// --- mergeAI ---

func TestMergeAI_Full(t *testing.T) {
	base := &AIConfig{
		Agents: []string{"claude-code"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{}},
		},
		Skills: []SkillEntry{
			{Source: "github.com/org/skills", Skills: []string{"commit"}, Agents: []string{"claude-code"}},
		},
		MCPs: []MCPEntry{
			{Name: "filesystem", Command: "npx", Args: []string{"@modelcontextprotocol/server-filesystem"}},
		},
	}
	overlay := &AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{"Bash"}},
			"cursor":      {Allow: []string{"Read(**)"}, Deny: []string{}},
		},
		Skills: []SkillEntry{
			{Source: "github.com/org/skills", Skills: []string{"review-pr"}, Agents: []string{"cursor"}},
		},
		MCPs: []MCPEntry{
			{Name: "playwright", Command: "npx", Args: []string{"@playwright/mcp"}},
		},
	}

	result := mergeAI(base, overlay)

	require.NotNil(t, result)

	// agents: last writer wins
	assert.Equal(t, []string{"claude-code", "cursor"}, result.Agents)

	// permissions: per-agent overlay wins
	require.NotNil(t, result.Permissions)
	require.Len(t, result.Permissions, 2)
	assert.Equal(t, []string{"Read"}, result.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{"Bash"}, result.Permissions["claude-code"].Deny)
	assert.Equal(t, []string{"Read(**)"}, result.Permissions["cursor"].Allow)

	// skills: union of (source, skill) tuples
	require.NotNil(t, result.Skills)

	// mcps: union by name (2 different names = 2 entries)
	assert.Len(t, result.MCPs, 2)
}

func TestMergeAI_BaseNil(t *testing.T) {
	overlay := &AIConfig{
		Agents: []string{"claude-code"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{}},
		},
	}

	result := mergeAI(nil, overlay)

	require.NotNil(t, result)
	assert.Equal(t, overlay, result)
}

func TestMergeAI_OverlayNil(t *testing.T) {
	base := &AIConfig{
		Agents: []string{"cursor"},
		Permissions: map[string]*PermissionsConfig{
			"cursor": {Allow: []string{"Read(**)"}, Deny: []string{}},
		},
	}

	result := mergeAI(base, nil)

	require.NotNil(t, result)
	assert.Equal(t, base, result)
}

func TestMergeAI_BothNil(t *testing.T) {
	result := mergeAI(nil, nil)
	assert.Nil(t, result)
}

func TestMergeAI_OverlayAddsNewAgent(t *testing.T) {
	base := &AIConfig{
		Agents: []string{"claude-code"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{}},
		},
	}
	overlay := &AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Permissions: map[string]*PermissionsConfig{
			"cursor": {Allow: []string{"Read(**)"}, Deny: []string{}},
		},
	}

	result := mergeAI(base, overlay)

	require.NotNil(t, result)
	assert.Equal(t, []string{"claude-code", "cursor"}, result.Agents)
	// claude-code from base preserved, cursor from overlay added
	require.Len(t, result.Permissions, 2)
	assert.Equal(t, []string{"Read"}, result.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{"Read(**)"}, result.Permissions["cursor"].Allow)
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run "TestMerge|TestMergeAI" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/profile/ai_merger.go internal/profile/ai_merger_test.go
git commit -m "refactor: update merger for per-agent permissions map

mergePermissions now merges map[string]*PermissionsConfig with per-agent
last-writer-wins. mergeOverrides removed. mergeAI simplified."
```

---

### Task 3: Update resolver — remove Overrides handling

**Files:**
- Modify: `internal/profile/resolver.go`

- [ ] **Step 1: Remove Overrides from resolveAI**

In `internal/profile/resolver.go`, update `resolveAI` to remove the `Overrides` field copy:

```go
func resolveAI(ai *AIConfig, vars map[string]any) (*AIConfig, error) {
	if ai == nil {
		return nil, nil
	}

	result := &AIConfig{
		Permissions: ai.Permissions,
	}

	// Deep-copy Agents slice
	if ai.Agents != nil {
		result.Agents = make([]string, len(ai.Agents))
		copy(result.Agents, ai.Agents)
	}

	// Deep-copy Skills slice (including inner slices to prevent aliasing)
	if ai.Skills != nil {
		result.Skills = make([]SkillEntry, len(ai.Skills))
		for i, entry := range ai.Skills {
			result.Skills[i] = SkillEntry{
				Source: entry.Source,
				Skills: append([]string{}, entry.Skills...),
				Agents: append([]string{}, entry.Agents...),
			}
		}
	}

	// Resolve MCPs
	if ai.MCPs != nil {
		result.MCPs = make([]MCPEntry, len(ai.MCPs))
		for i, mcp := range ai.MCPs {
			resolved, err := resolveMCPEntry(i, mcp, vars)
			if err != nil {
				return nil, err
			}
			result.MCPs[i] = resolved
		}
	}

	return result, nil
}
```

- [ ] **Step 2: Run profile tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/profile/resolver.go
git commit -m "refactor: remove Overrides from resolveAI"
```

---

### Task 4: Update ai.Resolve — read from per-agent permissions map

**Files:**
- Modify: `internal/ai/resolve.go`
- Modify: `internal/ai/resolve_test.go`

- [ ] **Step 1: Simplify Resolve function**

Replace `internal/ai/resolve.go`:

```go
package ai

import "facet/internal/profile"

// Resolve takes a merged AIConfig and produces per-agent effective configs.
// Permissions are already in agent-native terms (no mapping needed).
// Returns nil if cfg is nil.
func Resolve(cfg *profile.AIConfig) EffectiveAIConfig {
	if cfg == nil {
		return nil
	}

	result := make(EffectiveAIConfig, len(cfg.Agents))

	for _, agent := range cfg.Agents {
		// Step 1: look up per-agent permissions
		perms := ResolvedPermissions{}
		if agentPerms, ok := cfg.Permissions[agent]; ok && agentPerms != nil {
			perms.Allow = append([]string{}, agentPerms.Allow...)
			perms.Deny = append([]string{}, agentPerms.Deny...)
		}

		// Step 2: filter skills and flatten each SkillEntry into ResolvedSkills
		var skills []ResolvedSkill
		for _, entry := range cfg.Skills {
			if agentIncluded(agent, entry.Agents) {
				for _, skillName := range entry.Skills {
					skills = append(skills, ResolvedSkill{
						Source: entry.Source,
						Name:   skillName,
					})
				}
			}
		}

		// Step 3: filter MCPs
		var mcps []ResolvedMCP
		for _, entry := range cfg.MCPs {
			if agentIncluded(agent, entry.Agents) {
				argsCopy := append([]string{}, entry.Args...)
				envCopy := make(map[string]string, len(entry.Env))
				for k, v := range entry.Env {
					envCopy[k] = v
				}
				mcps = append(mcps, ResolvedMCP{
					Name:    entry.Name,
					Command: entry.Command,
					Args:    argsCopy,
					Env:     envCopy,
				})
			}
		}

		result[agent] = EffectiveAgentConfig{
			Permissions: perms,
			Skills:      skills,
			MCPs:        mcps,
		}
	}

	return result
}

// agentIncluded returns true if itemAgents is empty (meaning all agents)
// or contains the given agent name.
func agentIncluded(agent string, itemAgents []string) bool {
	if len(itemAgents) == 0 {
		return true
	}
	for _, a := range itemAgents {
		if a == agent {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Rewrite resolve tests**

Replace `internal/ai/resolve_test.go`:

```go
package ai

import (
	"facet/internal/profile"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_BasicResolution(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Permissions: map[string]*profile.PermissionsConfig{
			"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{"Bash"}},
			"cursor":      {Allow: []string{"Read(**)"}, Deny: []string{"Shell(*)"}},
		},
		Skills: []profile.SkillEntry{
			{
				Source: "github.com/example/skills",
				Skills: []string{"linting", "formatting"},
			},
		},
		MCPs: []profile.MCPEntry{
			{
				Name:    "filesystem",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Env:     map[string]string{"ROOT": "/tmp"},
			},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)
	assert.Len(t, result, 2)

	// claude-code gets its own native permissions
	claudeCfg := result["claude-code"]
	assert.Equal(t, []string{"Read", "Edit"}, claudeCfg.Permissions.Allow)
	assert.Equal(t, []string{"Bash"}, claudeCfg.Permissions.Deny)
	require.Len(t, claudeCfg.Skills, 2)
	require.Len(t, claudeCfg.MCPs, 1)

	// cursor gets its own native permissions
	cursorCfg := result["cursor"]
	assert.Equal(t, []string{"Read(**)"}, cursorCfg.Permissions.Allow)
	assert.Equal(t, []string{"Shell(*)"}, cursorCfg.Permissions.Deny)
	require.Len(t, cursorCfg.Skills, 2)
	require.Len(t, cursorCfg.MCPs, 1)
}

func TestResolve_PerItemAgentFiltering(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor", "codex"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/shared", Skills: []string{"shared-skill"}},
			{Source: "github.com/example/claude", Skills: []string{"claude-skill"}, Agents: []string{"claude-code"}},
		},
		MCPs: []profile.MCPEntry{
			{Name: "mcp-all", Command: "npx", Args: []string{"mcp-all"}},
			{Name: "mcp-claude-cursor", Command: "npx", Args: []string{"mcp-claude-cursor"}, Agents: []string{"claude-code", "cursor"}},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)
	assert.Len(t, result, 3)

	claudeCfg := result["claude-code"]
	require.Len(t, claudeCfg.Skills, 2)
	require.Len(t, claudeCfg.MCPs, 2)

	cursorCfg := result["cursor"]
	require.Len(t, cursorCfg.Skills, 1)
	require.Len(t, cursorCfg.MCPs, 2)

	codexCfg := result["codex"]
	require.Len(t, codexCfg.Skills, 1)
	require.Len(t, codexCfg.MCPs, 1)
}

func TestResolve_AgentWithNoPermissions(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "codex"},
		Permissions: map[string]*profile.PermissionsConfig{
			"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{}},
			// codex has no permissions entry
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)

	claudeCfg := result["claude-code"]
	assert.Equal(t, []string{"Read", "Edit"}, claudeCfg.Permissions.Allow)

	codexCfg := result["codex"]
	assert.Nil(t, codexCfg.Permissions.Allow)
	assert.Nil(t, codexCfg.Permissions.Deny)
}

func TestResolve_Nil(t *testing.T) {
	result := Resolve(nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestResolve -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ai/resolve.go internal/ai/resolve_test.go
git commit -m "refactor: simplify Resolve to read per-agent permissions directly

No more shared permissions or override replacement logic. Each agent
looks up its own permissions entry from the per-agent map."
```

---

### Task 5: Delete permission mapper and remove from interfaces

**Files:**
- Delete: `internal/ai/permission_mapper.go`
- Delete: `internal/ai/permission_mapper_test.go`
- Modify: `internal/ai/interfaces.go`

- [ ] **Step 1: Delete permission mapper files**

```bash
rm internal/ai/permission_mapper.go internal/ai/permission_mapper_test.go
```

- [ ] **Step 2: Remove PermissionMapper interface from interfaces.go**

Update `internal/ai/interfaces.go` to remove the `PermissionMapper` interface:

```go
package ai

// AgentProvider handles agent-specific file I/O for permissions and MCPs.
type AgentProvider interface {
	Name() string
	ApplyPermissions(permissions ResolvedPermissions) error
	RemovePermissions(previousPermissions ResolvedPermissions) error
	RegisterMCP(mcp ResolvedMCP) error
	RemoveMCP(name string) error
	SettingsFilePath() string
}

// SkillsManager manages skill installation/removal via external CLI.
type SkillsManager interface {
	Install(source string, skills []string, agents []string) error
	Remove(skills []string, agents []string) error
}

// CommandRunner executes a command name with a stable argv vector. It is
// defined here (not imported from packages/) to avoid cross-domain coupling.
type CommandRunner interface {
	Run(name string, args ...string) error
}
```

- [ ] **Step 3: Verify compile**

Run: `cd /Users/edocsss/aec/src/facet && go build ./internal/ai/...`
Expected: This will fail because orchestrator still references permMapper — that's expected, we fix it in the next task.

- [ ] **Step 4: Commit**

```bash
git add -A internal/ai/permission_mapper.go internal/ai/permission_mapper_test.go internal/ai/interfaces.go
git commit -m "refactor: delete PermissionMapper interface and implementation"
```

---

### Task 6: Update orchestrator — remove permission mapping

**Files:**
- Modify: `internal/ai/orchestrator.go`
- Modify: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Remove permMapper from Orchestrator**

Update `internal/ai/orchestrator.go`. Key changes:
- Remove `permMapper` field from `Orchestrator` struct
- Remove `permMapper` param from `NewOrchestrator`
- In `applyPermissions`, pass permissions directly to provider (no mapping step)
- Remove the "no mappable permissions" skip logic (permissions are already native)

```go
package ai

import (
	"fmt"
	"sort"
	"strings"
)

// Reporter is the interface that the Orchestrator uses for user-facing output.
type Reporter interface {
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Header(msg string)
	PrintLine(msg string)
	Dim(text string) string
}

// Orchestrator coordinates AI configuration across all agent providers.
type Orchestrator struct {
	providers     map[string]AgentProvider
	skillsManager SkillsManager
	reporter      Reporter
}

// NewOrchestrator constructs an Orchestrator with the given dependencies.
func NewOrchestrator(
	providers map[string]AgentProvider,
	skillsManager SkillsManager,
	reporter Reporter,
) *Orchestrator {
	return &Orchestrator{
		providers:     providers,
		skillsManager: skillsManager,
		reporter:      reporter,
	}
}

// Apply applies the effective AI configuration to all agents and returns the
// resulting state. Individual failures are non-fatal: they are logged and
// skipped. The function never returns an error.
func (o *Orchestrator) Apply(config EffectiveAIConfig, previousState *AIState) (*AIState, error) {
	state := &AIState{
		Permissions: make(map[string]PermissionState),
	}

	// 1. Apply permissions for each agent.
	o.applyPermissions(config, previousState, state)

	// 2. Apply skills with orphan removal.
	o.applySkills(config, previousState, state)

	// 3. Apply MCPs with orphan removal.
	o.applyMCPs(config, previousState, state)

	return state, nil
}

// Unapply removes all AI configuration tracked in previousState. Order is
// reverse of apply: MCPs, then skills, then permissions.
func (o *Orchestrator) Unapply(previousState *AIState) error {
	if previousState == nil {
		return nil
	}

	// 1. Remove MCPs.
	for _, mcp := range previousState.MCPs {
		for _, agent := range mcp.Agents {
			provider, ok := o.providers[agent]
			if !ok {
				o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping MCP removal for %q", agent, mcp.Name))
				continue
			}
			if err := provider.RemoveMCP(mcp.Name); err != nil {
				o.reporter.Warning(fmt.Sprintf("failed to remove MCP %q from %q: %v", mcp.Name, agent, err))
			}
		}
	}

	// 2. Remove skills.
	for _, skill := range previousState.Skills {
		if err := o.skillsManager.Remove([]string{skill.Name}, skill.Agents); err != nil {
			o.reporter.Warning(fmt.Sprintf("failed to remove skill %q: %v", skill.Name, err))
		}
	}

	// 3. Remove permissions.
	for agent, ps := range previousState.Permissions {
		provider, ok := o.providers[agent]
		if !ok {
			o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping permission removal", agent))
			continue
		}
		perms := ResolvedPermissions{Allow: ps.Allow, Deny: ps.Deny}
		if err := provider.RemovePermissions(perms); err != nil {
			o.reporter.Warning(fmt.Sprintf("failed to remove permissions for %q: %v", agent, err))
		}
	}

	return nil
}

// applyPermissions removes permissions for dropped agents, then applies
// native permissions directly for each current agent.
func (o *Orchestrator) applyPermissions(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	if previousState != nil {
		for agent, ps := range previousState.Permissions {
			if _, exists := config[agent]; exists {
				continue
			}

			provider, ok := o.providers[agent]
			if !ok {
				o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping permission removal", agent))
				continue
			}

			perms := ResolvedPermissions{Allow: ps.Allow, Deny: ps.Deny}
			if err := provider.RemovePermissions(perms); err != nil {
				o.reporter.Warning(fmt.Sprintf("failed to remove permissions for dropped agent %q: %v", agent, err))
			} else {
				o.reporter.Success(fmt.Sprintf("removed permissions for %s", agent))
			}
		}
	}

	// Sort agent names for deterministic iteration order.
	agents := sortedKeys(config)

	for _, agent := range agents {
		agentCfg := config[agent]

		provider, ok := o.providers[agent]
		if !ok {
			o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping permissions", agent))
			continue
		}

		perms := agentCfg.Permissions
		if err := provider.ApplyPermissions(perms); err != nil {
			o.reporter.Error(fmt.Sprintf("failed to apply permissions for %q: %v", agent, err))
			continue
		}

		state.Permissions[agent] = PermissionState{
			Allow: perms.Allow,
			Deny:  perms.Deny,
		}
		o.reporter.Success(fmt.Sprintf("applied permissions for %s", agent))
	}
}

// skillGroupKey uniquely identifies a group of skills that share the same
// source and agent set.
type skillGroupKey struct {
	source string
	agents string // sorted, comma-joined agent names
}

type skillID struct {
	source string
	name   string
}

// applySkills installs current skills and removes orphans from previousState.
func (o *Orchestrator) applySkills(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	currentSkills := make(map[skillID]map[string]struct{})

	for agent, agentCfg := range config {
		for _, skill := range agentCfg.Skills {
			key := skillID{source: skill.Source, name: skill.Name}
			if _, exists := currentSkills[key]; !exists {
				currentSkills[key] = make(map[string]struct{})
			}
			currentSkills[key][agent] = struct{}{}
		}
	}
	if previousState != nil {
		for _, prevSkill := range previousState.Skills {
			currentAgents := currentSkills[skillID{source: prevSkill.Source, name: prevSkill.Name}]
			for _, agent := range prevSkill.Agents {
				if _, exists := currentAgents[agent]; exists {
					continue
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
	for id, agentSet := range currentSkills {
		agents := sortedSetKeys(agentSet)
		key := skillGroupKey{
			source: id.source,
			agents: strings.Join(agents, ","),
		}
		groups[key] = append(groups[key], id.name)
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
		sort.Strings(skills)
		agents := strings.Split(gk.agents, ",")

		if err := o.skillsManager.Install(gk.source, skills, agents); err != nil {
			o.reporter.Warning(fmt.Sprintf("failed to install skills %v from %q: %v", skills, gk.source, err))
			continue
		}

		for _, skillName := range skills {
			state.Skills = append(state.Skills, SkillState{
				Source: gk.source,
				Name:   skillName,
				Agents: agents,
			})
		}
		o.reporter.Success(fmt.Sprintf("installed skills %v from %s", skills, gk.source))
	}
}

// applyMCPs registers current MCPs and removes orphans from previousState.
func (o *Orchestrator) applyMCPs(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	currentMCPs := make(map[string]map[string]struct{})
	mcpConfigs := make(map[string]ResolvedMCP)

	for agent, agentCfg := range config {
		for _, mcp := range agentCfg.MCPs {
			if _, exists := currentMCPs[mcp.Name]; !exists {
				currentMCPs[mcp.Name] = make(map[string]struct{})
			}
			currentMCPs[mcp.Name][agent] = struct{}{}
			if _, exists := mcpConfigs[mcp.Name]; !exists {
				mcpConfigs[mcp.Name] = mcp
			}
		}
	}
	if previousState != nil {
		for _, prevMCP := range previousState.MCPs {
			currentAgents := currentMCPs[prevMCP.Name]
			for _, agent := range prevMCP.Agents {
				if _, exists := currentAgents[agent]; exists {
					continue
				}
				provider, ok := o.providers[agent]
				if !ok {
					o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping orphan MCP removal for %q", agent, prevMCP.Name))
					continue
				}
				if err := provider.RemoveMCP(prevMCP.Name); err != nil {
					o.reporter.Warning(fmt.Sprintf("failed to remove orphan MCP %q from %q: %v", prevMCP.Name, agent, err))
				} else {
					o.reporter.Success(fmt.Sprintf("removed orphan MCP %q from %s", prevMCP.Name, agent))
				}
			}
		}
	}

	mcpSuccessAgents := make(map[string][]string)

	mcpNames := make([]string, 0, len(currentMCPs))
	for name := range currentMCPs {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)

	for _, mcpName := range mcpNames {
		agents := sortedSetKeys(currentMCPs[mcpName])
		mcpCfg := mcpConfigs[mcpName]

		for _, agent := range agents {
			provider, ok := o.providers[agent]
			if !ok {
				o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping MCP registration for %q", agent, mcpName))
				continue
			}
			if err := provider.RegisterMCP(mcpCfg); err != nil {
				o.reporter.Warning(fmt.Sprintf("failed to register MCP %q for %q: %v", mcpName, agent, err))
				continue
			}
			mcpSuccessAgents[mcpName] = append(mcpSuccessAgents[mcpName], agent)
		}
	}

	for _, mcpName := range mcpNames {
		agents, ok := mcpSuccessAgents[mcpName]
		if !ok || len(agents) == 0 {
			continue
		}
		state.MCPs = append(state.MCPs, MCPState{
			Name:   mcpName,
			Agents: agents,
		})
	}
}

// sortedKeys returns the keys of an EffectiveAIConfig map sorted alphabetically.
func sortedKeys(m EffectiveAIConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

- [ ] **Step 2: Update orchestrator tests — remove mockMapper**

The orchestrator tests need to:
1. Remove `mockMapper` struct entirely
2. Remove all mapper params from `NewOrchestrator` calls
3. Update `TestOrchestrator_Apply_Permissions` to use native terms directly (no canonical→native mapping)
4. Remove all `mockMapper` references from test helper construction

For `TestOrchestrator_Apply_Permissions`, the config now uses native terms directly:

```go
config := EffectiveAIConfig{
    "claude-code": EffectiveAgentConfig{
        Permissions: ResolvedPermissions{
            Allow: []string{"Read", "Edit"},
            Deny:  []string{"Bash"},
        },
    },
}
```

And the assertions verify that the provider received exactly those native terms (no mapping transformation).

For `TestOrchestrator_Apply_NonFatalPermissionError`, update configs to use native terms:

```go
config := EffectiveAIConfig{
    "claude-code": EffectiveAgentConfig{
        Permissions: ResolvedPermissions{Allow: []string{"Read"}},
    },
    "cursor": EffectiveAgentConfig{
        Permissions: ResolvedPermissions{Allow: []string{"Read(**)"}},
    },
}
```

Remove `mockMapper` from all test helper structs and `NewOrchestrator` calls. Every test that previously created a `mockMapper` should just remove that variable and the constructor arg.

- [ ] **Step 3: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "refactor: remove permission mapper from orchestrator

Orchestrator now passes native permissions directly to providers.
No mapping step needed since permissions are already agent-native."
```

---

### Task 7: Update main.go — remove permMapper wiring

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Remove permMapper from main.go**

Remove the `permMapper` creation line and update the `NewOrchestrator` call:

```go
// Delete this line:
// permMapper := ai.NewDefaultPermissionMapper()

// Change this:
// aiOrchestrator := ai.NewOrchestrator(providers, permMapper, skillsMgr, r)
// To:
aiOrchestrator := ai.NewOrchestrator(providers, skillsMgr, r)
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/edocsss/aec/src/facet && go build ./...`
Expected: PASS (clean build)

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "refactor: remove permMapper wiring from main.go"
```

---

### Task 8: Update E2E fixtures and tests

**Files:**
- Modify: `e2e/fixtures/setup-ai.sh`
- Modify: `e2e/suites/10-ai-config.sh`

- [ ] **Step 1: Update setup-ai.sh fixture**

Replace `e2e/fixtures/setup-ai.sh`:

```bash
#!/bin/bash
#
# Extends basic setup with AI configuration in profiles.
# Must be called AFTER setup-basic.sh.
set -euo pipefail

CONFIG_DIR="$HOME/dotfiles"

# ── Add AI section to base.yaml ──
cat >> "$CONFIG_DIR/base.yaml" << 'YAML'

ai:
  agents: [claude-code, cursor]
  permissions:
    claude-code:
      allow: [Read, Edit, Bash]
      deny: []
    cursor:
      allow: ["Read(**)", "Write(**)"]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      skills: [frontend-design]
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
    - name: github
      command: npx
      args: ["@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${facet:secret_key}"
      agents: [claude-code]
YAML

# ── Create minimal profile for orphan testing ──
cat > "$CONFIG_DIR/profiles/minimal.yaml" << 'YAML'
extends: base

ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
YAML

# Create .claude directory so settings.json can be written
mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.cursor"
mkdir -p "$HOME/.codex"

echo "[setup-ai] AI configuration added to base.yaml"
```

Key changes:
- Permissions now use per-agent native terms
- No `overrides` section
- Cursor permissions use native `Read(**)`, `Write(**)`

- [ ] **Step 2: Update E2E test suite**

Replace `e2e/suites/10-ai-config.sh`. Key changes to assertions:
- State JSON now stores native terms directly (same as before since state was always native)
- Cursor gets permissions from its own `permissions.cursor` entry instead of via override
- Test 3 (narrow AI scope) updates to use per-agent permissions format
- Test 8 (deny permissions) uses native terms
- Test 9 (codex) no longer has `overrides: {}`

```bash
#!/bin/bash
# e2e/suites/10-ai-config.sh — AI configuration E2E tests
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
bash "$FIXTURE_DIR/setup-ai.sh"

# Test 1: Apply with AI config
facet_apply work
echo "  apply with AI config exited cleanly"

# Verify state file has AI section with correct structure
assert_file_exists "$HOME/.facet/.state.json"
assert_json_field "$HOME/.facet/.state.json" '.profile' 'work'
assert_json_field "$HOME/.facet/.state.json" '.ai.permissions."claude-code".allow[0]' 'Read'
assert_json_field "$HOME/.facet/.state.json" '.ai.permissions."claude-code".allow[1]' 'Edit'
assert_json_field "$HOME/.facet/.state.json" '.ai.permissions."claude-code".allow[2]' 'Bash'
echo "  state.json has correct AI data"

# Verify Claude Code settings file written with permissions
assert_file_exists "$HOME/.claude/settings.json"
assert_file_contains "$HOME/.claude/settings.json" '"permissions"'
assert_file_contains "$HOME/.claude/settings.json" '"allow"'
assert_file_contains "$HOME/.claude/settings.json" '"Read"'
assert_file_contains "$HOME/.claude/settings.json" '"Edit"'
assert_file_contains "$HOME/.claude/settings.json" '"Bash"'
echo "  Claude Code permissions written correctly"

# Verify Cursor CLI config with its own permissions (Read(**), Write(**) — no Shell(*))
assert_file_exists "$HOME/.cursor/cli-config.json"
assert_file_contains "$HOME/.cursor/cli-config.json" '"permissions"'
assert_file_contains "$HOME/.cursor/cli-config.json" '"Read(**)"'
assert_file_contains "$HOME/.cursor/cli-config.json" '"Write(**)"'
assert_file_not_contains "$HOME/.cursor/cli-config.json" '"Shell(*)"'
echo "  Cursor permissions written correctly"

# Verify skills were installed via npx
assert_file_exists "$HOME/.mock-ai"
assert_file_contains "$HOME/.mock-ai" "npx skills add @vercel-labs/agent-skills"
assert_file_contains "$HOME/.mock-ai" "frontend-design"
echo "  skills installed via npx"

# Verify Claude MCP registered via claude CLI
assert_file_contains "$HOME/.mock-ai" "claude mcp add playwright"
assert_file_contains "$HOME/.mock-ai" "claude mcp add github"
echo "  Claude Code MCPs registered via CLI"

# Verify Cursor MCP written to file (playwright only — github is claude-code scoped)
assert_file_exists "$HOME/.cursor/mcp.json"
assert_file_contains "$HOME/.cursor/mcp.json" '"playwright"'
assert_file_not_contains "$HOME/.cursor/mcp.json" '"github"'
echo "  Cursor MCP file has playwright only (github scoped to claude-code)"

# Verify env var resolution in MCP (github MCP gets resolved secret_key)
assert_file_contains "$HOME/.mock-ai" "GITHUB_TOKEN=s3cret"
echo "  MCP env var resolved from .local.yaml"

# Test 2: Reapply same profile (idempotent)
: > "$HOME/.mock-ai"
facet_apply work
echo "  reapply exited cleanly"

# Verify settings files are still correct after idempotent reapply
assert_file_contains "$HOME/.claude/settings.json" '"Read"'
assert_file_contains "$HOME/.claude/settings.json" '"Edit"'
assert_file_contains "$HOME/.claude/settings.json" '"Bash"'
echo "  idempotent reapply: claude-code permissions still correct"

assert_file_contains "$HOME/.cursor/cli-config.json" '"Read(**)"'
assert_file_contains "$HOME/.cursor/cli-config.json" '"Write(**)"'
echo "  idempotent reapply: cursor permissions still correct"

assert_file_contains "$HOME/.cursor/mcp.json" '"playwright"'
echo "  idempotent reapply: cursor MCPs still correct"

# Test 3: Edit same profile to narrow AI scope to claude-code only
cat > "$HOME/dotfiles/profiles/work.yaml" << 'YAML'
extends: base

vars:
  git:
    email: sarah@acme.com

packages:
  - name: docker
    install: brew install docker
  - name: node
    install:
      macos: brew install node
      linux: sudo apt-get install -y nodejs

configs:
  ~/.gitconfig: configs/work/.gitconfig
  ~/.npmrc: configs/work/.npmrc

ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
YAML

: > "$HOME/.mock-ai"
facet_apply work
echo "  same-profile AI narrowing exited cleanly"

assert_file_contains "$HOME/.mock-ai" "npx skills remove frontend-design -a cursor -y"
echo "  cursor skill removed on same-profile narrowing"

assert_file_contains "$HOME/.cursor/cli-config.json" '"allow": []'
assert_file_not_contains "$HOME/.cursor/cli-config.json" '"Read(**)"'
echo "  cursor permissions cleared on same-profile narrowing"

assert_file_not_contains "$HOME/.cursor/mcp.json" '"playwright"'
echo "  cursor MCP removed on same-profile narrowing"

# Test 4: Switch to minimal profile — skills should be orphan-removed
: > "$HOME/.mock-ai"
facet_apply minimal
echo "  apply minimal profile exited cleanly"

# The previous skill (frontend-design) should have been removed
assert_file_contains "$HOME/.mock-ai" "npx skills remove frontend-design"
echo "  orphan skill removed on profile switch"

# MCPs should have been removed via unapply (profile switch)
assert_file_contains "$HOME/.mock-ai" "claude mcp remove playwright"
assert_file_contains "$HOME/.mock-ai" "claude mcp remove github"
echo "  orphan MCPs removed on profile switch"

# Minimal profile only has claude-code, so cursor settings should have been cleaned up
assert_json_field "$HOME/.facet/.state.json" '.profile' 'minimal'
echo "  state shows minimal profile"

# Test 5: Dry-run with AI — should show preview without side effects
: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --dry-run minimal > "$HOME/.dryrun-output" 2>&1
assert_file_contains "$HOME/.dryrun-output" "AI configuration"
assert_file_contains "$HOME/.dryrun-output" "No changes were made"
# mock-ai log should be empty (no AI commands executed during dry-run)
if [ -s "$HOME/.mock-ai" ]; then
    echo "  ASSERT FAIL: dry-run should not execute AI commands"
    cat "$HOME/.mock-ai"
    exit 1
fi
echo "  dry-run shows AI preview without side effects"

# Test 6: Force apply triggers full unapply+reapply
: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --force minimal
echo "  force apply exited cleanly"
assert_json_field "$HOME/.facet/.state.json" '.profile' 'minimal'
echo "  force reapply succeeded"

# Test 7: Status shows AI information
facet_status > "$HOME/.status-output" 2>&1
assert_file_contains "$HOME/.status-output" "AI"
assert_file_contains "$HOME/.status-output" "claude-code"
echo "  status shows AI section"

# Test 8: Deny permissions
cat > "$HOME/dotfiles/profiles/deny-test.yaml" << 'YAML'
extends: base

ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read, Edit]
      deny: [Bash]
YAML

: > "$HOME/.mock-ai"
facet_apply deny-test
assert_file_exists "$HOME/.claude/settings.json"
assert_json_field "$HOME/.facet/.state.json" '.ai.permissions."claude-code".deny[0]' 'Bash'
echo "  deny permissions applied correctly"

# Test 9: Codex agent receives MCPs
cat > "$HOME/dotfiles/profiles/codex-test.yaml" << 'YAML'
extends: base

ai:
  agents: [codex]
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
YAML

: > "$HOME/.mock-ai"
facet_apply codex-test
assert_file_exists "$HOME/.codex/config.toml"
assert_file_contains "$HOME/.codex/config.toml" "[mcp_servers.playwright]"
echo "  codex agent MCP registered in config.toml"
```

- [ ] **Step 3: Run E2E tests**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/run.sh`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add e2e/fixtures/setup-ai.sh e2e/suites/10-ai-config.sh
git commit -m "refactor: update E2E tests for per-agent native permissions

Fixtures and assertions now use per-agent permissions maps with native
terms instead of canonical terms with overrides."
```

---

### Task 9: Run full test suite and verify

- [ ] **Step 1: Run all unit tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: PASS (all tests green)

- [ ] **Step 2: Run E2E tests**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/run.sh`
Expected: PASS (all suites green)

- [ ] **Step 3: Verify no stale references**

Search for any remaining references to canonical terms or overrides:
- `grep -r "Overrides\|AIOverride\|mergeOverrides\|PermissionMapper\|MapToNative\|MapAllToNative\|permMapper\|canonical" internal/ main.go`
Expected: No matches (all references cleaned up)

- [ ] **Step 4: Commit any fixups if needed**
