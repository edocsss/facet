# AI Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add AI coding tool configuration (permissions, skills, MCPs) to facet profiles, supporting Claude Code, Cursor, and Codex.

**Architecture:** A new `internal/ai/` package handles AI-specific logic (resolution, provider abstraction, orchestration). The `profile/` package is extended with AI types and merge logic. The `app/` layer calls `ai.Resolve()` then delegates to an `AIOrchestrator` interface. All external tool interaction (npx skills, claude CLI) goes through injectable interfaces.

**Tech Stack:** Go, gopkg.in/yaml.v3, encoding/json, os/exec (via ShellRunner), testify

**Design Spec:** `docs/superpowers/specs/2026-03-19-ai-configuration-design.md`

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `internal/profile/ai_types.go` | AI-related YAML types: `AIConfig`, `PermissionsConfig`, `SkillEntry`, `MCPEntry`, `AIOverride` |
| `internal/profile/ai_types_test.go` | YAML unmarshaling tests for AI types |
| `internal/profile/ai_merger.go` | `mergeAI()`, `mergeSkills()`, `mergeMCPs()`, `mergePermissions()`, `mergeOverrides()` |
| `internal/profile/ai_merger_test.go` | AI merge logic tests |
| `internal/ai/types.go` | Resolved types: `ResolvedPermissions`, `ResolvedSkill`, `ResolvedMCP`, `EffectiveAgentConfig`, `EffectiveAIConfig` |
| `internal/ai/interfaces.go` | Internal interfaces: `AgentProvider`, `PermissionMapper`, `SkillsManager` |
| `internal/ai/resolve.go` | `Resolve(*profile.AIConfig) EffectiveAIConfig` — pure function |
| `internal/ai/resolve_test.go` | Resolution tests |
| `internal/ai/permission_mapper.go` | `DefaultPermissionMapper` with mapping table |
| `internal/ai/permission_mapper_test.go` | Mapping tests |
| `internal/ai/claude_code_provider.go` | `ClaudeCodeProvider` — reads/writes `.claude/settings.json`, shells out for MCPs |
| `internal/ai/claude_code_provider_test.go` | Provider tests with `t.TempDir()` |
| `internal/ai/cursor_provider.go` | `CursorProvider` — reads/writes `.cursor/settings.json` and `.cursor/mcp.json` |
| `internal/ai/cursor_provider_test.go` | Provider tests with `t.TempDir()` |
| `internal/ai/codex_provider.go` | `CodexProvider` — reads/writes Codex config files |
| `internal/ai/codex_provider_test.go` | Provider tests with `t.TempDir()` |
| `internal/ai/skills_manager.go` | `NPXSkillsManager` — wraps `npx skills` CLI via `CommandRunner` |
| `internal/ai/skills_manager_test.go` | Tests with mock `CommandRunner` |
| `internal/ai/orchestrator.go` | `Orchestrator` — coordinates apply/unapply across all providers |
| `internal/ai/orchestrator_test.go` | Orchestrator tests with all mocks |
| `internal/ai/test_helpers_test.go` | Shared mock types for AI package tests (`mockRunner`, `mockProvider`, etc.) |
| `internal/ai/state.go` | `AIState`, `SkillState`, `MCPState`, `PermissionState` types |
| `internal/ai/jsonutil.go` | Shared `readJSONFile`/`writeJSONFile` helpers |

### Modified files

| File | Changes |
|---|---|
| `internal/profile/types.go` | Add `AI *AIConfig` field to `FacetConfig` |
| `internal/profile/merger.go` | Call `mergeAI()` in `Merge()` |
| `internal/profile/resolver.go` | Extend `Resolve()` to walk `ai.mcps[].env`, `ai.mcps[].command`, `ai.mcps[].args` |
| `internal/profile/resolver_test.go` | Tests for AI variable resolution |
| `internal/app/interfaces.go` | Add `AIOrchestrator` interface |
| `internal/app/state.go` | Add `AI *ai.AIState` field to `ApplyState` |
| `internal/app/app.go` | Add `AIOrchestrator` to `Deps` and `App` |
| `internal/app/apply.go` | Integrate AI steps 11-13 after config deployment, AI unapply in unapply flow |
| `internal/app/report.go` | Extend `printDryRun()`, `printApplyReport()` for AI section |
| `internal/app/status.go` | Extend `Status()` to display AI state |
| `main.go` | Wire AI dependencies |

---

## Chunk 1: Profile Schema & Merge (Tasks 1-4)

### Task 1: AI types in profile package

**Files:**
- Create: `internal/profile/ai_types.go`
- Create: `internal/profile/ai_types_test.go`
- Modify: `internal/profile/types.go:15-20`

- [ ] **Step 1: Write YAML unmarshaling test for AIConfig**

```go
// internal/profile/ai_types_test.go
package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAIConfig_UnmarshalYAML_Full(t *testing.T) {
	input := `
agents: [claude-code, cursor, codex]
permissions:
  allow: [read, edit, bash]
  deny: [computer-use]
skills:
  - source: vercel-labs/agent-skills
    skills: [frontend-design, writing-plans]
  - source: anthropics/claude-plugins-official
    skills: [superpowers]
    agents: [claude-code]
mcps:
  - name: playwright
    command: npx
    args: ["@anthropic/mcp-playwright"]
  - name: github
    command: npx
    args: ["@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: "${facet:github.token}"
    agents: [claude-code, cursor]
overrides:
  cursor:
    permissions:
      allow: [read, edit]
      deny: []
`
	var cfg AIConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"claude-code", "cursor", "codex"}, cfg.Agents)
	assert.Equal(t, []string{"read", "edit", "bash"}, cfg.Permissions.Allow)
	assert.Equal(t, []string{"computer-use"}, cfg.Permissions.Deny)

	require.Len(t, cfg.Skills, 2)
	assert.Equal(t, "vercel-labs/agent-skills", cfg.Skills[0].Source)
	assert.Equal(t, []string{"frontend-design", "writing-plans"}, cfg.Skills[0].Skills)
	assert.Nil(t, cfg.Skills[0].Agents)
	assert.Equal(t, []string{"claude-code"}, cfg.Skills[1].Agents)

	require.Len(t, cfg.MCPs, 2)
	assert.Equal(t, "playwright", cfg.MCPs[0].Name)
	assert.Equal(t, "npx", cfg.MCPs[0].Command)
	assert.Equal(t, []string{"@anthropic/mcp-playwright"}, cfg.MCPs[0].Args)
	assert.Equal(t, "${facet:github.token}", cfg.MCPs[1].Env["GITHUB_TOKEN"])
	assert.Equal(t, []string{"claude-code", "cursor"}, cfg.MCPs[1].Agents)

	require.Contains(t, cfg.Overrides, "cursor")
	assert.Equal(t, []string{"read", "edit"}, cfg.Overrides["cursor"].Permissions.Allow)
	assert.Empty(t, cfg.Overrides["cursor"].Permissions.Deny)
}

func TestAIConfig_UnmarshalYAML_Empty(t *testing.T) {
	input := `agents: [claude-code]`
	var cfg AIConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"claude-code"}, cfg.Agents)
	assert.Nil(t, cfg.Permissions)
	assert.Nil(t, cfg.Skills)
	assert.Nil(t, cfg.MCPs)
	assert.Nil(t, cfg.Overrides)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestAIConfig -v`
Expected: FAIL — `AIConfig` type not defined

- [ ] **Step 3: Create AI types**

```go
// internal/profile/ai_types.go
package profile

// AIConfig represents the "ai" section of a profile.
type AIConfig struct {
	Agents      []string               `yaml:"agents"`
	Permissions *PermissionsConfig     `yaml:"permissions,omitempty"`
	Skills      []SkillEntry           `yaml:"skills,omitempty"`
	MCPs        []MCPEntry             `yaml:"mcps,omitempty"`
	Overrides   map[string]*AIOverride `yaml:"overrides,omitempty"`
}

// AIOverride holds per-agent overrides within the AI section.
type AIOverride struct {
	Permissions *PermissionsConfig `yaml:"permissions,omitempty"`
}

// PermissionsConfig holds allow/deny permission lists using canonical terms.
type PermissionsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// SkillEntry declares a skill source and which skills to install.
type SkillEntry struct {
	Source string   `yaml:"source"`
	Skills []string `yaml:"skills"`
	Agents []string `yaml:"agents,omitempty"`
}

// MCPEntry declares an MCP server to register.
type MCPEntry struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Agents  []string          `yaml:"agents,omitempty"`
}
```

- [ ] **Step 4: Add AI field to FacetConfig**

In `internal/profile/types.go`, add the `AI` field to `FacetConfig`:

```go
type FacetConfig struct {
	Extends  string            `yaml:"extends,omitempty"`
	Vars     map[string]any    `yaml:"vars,omitempty"`
	Packages []PackageEntry    `yaml:"packages,omitempty"`
	Configs  map[string]string `yaml:"configs,omitempty"`
	AI       *AIConfig         `yaml:"ai,omitempty"`
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestAIConfig -v`
Expected: PASS

- [ ] **Step 6: Write test for FacetConfig with AI section**

Add to `internal/profile/ai_types_test.go`:

```go
func TestFacetConfig_WithAI(t *testing.T) {
	input := `
vars:
  git_name: Sarah
packages:
  - name: git
    install: brew install git
configs:
  ~/.gitconfig: configs/.gitconfig
ai:
  agents: [claude-code]
  permissions:
    allow: [read, edit]
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "Sarah", cfg.Vars["git_name"])
	require.NotNil(t, cfg.AI)
	assert.Equal(t, []string{"claude-code"}, cfg.AI.Agents)
	assert.Equal(t, []string{"read", "edit"}, cfg.AI.Permissions.Allow)
}

func TestFacetConfig_WithoutAI(t *testing.T) {
	input := `
vars:
  git_name: Sarah
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	assert.Nil(t, cfg.AI)
}
```

- [ ] **Step 7: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestFacetConfig -v`
Expected: PASS

- [ ] **Step 8: Run all existing tests to verify no regressions**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/profile/ai_types.go internal/profile/ai_types_test.go internal/profile/types.go
git commit -m "feat(profile): add AI configuration types"
```

---

### Task 2: AI merge logic

**Files:**
- Create: `internal/profile/ai_merger.go`
- Create: `internal/profile/ai_merger_test.go`
- Modify: `internal/profile/merger.go:9-26`

- [ ] **Step 1: Write test for mergePermissions**

```go
// internal/profile/ai_merger_test.go
package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergePermissions_OverlayWins(t *testing.T) {
	base := &PermissionsConfig{
		Allow: []string{"read", "edit", "bash"},
		Deny:  []string{"computer-use"},
	}
	overlay := &PermissionsConfig{
		Allow: []string{"read", "edit"},
		Deny:  []string{},
	}
	result := mergePermissions(base, overlay)
	assert.Equal(t, []string{"read", "edit"}, result.Allow)
	assert.Empty(t, result.Deny)
}

func TestMergePermissions_BaseOnly(t *testing.T) {
	base := &PermissionsConfig{
		Allow: []string{"read", "edit"},
		Deny:  []string{"bash"},
	}
	result := mergePermissions(base, nil)
	assert.Equal(t, []string{"read", "edit"}, result.Allow)
	assert.Equal(t, []string{"bash"}, result.Deny)
}

func TestMergePermissions_OverlayOnly(t *testing.T) {
	overlay := &PermissionsConfig{
		Allow: []string{"read"},
	}
	result := mergePermissions(nil, overlay)
	assert.Equal(t, []string{"read"}, result.Allow)
}

func TestMergePermissions_BothNil(t *testing.T) {
	result := mergePermissions(nil, nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergePermissions -v`
Expected: FAIL

- [ ] **Step 3: Implement mergePermissions**

```go
// internal/profile/ai_merger.go
package profile

// mergePermissions returns the overlay if present, otherwise the base. Last writer wins.
func mergePermissions(base, overlay *PermissionsConfig) *PermissionsConfig {
	if overlay != nil {
		return overlay
	}
	return base
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergePermissions -v`
Expected: PASS

- [ ] **Step 5: Write test for mergeSkills (tuple normalization + union)**

Add to `internal/profile/ai_merger_test.go`:

```go
func TestMergeSkills_UnionBySourceAndName(t *testing.T) {
	base := []SkillEntry{
		{Source: "vercel-labs/agent-skills", Skills: []string{"frontend-design", "writing-plans"}},
	}
	overlay := []SkillEntry{
		{Source: "vercel-labs/agent-skills", Skills: []string{"writing-plans", "code-review"}},
		{Source: "other/repo", Skills: []string{"tool-a"}},
	}
	result := mergeSkills(base, overlay)

	// Flatten to (source, skill) tuples for easy assertion
	type tuple struct{ source, skill string }
	var tuples []tuple
	for _, entry := range result {
		for _, s := range entry.Skills {
			tuples = append(tuples, tuple{entry.Source, s})
		}
	}

	assert.Contains(t, tuples, tuple{"vercel-labs/agent-skills", "frontend-design"})
	assert.Contains(t, tuples, tuple{"vercel-labs/agent-skills", "writing-plans"})
	assert.Contains(t, tuples, tuple{"vercel-labs/agent-skills", "code-review"})
	assert.Contains(t, tuples, tuple{"other/repo", "tool-a"})
}

func TestMergeSkills_OverlayAgentsWin(t *testing.T) {
	base := []SkillEntry{
		{Source: "repo/a", Skills: []string{"skill-x"}, Agents: []string{"claude-code"}},
	}
	overlay := []SkillEntry{
		{Source: "repo/a", Skills: []string{"skill-x"}, Agents: []string{"cursor"}},
	}
	result := mergeSkills(base, overlay)

	// Find skill-x entry — overlay agents should win
	for _, entry := range result {
		for _, s := range entry.Skills {
			if s == "skill-x" {
				assert.Equal(t, []string{"cursor"}, entry.Agents)
				return
			}
		}
	}
	t.Fatal("skill-x not found in result")
}

func TestMergeSkills_BothNil(t *testing.T) {
	result := mergeSkills(nil, nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeSkills -v`
Expected: FAIL

- [ ] **Step 7: Implement mergeSkills**

Add to `internal/profile/ai_merger.go`:

```go
// skillTuple represents a normalized (source, skill_name) pair with metadata.
type skillTuple struct {
	Source string
	Skill  string
	Agents []string
}

// mergeSkills normalizes skill entries to (source, skill_name) tuples, unions them,
// and re-groups by source. Last writer wins on conflict (same source + skill name).
func mergeSkills(base, overlay []SkillEntry) []SkillEntry {
	if base == nil && overlay == nil {
		return nil
	}

	// Normalize to tuples, preserving insertion order
	seen := make(map[string]int) // "source\x00skill" -> index in tuples
	var tuples []skillTuple

	for _, entries := range [][]SkillEntry{base, overlay} {
		for _, entry := range entries {
			for _, skill := range entry.Skills {
				key := entry.Source + "\x00" + skill
				if idx, exists := seen[key]; exists {
					tuples[idx] = skillTuple{Source: entry.Source, Skill: skill, Agents: entry.Agents}
				} else {
					seen[key] = len(tuples)
					tuples = append(tuples, skillTuple{Source: entry.Source, Skill: skill, Agents: entry.Agents})
				}
			}
		}
	}

	// Re-group by source, preserving order
	type sourceGroup struct {
		source string
		skills []string
		agents []string // agents from first tuple in group (all tuples from same source+overlay share agents)
	}
	sourceOrder := make([]string, 0)
	groups := make(map[string]*SkillEntry)

	for _, t := range tuples {
		if _, exists := groups[t.Source]; !exists {
			sourceOrder = append(sourceOrder, t.Source)
			groups[t.Source] = &SkillEntry{Source: t.Source, Agents: t.Agents}
		}
		groups[t.Source].Skills = append(groups[t.Source].Skills, t.Skill)
		// Use the latest agents for this source (last tuple's agents)
		if t.Agents != nil {
			groups[t.Source].Agents = t.Agents
		}
	}

	result := make([]SkillEntry, 0, len(sourceOrder))
	for _, src := range sourceOrder {
		result = append(result, *groups[src])
	}
	return result
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeSkills -v`
Expected: PASS

- [ ] **Step 9: Write test for mergeMCPs**

Add to `internal/profile/ai_merger_test.go`:

```go
func TestMergeMCPs_UnionByName(t *testing.T) {
	base := []MCPEntry{
		{Name: "playwright", Command: "npx", Args: []string{"@anthropic/mcp-playwright"}},
		{Name: "github", Command: "npx", Args: []string{"@modelcontextprotocol/server-github"}},
	}
	overlay := []MCPEntry{
		{Name: "github", Command: "npx", Args: []string{"@modelcontextprotocol/server-github"}, Agents: []string{"claude-code"}},
		{Name: "slack", Command: "npx", Args: []string{"@modelcontextprotocol/server-slack"}},
	}
	result := mergeMCPs(base, overlay)

	assert.Len(t, result, 3)
	// Find github — overlay should win
	for _, mcp := range result {
		if mcp.Name == "github" {
			assert.Equal(t, []string{"claude-code"}, mcp.Agents)
		}
	}
}

func TestMergeMCPs_BothNil(t *testing.T) {
	result := mergeMCPs(nil, nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeMCPs -v`
Expected: FAIL

- [ ] **Step 11: Implement mergeMCPs**

Add to `internal/profile/ai_merger.go`:

```go
// mergeMCPs unions MCP entries by name. Last writer wins on conflict.
func mergeMCPs(base, overlay []MCPEntry) []MCPEntry {
	if base == nil && overlay == nil {
		return nil
	}

	seen := make(map[string]int) // name -> index
	var result []MCPEntry

	for _, entries := range [][]MCPEntry{base, overlay} {
		for _, entry := range entries {
			if idx, exists := seen[entry.Name]; exists {
				result[idx] = entry
			} else {
				seen[entry.Name] = len(result)
				result = append(result, entry)
			}
		}
	}
	return result
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeMCPs -v`
Expected: PASS

- [ ] **Step 13: Write test for mergeOverrides**

Add to `internal/profile/ai_merger_test.go`:

```go
func TestMergeOverrides_DeepMerge(t *testing.T) {
	base := map[string]*AIOverride{
		"cursor": {Permissions: &PermissionsConfig{Allow: []string{"read"}, Deny: []string{"bash"}}},
	}
	overlay := map[string]*AIOverride{
		"cursor": {Permissions: &PermissionsConfig{Allow: []string{"read", "edit"}}},
		"codex":  {Permissions: &PermissionsConfig{Allow: []string{"read"}}},
	}
	result := mergeOverrides(base, overlay)

	require.Contains(t, result, "cursor")
	assert.Equal(t, []string{"read", "edit"}, result["cursor"].Permissions.Allow)
	require.Contains(t, result, "codex")
	assert.Equal(t, []string{"read"}, result["codex"].Permissions.Allow)
}

func TestMergeOverrides_BothNil(t *testing.T) {
	result := mergeOverrides(nil, nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 14: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeOverrides -v`
Expected: FAIL

- [ ] **Step 15: Implement mergeOverrides**

Add to `internal/profile/ai_merger.go`:

```go
// mergeOverrides deep-merges per-agent overrides. Overlay wins per agent.
func mergeOverrides(base, overlay map[string]*AIOverride) map[string]*AIOverride {
	if base == nil && overlay == nil {
		return nil
	}

	result := make(map[string]*AIOverride)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if existing, ok := result[k]; ok {
			result[k] = &AIOverride{
				Permissions: mergePermissions(existing.Permissions, v.Permissions),
			}
		} else {
			result[k] = v
		}
	}
	return result
}
```

- [ ] **Step 16: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeOverrides -v`
Expected: PASS

- [ ] **Step 17: Write test for mergeAI (top-level orchestration)**

Add to `internal/profile/ai_merger_test.go`:

```go
func TestMergeAI_Full(t *testing.T) {
	base := &AIConfig{
		Agents:      []string{"claude-code", "cursor"},
		Permissions: &PermissionsConfig{Allow: []string{"read", "edit", "bash"}},
		Skills:      []SkillEntry{{Source: "repo/a", Skills: []string{"skill-1"}}},
		MCPs:        []MCPEntry{{Name: "playwright", Command: "npx"}},
	}
	overlay := &AIConfig{
		Agents:      []string{"claude-code", "cursor", "codex"},
		Permissions: &PermissionsConfig{Allow: []string{"read", "edit"}},
		Skills:      []SkillEntry{{Source: "repo/b", Skills: []string{"skill-2"}}},
		MCPs:        []MCPEntry{{Name: "github", Command: "npx"}},
		Overrides: map[string]*AIOverride{
			"codex": {Permissions: &PermissionsConfig{Allow: []string{"read"}}},
		},
	}
	result := mergeAI(base, overlay)

	require.NotNil(t, result)
	assert.Equal(t, []string{"claude-code", "cursor", "codex"}, result.Agents)
	assert.Equal(t, []string{"read", "edit"}, result.Permissions.Allow)
	assert.Len(t, result.Skills, 2) // skill-1 and skill-2 from different sources
	assert.Len(t, result.MCPs, 2)   // playwright and github
	require.Contains(t, result.Overrides, "codex")
}

func TestMergeAI_BaseNil(t *testing.T) {
	overlay := &AIConfig{Agents: []string{"claude-code"}}
	result := mergeAI(nil, overlay)
	assert.Equal(t, []string{"claude-code"}, result.Agents)
}

func TestMergeAI_OverlayNil(t *testing.T) {
	base := &AIConfig{Agents: []string{"claude-code"}}
	result := mergeAI(base, nil)
	assert.Equal(t, []string{"claude-code"}, result.Agents)
}

func TestMergeAI_BothNil(t *testing.T) {
	result := mergeAI(nil, nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 18: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeAI -v`
Expected: FAIL

- [ ] **Step 19: Implement mergeAI**

Add to `internal/profile/ai_merger.go`:

```go
// mergeAI merges two AI configurations. Overlay wins on conflicts.
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

	result.Permissions = mergePermissions(base.Permissions, overlay.Permissions)
	result.Skills = mergeSkills(base.Skills, overlay.Skills)
	result.MCPs = mergeMCPs(base.MCPs, overlay.MCPs)
	result.Overrides = mergeOverrides(base.Overrides, overlay.Overrides)

	return result
}
```

- [ ] **Step 20: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestMergeAI -v`
Expected: PASS

- [ ] **Step 21: Integrate mergeAI into Merge()**

In `internal/profile/merger.go`, add the AI merge call inside `Merge()` after `mergeConfigs()`:

```go
result.AI = mergeAI(base.AI, overlay.AI)
```

- [ ] **Step 22: Run all profile tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All PASS

- [ ] **Step 23: Commit**

```bash
git add internal/profile/ai_merger.go internal/profile/ai_merger_test.go internal/profile/merger.go
git commit -m "feat(profile): add AI config merge logic"
```

---

### Task 3: AI variable resolution

**Files:**
- Modify: `internal/profile/resolver.go:14-39`
- Modify: `internal/profile/resolver_test.go`

- [ ] **Step 1: Write test for AI variable resolution in MCP env**

Add to `internal/profile/resolver_test.go`:

```go
func TestResolve_AI_MCPEnv(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"github": map[string]any{
				"token": "ghp_abc123",
			},
		},
		AI: &AIConfig{
			Agents: []string{"claude-code"},
			MCPs: []MCPEntry{
				{
					Name:    "github",
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-github"},
					Env:     map[string]string{"GITHUB_TOKEN": "${facet:github.token}"},
				},
			},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "ghp_abc123", resolved.AI.MCPs[0].Env["GITHUB_TOKEN"])
}

func TestResolve_AI_MCPArgs(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"mcp_version": "1.0.0",
		},
		AI: &AIConfig{
			Agents: []string{"claude-code"},
			MCPs: []MCPEntry{
				{
					Name:    "test",
					Command: "${facet:mcp_version}",
					Args:    []string{"--version=${facet:mcp_version}"},
				},
			},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resolved.AI.MCPs[0].Command)
	assert.Equal(t, "--version=1.0.0", resolved.AI.MCPs[0].Args[0])
}

func TestResolve_AI_UndefinedVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{},
		AI: &AIConfig{
			Agents: []string{"claude-code"},
			MCPs: []MCPEntry{
				{
					Name: "test",
					Env:  map[string]string{"KEY": "${facet:undefined}"},
				},
			},
		},
	}

	_, err := Resolve(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undefined")
}

func TestResolve_AI_Nil(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"x": "y"},
	}
	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Nil(t, resolved.AI)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestResolve_AI -v`
Expected: FAIL — AI section not walked

- [ ] **Step 3: Extend Resolve() to walk AI section**

In `internal/profile/resolver.go`, add a helper and call it at the end of `Resolve()`:

```go
// resolveAI resolves ${facet:...} variables in the AI config section.
func resolveAI(ai *AIConfig, vars map[string]any) (*AIConfig, error) {
	if ai == nil {
		return nil, nil
	}

	result := *ai // shallow copy — deep copy mutable fields
	result.Agents = append([]string{}, ai.Agents...)
	if ai.Skills != nil {
		result.Skills = make([]SkillEntry, len(ai.Skills))
		copy(result.Skills, ai.Skills)
	}
	result.MCPs = make([]MCPEntry, len(ai.MCPs))

	for i, mcp := range ai.MCPs {
		resolved := mcp // copy

		// Resolve command
		if mcp.Command != "" {
			cmd, err := substituteVars(mcp.Command, vars)
			if err != nil {
				return nil, fmt.Errorf("ai.mcps[%d].command: %w", i, err)
			}
			resolved.Command = cmd
		}

		// Resolve args
		if len(mcp.Args) > 0 {
			resolved.Args = make([]string, len(mcp.Args))
			for j, arg := range mcp.Args {
				a, err := substituteVars(arg, vars)
				if err != nil {
					return nil, fmt.Errorf("ai.mcps[%d].args[%d]: %w", i, j, err)
				}
				resolved.Args[j] = a
			}
		}

		// Resolve env values
		if len(mcp.Env) > 0 {
			resolved.Env = make(map[string]string, len(mcp.Env))
			for k, v := range mcp.Env {
				val, err := substituteVars(v, vars)
				if err != nil {
					return nil, fmt.Errorf("ai.mcps[%d].env[%s]: %w", i, k, err)
				}
				resolved.Env[k] = val
			}
		}

		result.MCPs[i] = resolved
	}

	return &result, nil
}
```

Then add to the end of `Resolve()` (before the return):

```go
resolvedAI, err := resolveAI(cfg.AI, cfg.Vars)
if err != nil {
    return nil, err
}
result.AI = resolvedAI
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestResolve -v`
Expected: All PASS

- [ ] **Step 5: Run all profile tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/profile/resolver.go internal/profile/resolver_test.go
git commit -m "feat(profile): extend variable resolution to AI config section"
```

---

### Task 3b: AI config validation

**Files:**
- Modify: `internal/profile/loader.go` (ValidateProfile function)
- Modify: `internal/profile/loader_test.go`

- [ ] **Step 1: Write validation tests**

Add to `internal/profile/loader_test.go`:

```go
func TestValidateProfile_AIEmptyAgents(t *testing.T) {
	cfg := &FacetConfig{
		Extends: "base",
		AI:      &AIConfig{Agents: []string{}},
	}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agents")
}

func TestValidateProfile_AIOverrideUnknownAgent(t *testing.T) {
	cfg := &FacetConfig{
		Extends: "base",
		AI: &AIConfig{
			Agents: []string{"claude-code"},
			Overrides: map[string]*AIOverride{
				"unknown-agent": {Permissions: &PermissionsConfig{Allow: []string{"read"}}},
			},
		},
	}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-agent")
}

func TestValidateProfile_AINoSection(t *testing.T) {
	cfg := &FacetConfig{Extends: "base"}
	err := ValidateProfile(cfg)
	assert.NoError(t, err) // AI is optional
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestValidateProfile_AI -v`
Expected: FAIL

- [ ] **Step 3: Add validation logic**

In `internal/profile/loader.go`, extend `ValidateProfile()`:

```go
// Inside ValidateProfile(), after existing checks:
if cfg.AI != nil {
	if len(cfg.AI.Agents) == 0 {
		return fmt.Errorf("ai.agents must not be empty when ai section is present")
	}

	// Validate override keys reference declared agents
	agentSet := make(map[string]bool, len(cfg.AI.Agents))
	for _, a := range cfg.AI.Agents {
		agentSet[a] = true
	}
	for agentName := range cfg.AI.Overrides {
		if !agentSet[agentName] {
			return fmt.Errorf("ai.overrides references undeclared agent %q (declared: %v)", agentName, cfg.AI.Agents)
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/profile/ -run TestValidateProfile -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/profile/loader.go internal/profile/loader_test.go
git commit -m "feat(profile): add validation for AI config section"
```

---

## Chunk 2: AI Core Types, Resolution & Mapping (Tasks 4-6)

### Task 4: AI resolved types and interfaces

**Files:**
- Create: `internal/ai/types.go`
- Create: `internal/ai/interfaces.go`
- Create: `internal/ai/state.go`

- [ ] **Step 1: Create AI types**

```go
// internal/ai/types.go
package ai

import "facet/internal/profile"

// ResolvedPermissions holds permissions in canonical terms.
type ResolvedPermissions struct {
	Allow []string
	Deny  []string
}

// ResolvedSkill identifies a single skill to install.
type ResolvedSkill struct {
	Source string
	Name   string
}

// ResolvedMCP holds a fully resolved MCP configuration.
type ResolvedMCP struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// EffectiveAgentConfig holds the fully resolved config for a single agent.
type EffectiveAgentConfig struct {
	Permissions ResolvedPermissions
	Skills      []ResolvedSkill
	MCPs        []ResolvedMCP
}

// EffectiveAIConfig maps agent name to its resolved configuration.
type EffectiveAIConfig map[string]EffectiveAgentConfig

```

- [ ] **Step 2: Create AI interfaces**

```go
// internal/ai/interfaces.go
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

// PermissionMapper translates canonical permission terms to agent-native terms.
type PermissionMapper interface {
	MapToNative(canonical string, agent string) (string, error)
	MapAllToNative(canonical []string, agent string) ([]string, []string)
}

// SkillsManager manages skill installation/removal via external CLI.
type SkillsManager interface {
	Install(source string, skills []string, agents []string) error
	Remove(skills []string, agents []string) error
}

// CommandRunner executes shell commands. Defined here (not imported from
// packages/) to avoid cross-domain coupling. The concrete ShellRunner
// from packages/ satisfies this interface — wired in main.go.
type CommandRunner interface {
	Run(command string) error
}
```

- [ ] **Step 3: Create AI state types**

```go
// internal/ai/state.go
package ai

// AIState tracks facet-managed AI configuration in .state.json.
type AIState struct {
	Skills      []SkillState               `json:"skills"`
	MCPs        []MCPState                 `json:"mcps"`
	Permissions map[string]PermissionState `json:"permissions"`
}

// SkillState records a managed skill and which agents it was installed for.
type SkillState struct {
	Source string   `json:"source"`
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
}

// MCPState records a managed MCP and which agents it was registered for.
type MCPState struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
}

// PermissionState records agent-native permission terms that were applied.
type PermissionState struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/edocsss/aec/src/facet && go build ./internal/ai/`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/ai/types.go internal/ai/interfaces.go internal/ai/state.go
git commit -m "feat(ai): add core types, interfaces, and state structs"
```

---

### Task 5: Effective resolution (Resolve function)

**Files:**
- Create: `internal/ai/resolve.go`
- Create: `internal/ai/resolve_test.go`

- [ ] **Step 1: Write test for basic resolution (no overrides)**

```go
// internal/ai/resolve_test.go
package ai

import (
	"facet/internal/profile"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_BasicResolution(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents:      []string{"claude-code", "cursor"},
		Permissions: &profile.PermissionsConfig{Allow: []string{"read", "edit"}, Deny: []string{"bash"}},
		Skills: []profile.SkillEntry{
			{Source: "repo/a", Skills: []string{"skill-1", "skill-2"}},
		},
		MCPs: []profile.MCPEntry{
			{Name: "playwright", Command: "npx", Args: []string{"@anthropic/mcp-playwright"}},
		},
	}

	result := Resolve(cfg)

	require.Len(t, result, 2)

	// Both agents get the same shared config
	for _, agent := range []string{"claude-code", "cursor"} {
		agentCfg, ok := result[agent]
		require.True(t, ok, "missing config for %s", agent)
		assert.Equal(t, []string{"read", "edit"}, agentCfg.Permissions.Allow)
		assert.Equal(t, []string{"bash"}, agentCfg.Permissions.Deny)
		assert.Len(t, agentCfg.Skills, 2)
		assert.Equal(t, "skill-1", agentCfg.Skills[0].Name)
		assert.Equal(t, "repo/a", agentCfg.Skills[0].Source)
		assert.Len(t, agentCfg.MCPs, 1)
		assert.Equal(t, "playwright", agentCfg.MCPs[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestResolve_Basic -v`
Expected: FAIL

- [ ] **Step 3: Implement Resolve**

```go
// internal/ai/resolve.go
package ai

import "facet/internal/profile"

// Resolve takes a merged AIConfig and produces per-agent effective configs.
// Permissions are returned in canonical terms (not mapped to native).
func Resolve(cfg *profile.AIConfig) EffectiveAIConfig {
	if cfg == nil {
		return nil
	}

	result := make(EffectiveAIConfig, len(cfg.Agents))

	for _, agent := range cfg.Agents {
		agentCfg := EffectiveAgentConfig{}

		// Start with shared permissions
		if cfg.Permissions != nil {
			agentCfg.Permissions = ResolvedPermissions{
				Allow: cfg.Permissions.Allow,
				Deny:  cfg.Permissions.Deny,
			}
		}

		// Filter skills by agents list
		for _, entry := range cfg.Skills {
			if !agentIncluded(agent, entry.Agents, cfg.Agents) {
				continue
			}
			for _, skill := range entry.Skills {
				agentCfg.Skills = append(agentCfg.Skills, ResolvedSkill{
					Source: entry.Source,
					Name:   skill,
				})
			}
		}

		// Filter MCPs by agents list
		for _, mcp := range cfg.MCPs {
			if !agentIncluded(agent, mcp.Agents, cfg.Agents) {
				continue
			}
			agentCfg.MCPs = append(agentCfg.MCPs, ResolvedMCP{
				Name:    mcp.Name,
				Command: mcp.Command,
				Args:    mcp.Args,
				Env:     mcp.Env,
			})
		}

		// Apply per-agent permission overrides
		if cfg.Overrides != nil {
			if override, ok := cfg.Overrides[agent]; ok && override.Permissions != nil {
				agentCfg.Permissions = ResolvedPermissions{
					Allow: override.Permissions.Allow,
					Deny:  override.Permissions.Deny,
				}
			}
		}

		result[agent] = agentCfg
	}

	return result
}

// agentIncluded checks if an agent is targeted by an item's agents list.
// If the item's agents list is empty, it defaults to all agents.
func agentIncluded(agent string, itemAgents, allAgents []string) bool {
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

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestResolve_Basic -v`
Expected: PASS

- [ ] **Step 5: Write test for per-item agent filtering**

Add to `internal/ai/resolve_test.go`:

```go
func TestResolve_PerItemAgentFiltering(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor", "codex"},
		Skills: []profile.SkillEntry{
			{Source: "repo/a", Skills: []string{"skill-all"}},
			{Source: "repo/b", Skills: []string{"skill-cc-only"}, Agents: []string{"claude-code"}},
		},
		MCPs: []profile.MCPEntry{
			{Name: "mcp-all", Command: "npx"},
			{Name: "mcp-cc-cursor", Command: "npx", Agents: []string{"claude-code", "cursor"}},
		},
	}

	result := Resolve(cfg)

	// claude-code gets everything
	assert.Len(t, result["claude-code"].Skills, 2)
	assert.Len(t, result["claude-code"].MCPs, 2)

	// cursor gets skill-all and both MCPs
	assert.Len(t, result["cursor"].Skills, 1)
	assert.Equal(t, "skill-all", result["cursor"].Skills[0].Name)
	assert.Len(t, result["cursor"].MCPs, 2)

	// codex gets skill-all and mcp-all only
	assert.Len(t, result["codex"].Skills, 1)
	assert.Len(t, result["codex"].MCPs, 1)
	assert.Equal(t, "mcp-all", result["codex"].MCPs[0].Name)
}
```

- [ ] **Step 6: Run test**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestResolve_PerItem -v`
Expected: PASS

- [ ] **Step 7: Write test for per-agent permission overrides**

Add to `internal/ai/resolve_test.go`:

```go
func TestResolve_PermissionOverrides(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents:      []string{"claude-code", "cursor"},
		Permissions: &profile.PermissionsConfig{Allow: []string{"read", "edit", "bash"}},
		Overrides: map[string]*profile.AIOverride{
			"cursor": {Permissions: &profile.PermissionsConfig{Allow: []string{"read", "edit"}, Deny: []string{"bash"}}},
		},
	}

	result := Resolve(cfg)

	assert.Equal(t, []string{"read", "edit", "bash"}, result["claude-code"].Permissions.Allow)
	assert.Empty(t, result["claude-code"].Permissions.Deny)

	assert.Equal(t, []string{"read", "edit"}, result["cursor"].Permissions.Allow)
	assert.Equal(t, []string{"bash"}, result["cursor"].Permissions.Deny)
}

func TestResolve_Nil(t *testing.T) {
	result := Resolve(nil)
	assert.Nil(t, result)
}
```

- [ ] **Step 8: Run all resolve tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestResolve -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/ai/resolve.go internal/ai/resolve_test.go
git commit -m "feat(ai): implement effective resolution for per-agent configs"
```

---

### Task 6: Permission mapper

**Files:**
- Create: `internal/ai/permission_mapper.go`
- Create: `internal/ai/permission_mapper_test.go`

- [ ] **Step 1: Write test for canonical-to-native mapping**

```go
// internal/ai/permission_mapper_test.go
package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPermissionMapper_MapToNative_ClaudeCode(t *testing.T) {
	m := NewDefaultPermissionMapper()

	// Claude Code uses canonical terms directly (it IS the canonical source)
	native, err := m.MapToNative("bash", "claude-code")
	require.NoError(t, err)
	assert.Equal(t, "Bash", native)
}

func TestDefaultPermissionMapper_MapToNative_Cursor(t *testing.T) {
	m := NewDefaultPermissionMapper()

	native, err := m.MapToNative("bash", "cursor")
	require.NoError(t, err)
	assert.Equal(t, "terminal", native)
}

func TestDefaultPermissionMapper_MapToNative_Codex(t *testing.T) {
	m := NewDefaultPermissionMapper()

	native, err := m.MapToNative("bash", "codex")
	require.NoError(t, err)
	assert.Equal(t, "shell", native)
}

func TestDefaultPermissionMapper_MapToNative_Unknown(t *testing.T) {
	m := NewDefaultPermissionMapper()

	_, err := m.MapToNative("nonexistent-permission", "claude-code")
	assert.Error(t, err)
}

func TestDefaultPermissionMapper_MapAllToNative(t *testing.T) {
	m := NewDefaultPermissionMapper()

	mapped, warnings := m.MapAllToNative([]string{"read", "edit", "bash", "unknown-perm"}, "cursor")
	assert.Contains(t, mapped, "read")
	assert.Contains(t, mapped, "edit")
	assert.Contains(t, mapped, "terminal")
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "unknown-perm")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestDefaultPermissionMapper -v`
Expected: FAIL

- [ ] **Step 3: Implement DefaultPermissionMapper**

```go
// internal/ai/permission_mapper.go
package ai

import "fmt"

// DefaultPermissionMapper maps canonical (Claude Code) permission terms
// to agent-native terms using a static mapping table.
type DefaultPermissionMapper struct {
	// mapping[agent][canonical] = native
	mapping map[string]map[string]string
}

// NewDefaultPermissionMapper creates a mapper with the built-in mapping table.
func NewDefaultPermissionMapper() *DefaultPermissionMapper {
	return &DefaultPermissionMapper{
		mapping: map[string]map[string]string{
			"claude-code": {
				"read":          "Read",
				"edit":          "Edit",
				"bash":          "Bash",
				"web-search":    "WebSearch",
				"computer-use":  "ComputerUse",
				"mcp":           "MCP",
				"notebook-edit": "NotebookEdit",
			},
			"cursor": {
				"read":       "read",
				"edit":       "edit",
				"bash":       "terminal",
				"web-search": "web",
			},
			"codex": {
				"read":       "read",
				"edit":       "edit",
				"bash":       "shell",
				"web-search": "web-search",
			},
		},
	}
}

// MapToNative maps a single canonical permission to its agent-native term.
func (m *DefaultPermissionMapper) MapToNative(canonical string, agent string) (string, error) {
	agentMap, ok := m.mapping[agent]
	if !ok {
		return "", fmt.Errorf("unknown agent %q", agent)
	}
	native, ok := agentMap[canonical]
	if !ok {
		return "", fmt.Errorf("unmappable permission %q for agent %q", canonical, agent)
	}
	return native, nil
}

// MapAllToNative maps a list of canonical permissions to agent-native terms.
// Returns the successfully mapped terms and a list of warning messages for unmappable terms.
func (m *DefaultPermissionMapper) MapAllToNative(canonical []string, agent string) ([]string, []string) {
	var mapped []string
	var warnings []string

	for _, c := range canonical {
		native, err := m.MapToNative(c, agent)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping permission %q for %s: %v", c, agent, err))
			continue
		}
		mapped = append(mapped, native)
	}

	return mapped, warnings
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestDefaultPermissionMapper -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/permission_mapper.go internal/ai/permission_mapper_test.go
git commit -m "feat(ai): implement permission mapper with canonical-to-native mapping"
```

---

## Chunk 3: Agent Providers (Tasks 7-9)

### Task 7: Claude Code provider

**Files:**
- Create: `internal/ai/claude_code_provider.go`
- Create: `internal/ai/claude_code_provider_test.go`

- [ ] **Step 1: Write test for ApplyPermissions**

```go
// internal/ai/claude_code_provider_test.go
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeProvider_ApplyPermissions_MergeWithExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))

	// Pre-existing settings with other keys
	existing := map[string]any{
		"model":        "claude-sonnet-4-6",
		"allowedTools": []any{"Read", "Write"},
		"deniedTools":  []any{"ComputerUse"},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(settingsPath, data, 0o644))

	provider := NewClaudeCodeProvider(settingsPath, nil)
	err := provider.ApplyPermissions(ResolvedPermissions{
		Allow: []string{"Read", "Edit", "Bash"},
		Deny:  []string{},
	})
	require.NoError(t, err)

	// Read back and verify
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	assert.Equal(t, "claude-sonnet-4-6", result["model"]) // preserved
	assert.ElementsMatch(t, []any{"Read", "Edit", "Bash"}, result["allowedTools"])
	assert.Empty(t, result["deniedTools"])
}

func TestClaudeCodeProvider_ApplyPermissions_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	provider := NewClaudeCodeProvider(settingsPath, nil)
	err := provider.ApplyPermissions(ResolvedPermissions{
		Allow: []string{"Read", "Edit"},
		Deny:  []string{"Bash"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	assert.ElementsMatch(t, []any{"Read", "Edit"}, result["allowedTools"])
	assert.ElementsMatch(t, []any{"Bash"}, result["deniedTools"])
}

func TestClaudeCodeProvider_RemovePermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))

	existing := map[string]any{
		"model":        "claude-sonnet-4-6",
		"allowedTools": []any{"Read", "Edit"},
		"deniedTools":  []any{"Bash"},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(settingsPath, data, 0o644))

	provider := NewClaudeCodeProvider(settingsPath, nil)
	err := provider.RemovePermissions(ResolvedPermissions{})
	require.NoError(t, err)

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	assert.Equal(t, "claude-sonnet-4-6", result["model"])
	assert.Empty(t, result["allowedTools"])
	assert.Empty(t, result["deniedTools"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestClaudeCodeProvider -v`
Expected: FAIL

- [ ] **Step 3: Implement ClaudeCodeProvider**

```go
// internal/ai/claude_code_provider.go
package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeCodeProvider manages Claude Code's settings and MCPs.
type ClaudeCodeProvider struct {
	settingsPath string
	runner       CommandRunner
}

// NewClaudeCodeProvider creates a provider for Claude Code.
// settingsPath is the path to .claude/settings.json.
// runner is used for shelling out to `claude mcp add/remove`.
func NewClaudeCodeProvider(settingsPath string, runner CommandRunner) *ClaudeCodeProvider {
	return &ClaudeCodeProvider{
		settingsPath: settingsPath,
		runner:       runner,
	}
}

func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

func (p *ClaudeCodeProvider) SettingsFilePath() string { return p.settingsPath }

func (p *ClaudeCodeProvider) ApplyPermissions(permissions ResolvedPermissions) error {
	settings, err := p.readSettings()
	if err != nil {
		return err
	}

	// Nuke permission keys and replace
	settings["allowedTools"] = permissions.Allow
	settings["deniedTools"] = permissions.Deny

	return p.writeSettings(settings)
}

func (p *ClaudeCodeProvider) RemovePermissions(previousPermissions ResolvedPermissions) error {
	settings, err := p.readSettings()
	if err != nil {
		return err
	}

	settings["allowedTools"] = []string{}
	settings["deniedTools"] = []string{}

	return p.writeSettings(settings)
}

func (p *ClaudeCodeProvider) RegisterMCP(mcp ResolvedMCP) error {
	// Build: claude mcp add <name> -- <command> <args...>
	cmd := fmt.Sprintf("claude mcp add %s", mcp.Name)
	for k, v := range mcp.Env {
		cmd += fmt.Sprintf(" -e %s=%s", k, v)
	}
	cmd += " -- " + mcp.Command
	for _, arg := range mcp.Args {
		cmd += " " + arg
	}
	return p.runner.Run(cmd)
}

func (p *ClaudeCodeProvider) RemoveMCP(name string) error {
	return p.runner.Run(fmt.Sprintf("claude mcp remove %s", name))
}

// readSettings reads the Claude Code settings JSON file.
// Returns an empty map if the file doesn't exist.
func (p *ClaudeCodeProvider) readSettings() (map[string]any, error) {
	data, err := os.ReadFile(p.settingsPath)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p.settingsPath, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p.settingsPath, err)
	}
	return settings, nil
}

// writeSettings writes the settings map to the Claude Code settings JSON file.
func (p *ClaudeCodeProvider) writeSettings(settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(p.settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(p.settingsPath, data, 0o644)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestClaudeCodeProvider -v`
Expected: PASS

- [ ] **Step 5: Create shared test helpers file**

Create `internal/ai/test_helpers_test.go` with mock types shared across all AI test files:

```go
// internal/ai/test_helpers_test.go
package ai

// mockRunner records commands and optionally returns an error.
type mockRunner struct {
	commands []string
	err      error
}

func (m *mockRunner) Run(command string) error {
	m.commands = append(m.commands, command)
	return m.err
}
```

- [ ] **Step 6: Write test for MCP registration**

Add to `internal/ai/claude_code_provider_test.go`:

```go
// NOTE: mockRunner is defined in internal/ai/test_helpers_test.go (created
// once, shared across all _test.go files in the ai package):
//
//   type mockRunner struct {
//       commands []string
//       err      error
//   }
//   func (m *mockRunner) Run(command string) error {
//       m.commands = append(m.commands, command)
//       return m.err
//   }

func TestClaudeCodeProvider_RegisterMCP(t *testing.T) {
	runner := &mockRunner{}
	provider := NewClaudeCodeProvider("/tmp/settings.json", runner)

	err := provider.RegisterMCP(ResolvedMCP{
		Name:    "playwright",
		Command: "npx",
		Args:    []string{"@anthropic/mcp-playwright"},
		Env:     map[string]string{"KEY": "value"},
	})
	require.NoError(t, err)
	require.Len(t, runner.commands, 1)
	assert.Contains(t, runner.commands[0], "claude mcp add playwright")
	assert.Contains(t, runner.commands[0], "-e KEY=value")
	assert.Contains(t, runner.commands[0], "-- npx @anthropic/mcp-playwright")
}

func TestClaudeCodeProvider_RemoveMCP(t *testing.T) {
	runner := &mockRunner{}
	provider := NewClaudeCodeProvider("/tmp/settings.json", runner)

	err := provider.RemoveMCP("playwright")
	require.NoError(t, err)
	require.Len(t, runner.commands, 1)
	assert.Equal(t, "claude mcp remove playwright", runner.commands[0])
}
```

- [ ] **Step 6: Run all Claude Code provider tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestClaudeCodeProvider -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ai/claude_code_provider.go internal/ai/claude_code_provider_test.go
git commit -m "feat(ai): implement Claude Code agent provider"
```

---

### Task 8: Cursor provider

**Files:**
- Create: `internal/ai/cursor_provider.go`
- Create: `internal/ai/cursor_provider_test.go`

- [ ] **Step 1: Write test for Cursor permissions (settings file)**

```go
// internal/ai/cursor_provider_test.go
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorProvider_ApplyPermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".cursor", "settings.json")
	mcpPath := filepath.Join(dir, ".cursor", "mcp.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))

	existing := map[string]any{"theme": "dark", "allowedTools": []any{"old"}}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(settingsPath, data, 0o644))

	provider := NewCursorProvider(settingsPath, mcpPath)
	err := provider.ApplyPermissions(ResolvedPermissions{
		Allow: []string{"read", "edit", "terminal"},
	})
	require.NoError(t, err)

	content, _ := os.ReadFile(settingsPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	assert.Equal(t, "dark", result["theme"])
	assert.ElementsMatch(t, []any{"read", "edit", "terminal"}, result["allowedTools"])
}

func TestCursorProvider_RegisterMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".cursor", "settings.json")
	mcpPath := filepath.Join(dir, ".cursor", "mcp.json")

	provider := NewCursorProvider(settingsPath, mcpPath)
	err := provider.RegisterMCP(ResolvedMCP{
		Name:    "playwright",
		Command: "npx",
		Args:    []string{"@anthropic/mcp-playwright"},
		Env:     map[string]string{"KEY": "val"},
	})
	require.NoError(t, err)

	content, _ := os.ReadFile(mcpPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	mcpServers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok)
	pw, ok := mcpServers["playwright"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "npx", pw["command"])
	assert.Equal(t, map[string]any{"KEY": "val"}, pw["env"])
}

func TestCursorProvider_RemoveMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".cursor", "settings.json")
	mcpPath := filepath.Join(dir, ".cursor", "mcp.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(mcpPath), 0o755))

	existing := map[string]any{
		"mcpServers": map[string]any{
			"playwright": map[string]any{"command": "npx"},
			"github":     map[string]any{"command": "npx"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(mcpPath, data, 0o644))

	provider := NewCursorProvider(settingsPath, mcpPath)
	err := provider.RemoveMCP("playwright")
	require.NoError(t, err)

	content, _ := os.ReadFile(mcpPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))

	mcpServers := result["mcpServers"].(map[string]any)
	assert.NotContains(t, mcpServers, "playwright")
	assert.Contains(t, mcpServers, "github")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestCursorProvider -v`
Expected: FAIL

- [ ] **Step 3: Implement CursorProvider**

```go
// internal/ai/cursor_provider.go
package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CursorProvider manages Cursor's settings and MCP configuration.
type CursorProvider struct {
	settingsPath string
	mcpPath      string
}

// NewCursorProvider creates a provider for Cursor.
func NewCursorProvider(settingsPath, mcpPath string) *CursorProvider {
	return &CursorProvider{
		settingsPath: settingsPath,
		mcpPath:      mcpPath,
	}
}

func (p *CursorProvider) Name() string              { return "cursor" }
func (p *CursorProvider) SettingsFilePath() string   { return p.settingsPath }

func (p *CursorProvider) ApplyPermissions(permissions ResolvedPermissions) error {
	settings, err := readJSONFile(p.settingsPath)
	if err != nil {
		return err
	}
	settings["allowedTools"] = permissions.Allow
	settings["deniedTools"] = permissions.Deny
	return writeJSONFile(p.settingsPath, settings)
}

func (p *CursorProvider) RemovePermissions(previousPermissions ResolvedPermissions) error {
	settings, err := readJSONFile(p.settingsPath)
	if err != nil {
		return err
	}
	settings["allowedTools"] = []string{}
	settings["deniedTools"] = []string{}
	return writeJSONFile(p.settingsPath, settings)
}

func (p *CursorProvider) RegisterMCP(mcp ResolvedMCP) error {
	config, err := readJSONFile(p.mcpPath)
	if err != nil {
		return err
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}

	entry := map[string]any{
		"command": mcp.Command,
	}
	if len(mcp.Args) > 0 {
		entry["args"] = mcp.Args
	}
	if len(mcp.Env) > 0 {
		entry["env"] = mcp.Env
	}
	servers[mcp.Name] = entry
	config["mcpServers"] = servers

	return writeJSONFile(p.mcpPath, config)
}

func (p *CursorProvider) RemoveMCP(name string) error {
	config, err := readJSONFile(p.mcpPath)
	if err != nil {
		return err
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return nil
	}
	delete(servers, name)
	config["mcpServers"] = servers

	return writeJSONFile(p.mcpPath, config)
}

// These helpers live in internal/ai/jsonutil.go (not in cursor_provider.go).

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return result, nil
}

func writeJSONFile(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}
```

- [ ] **Step 4: Refactor ClaudeCodeProvider to use shared helpers**

Update `claude_code_provider.go` to use `readJSONFile`/`writeJSONFile` instead of its own `readSettings`/`writeSettings`. Remove the duplicate methods.

- [ ] **Step 5: Run all provider tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run "TestClaudeCodeProvider|TestCursorProvider" -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ai/cursor_provider.go internal/ai/cursor_provider_test.go internal/ai/claude_code_provider.go
git commit -m "feat(ai): implement Cursor provider and extract shared JSON helpers"
```

---

### Task 9: Codex provider

**Files:**
- Create: `internal/ai/codex_provider.go`
- Create: `internal/ai/codex_provider_test.go`

- [ ] **Step 1: Write tests for Codex provider**

Note: Codex's config format needs to be confirmed from documentation. For now, model it similarly to Cursor with file-based config. Adjust the implementation once Codex's actual config format is confirmed.

```go
// internal/ai/codex_provider_test.go
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexProvider_ApplyPermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".codex", "settings.json")

	provider := NewCodexProvider(settingsPath, filepath.Join(dir, ".codex", "mcp.json"))
	err := provider.ApplyPermissions(ResolvedPermissions{
		Allow: []string{"read", "edit", "shell"},
	})
	require.NoError(t, err)

	content, _ := os.ReadFile(settingsPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))
	assert.ElementsMatch(t, []any{"read", "edit", "shell"}, result["allowedTools"])
}

func TestCodexProvider_RegisterMCP(t *testing.T) {
	dir := t.TempDir()
	mcpPath := filepath.Join(dir, ".codex", "mcp.json")

	provider := NewCodexProvider(filepath.Join(dir, ".codex", "settings.json"), mcpPath)
	err := provider.RegisterMCP(ResolvedMCP{
		Name:    "playwright",
		Command: "npx",
		Args:    []string{"@anthropic/mcp-playwright"},
	})
	require.NoError(t, err)

	content, _ := os.ReadFile(mcpPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))
	mcpServers := result["mcpServers"].(map[string]any)
	assert.Contains(t, mcpServers, "playwright")
}

func TestCodexProvider_RemoveMCP(t *testing.T) {
	dir := t.TempDir()
	mcpPath := filepath.Join(dir, ".codex", "mcp.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(mcpPath), 0o755))

	existing := map[string]any{
		"mcpServers": map[string]any{
			"playwright": map[string]any{"command": "npx"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(mcpPath, data, 0o644))

	provider := NewCodexProvider(filepath.Join(dir, ".codex", "settings.json"), mcpPath)
	err := provider.RemoveMCP("playwright")
	require.NoError(t, err)

	content, _ := os.ReadFile(mcpPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(content, &result))
	mcpServers := result["mcpServers"].(map[string]any)
	assert.NotContains(t, mcpServers, "playwright")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestCodexProvider -v`
Expected: FAIL

- [ ] **Step 3: Implement CodexProvider**

```go
// internal/ai/codex_provider.go
package ai

// CodexProvider manages Codex's settings and MCP configuration.
// Uses file-based config similar to Cursor.
type CodexProvider struct {
	settingsPath string
	mcpPath      string
}

func NewCodexProvider(settingsPath, mcpPath string) *CodexProvider {
	return &CodexProvider{settingsPath: settingsPath, mcpPath: mcpPath}
}

func (p *CodexProvider) Name() string            { return "codex" }
func (p *CodexProvider) SettingsFilePath() string { return p.settingsPath }

func (p *CodexProvider) ApplyPermissions(permissions ResolvedPermissions) error {
	settings, err := readJSONFile(p.settingsPath)
	if err != nil {
		return err
	}
	settings["allowedTools"] = permissions.Allow
	settings["deniedTools"] = permissions.Deny
	return writeJSONFile(p.settingsPath, settings)
}

func (p *CodexProvider) RemovePermissions(previousPermissions ResolvedPermissions) error {
	settings, err := readJSONFile(p.settingsPath)
	if err != nil {
		return err
	}
	settings["allowedTools"] = []string{}
	settings["deniedTools"] = []string{}
	return writeJSONFile(p.settingsPath, settings)
}

func (p *CodexProvider) RegisterMCP(mcp ResolvedMCP) error {
	config, err := readJSONFile(p.mcpPath)
	if err != nil {
		return err
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}

	entry := map[string]any{"command": mcp.Command}
	if len(mcp.Args) > 0 {
		entry["args"] = mcp.Args
	}
	if len(mcp.Env) > 0 {
		entry["env"] = mcp.Env
	}
	servers[mcp.Name] = entry
	config["mcpServers"] = servers

	return writeJSONFile(p.mcpPath, config)
}

func (p *CodexProvider) RemoveMCP(name string) error {
	config, err := readJSONFile(p.mcpPath)
	if err != nil {
		return err
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return nil
	}
	delete(servers, name)
	config["mcpServers"] = servers

	return writeJSONFile(p.mcpPath, config)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestCodexProvider -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/codex_provider.go internal/ai/codex_provider_test.go
git commit -m "feat(ai): implement Codex agent provider"
```

---

## Chunk 4: Skills Manager (Task 10)

### Task 10: NPX Skills Manager

**Files:**
- Create: `internal/ai/skills_manager.go`
- Create: `internal/ai/skills_manager_test.go`

- [ ] **Step 1: Write test for Install**

```go
// internal/ai/skills_manager_test.go
package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNPXSkillsManager_Install(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Install("vercel-labs/agent-skills", []string{"frontend-design", "writing-plans"}, []string{"claude-code", "cursor"})
	require.NoError(t, err)

	require.Len(t, runner.commands, 1)
	cmd := runner.commands[0]
	assert.Contains(t, cmd, "npx skills add vercel-labs/agent-skills")
	assert.Contains(t, cmd, "--skill frontend-design")
	assert.Contains(t, cmd, "--skill writing-plans")
	assert.Contains(t, cmd, "-a claude-code")
	assert.Contains(t, cmd, "-a cursor")
	assert.Contains(t, cmd, "-y")
}

func TestNPXSkillsManager_Remove(t *testing.T) {
	runner := &mockRunner{}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Remove([]string{"frontend-design", "superpowers"}, []string{"claude-code"})
	require.NoError(t, err)

	require.Len(t, runner.commands, 1)
	cmd := runner.commands[0]
	assert.Contains(t, cmd, "npx skills remove frontend-design superpowers")
	assert.Contains(t, cmd, "-a claude-code")
	assert.Contains(t, cmd, "-y")
}

func TestNPXSkillsManager_Install_Error(t *testing.T) {
	runner := &mockRunner{err: assert.AnError}
	mgr := NewNPXSkillsManager(runner)

	err := mgr.Install("repo/a", []string{"skill-1"}, []string{"claude-code"})
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestNPXSkillsManager -v`
Expected: FAIL

- [ ] **Step 3: Implement NPXSkillsManager**

```go
// internal/ai/skills_manager.go
package ai

import (
	"fmt"
	"strings"
)

// NPXSkillsManager manages skills via the `npx skills` CLI.
type NPXSkillsManager struct {
	runner CommandRunner
}

// NewNPXSkillsManager creates a skills manager that shells out to npx.
func NewNPXSkillsManager(runner CommandRunner) *NPXSkillsManager {
	return &NPXSkillsManager{runner: runner}
}

// CheckNPX verifies that npx is available on PATH.
func (m *NPXSkillsManager) CheckNPX() error {
	if err := m.runner.Run("npx --version"); err != nil {
		return fmt.Errorf("npx not found on PATH — required for skills management: %w", err)
	}
	return nil
}

// Install runs `npx skills add <source> --skill <s1> --skill <s2> -a <a1> -a <a2> -y`.
func (m *NPXSkillsManager) Install(source string, skills []string, agents []string) error {
	var parts []string
	parts = append(parts, "npx skills add", source)
	for _, s := range skills {
		parts = append(parts, "--skill", s)
	}
	for _, a := range agents {
		parts = append(parts, "-a", a)
	}
	parts = append(parts, "-y")

	cmd := strings.Join(parts, " ")
	if err := m.runner.Run(cmd); err != nil {
		return fmt.Errorf("skills install %s: %w", source, err)
	}
	return nil
}

// Remove runs `npx skills remove <s1> <s2> -a <a1> -y`.
func (m *NPXSkillsManager) Remove(skills []string, agents []string) error {
	var parts []string
	parts = append(parts, "npx skills remove")
	parts = append(parts, strings.Join(skills, " "))
	for _, a := range agents {
		parts = append(parts, "-a", a)
	}
	parts = append(parts, "-y")

	cmd := strings.Join(parts, " ")
	if err := m.runner.Run(cmd); err != nil {
		return fmt.Errorf("skills remove: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestNPXSkillsManager -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/skills_manager.go internal/ai/skills_manager_test.go
git commit -m "feat(ai): implement NPX skills manager"
```

---

## Chunk 5: Orchestrator (Tasks 11-12)

### Task 11: Orchestrator — Apply

**Files:**
- Create: `internal/ai/orchestrator.go`
- Create: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Write test for basic Apply (permissions only)**

```go
// internal/ai/orchestrator_test.go
package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

type mockProvider struct {
	name                 string
	appliedPermissions   *ResolvedPermissions
	removedPermissions   *ResolvedPermissions
	registeredMCPs       []ResolvedMCP
	removedMCPs          []string
	applyPermErr         error
	registerMCPErr       error
}

func (m *mockProvider) Name() string            { return m.name }
func (m *mockProvider) SettingsFilePath() string { return "/mock/" + m.name }

func (m *mockProvider) ApplyPermissions(p ResolvedPermissions) error {
	m.appliedPermissions = &p
	return m.applyPermErr
}

func (m *mockProvider) RemovePermissions(p ResolvedPermissions) error {
	m.removedPermissions = &p
	return nil
}

func (m *mockProvider) RegisterMCP(mcp ResolvedMCP) error {
	m.registeredMCPs = append(m.registeredMCPs, mcp)
	return m.registerMCPErr
}

func (m *mockProvider) RemoveMCP(name string) error {
	m.removedMCPs = append(m.removedMCPs, name)
	return nil
}

type mockMapper struct {
	mapping map[string]map[string]string
}

func (m *mockMapper) MapToNative(canonical, agent string) (string, error) {
	if agentMap, ok := m.mapping[agent]; ok {
		if native, ok := agentMap[canonical]; ok {
			return native, nil
		}
	}
	return canonical, nil // passthrough
}

func (m *mockMapper) MapAllToNative(canonical []string, agent string) ([]string, []string) {
	var mapped []string
	for _, c := range canonical {
		native, _ := m.MapToNative(c, agent)
		mapped = append(mapped, native)
	}
	return mapped, nil
}

type mockSkillsMgr struct {
	installed []struct{ source string; skills, agents []string }
	removed   []struct{ skills, agents []string }
	err       error
}

func (m *mockSkillsMgr) Install(source string, skills, agents []string) error {
	m.installed = append(m.installed, struct{ source string; skills, agents []string }{source, skills, agents})
	return m.err
}

func (m *mockSkillsMgr) Remove(skills, agents []string) error {
	m.removed = append(m.removed, struct{ skills, agents []string }{skills, agents})
	return m.err
}

type mockReporter struct{}

func (m *mockReporter) Success(msg string)    {}
func (m *mockReporter) Warning(msg string)    {}
func (m *mockReporter) Error(msg string)      {}
func (m *mockReporter) Header(msg string)     {}
func (m *mockReporter) PrintLine(msg string)  {}
func (m *mockReporter) Dim(text string) string { return text }

// --- Tests ---

func TestOrchestrator_Apply_Permissions(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	mapper := &mockMapper{
		mapping: map[string]map[string]string{
			"claude-code": {"read": "Read", "edit": "Edit"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		mapper,
		&mockSkillsMgr{},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": EffectiveAgentConfig{
			Permissions: ResolvedPermissions{Allow: []string{"read", "edit"}, Deny: []string{}},
		},
	}

	state, err := orch.Apply(config, nil)
	require.NoError(t, err)

	require.NotNil(t, provider.appliedPermissions)
	assert.Equal(t, []string{"Read", "Edit"}, provider.appliedPermissions.Allow)

	// State should record native terms
	require.Contains(t, state.Permissions, "claude-code")
	assert.Equal(t, []string{"Read", "Edit"}, state.Permissions["claude-code"].Allow)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Apply_Permissions -v`
Expected: FAIL

- [ ] **Step 3: Implement Orchestrator constructor and Apply (permissions)**

```go
// internal/ai/orchestrator.go
package ai

import (
	"fmt"
	"sort"
	"strings"
)

// Reporter is the subset of reporting methods the orchestrator needs.
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
	permMapper    PermissionMapper
	skillsManager SkillsManager
	reporter      Reporter
}

// NewOrchestrator creates an orchestrator with all dependencies injected.
func NewOrchestrator(
	providers map[string]AgentProvider,
	permMapper PermissionMapper,
	skillsManager SkillsManager,
	reporter Reporter,
) *Orchestrator {
	return &Orchestrator{
		providers:     providers,
		permMapper:    permMapper,
		skillsManager: skillsManager,
		reporter:      reporter,
	}
}

// Apply applies AI configuration and returns the resulting state.
func (o *Orchestrator) Apply(config EffectiveAIConfig, previousState *AIState) (*AIState, error) {
	state := &AIState{
		Permissions: make(map[string]PermissionState),
	}

	// Step 1: Apply permissions
	for agent, agentCfg := range config {
		provider, ok := o.providers[agent]
		if !ok {
			o.reporter.Warning(fmt.Sprintf("no provider for agent %q, skipping", agent))
			continue
		}

		// Map canonical to native
		nativeAllow, warnings := o.permMapper.MapAllToNative(agentCfg.Permissions.Allow, agent)
		for _, w := range warnings {
			o.reporter.Warning(w)
		}
		nativeDeny, warnings := o.permMapper.MapAllToNative(agentCfg.Permissions.Deny, agent)
		for _, w := range warnings {
			o.reporter.Warning(w)
		}

		nativePerms := ResolvedPermissions{Allow: nativeAllow, Deny: nativeDeny}
		if err := provider.ApplyPermissions(nativePerms); err != nil {
			o.reporter.Error(fmt.Sprintf("failed to apply permissions for %s: %v", agent, err))
			continue
		}
		o.reporter.Success(fmt.Sprintf("applied permissions for %s", agent))

		state.Permissions[agent] = PermissionState{
			Allow: nativeAllow,
			Deny:  nativeDeny,
		}
	}

	// Step 2: Apply skills (with orphan removal)
	o.applySkills(config, previousState, state)

	// Step 3: Apply MCPs (with orphan removal)
	o.applyMCPs(config, previousState, state)

	return state, nil
}

// applySkills handles skill orphan removal and installation.
func (o *Orchestrator) applySkills(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	// Collect all current skills across agents
	type skillKey struct{ source, name string }
	currentSkills := make(map[skillKey][]string) // skill -> agents

	for agent, agentCfg := range config {
		for _, skill := range agentCfg.Skills {
			key := skillKey{skill.Source, skill.Name}
			currentSkills[key] = append(currentSkills[key], agent)
		}
	}

	// Find and remove orphans from previous state
	if previousState != nil {
		for _, prevSkill := range previousState.Skills {
			key := skillKey{prevSkill.Source, prevSkill.Name}
			if _, exists := currentSkills[key]; !exists {
				// Orphan — remove from all agents it was installed on
				if err := o.skillsManager.Remove([]string{prevSkill.Name}, prevSkill.Agents); err != nil {
					o.reporter.Error(fmt.Sprintf("failed to remove orphaned skill %s: %v", prevSkill.Name, err))
				} else {
					o.reporter.Success(fmt.Sprintf("removed orphaned skill %s", prevSkill.Name))
				}
			}
		}
	}

	// Install current skills grouped by (source, agent-set).
	// Skills from the same source but targeting different agents must be
	// installed separately to avoid installing to wrong agents.
	type installGroup struct {
		source string
		skills []string
		agents []string
	}

	// Build a key that includes both source and sorted agent list
	agentSetKey := func(agents []string) string {
		sorted := append([]string{}, agents...)
		sort.Strings(sorted)
		return strings.Join(sorted, ",")
	}

	groupMap := make(map[string]*installGroup) // "source\x00agentSet" -> group
	var groupOrder []string

	for key, agents := range currentSkills {
		gk := key.source + "\x00" + agentSetKey(agents)
		if _, exists := groupMap[gk]; !exists {
			groupOrder = append(groupOrder, gk)
			groupMap[gk] = &installGroup{source: key.source, agents: agents}
		}
		groupMap[gk].skills = append(groupMap[gk].skills, key.name)
	}

	for _, gk := range groupOrder {
		g := groupMap[gk]
		if err := o.skillsManager.Install(g.source, g.skills, g.agents); err != nil {
			o.reporter.Error(fmt.Sprintf("failed to install skills from %s: %v", g.source, err))
		} else {
			o.reporter.Success(fmt.Sprintf("installed skills from %s", g.source))
			for _, skill := range g.skills {
				state.Skills = append(state.Skills, SkillState{
					Source: g.source,
					Name:   skill,
					Agents: g.agents,
				})
			}
		}
	}
}

// applyMCPs handles MCP orphan removal and registration.
func (o *Orchestrator) applyMCPs(config EffectiveAIConfig, previousState *AIState, state *AIState) {
	// Collect current MCPs per agent
	currentMCPs := make(map[string][]string) // mcp name -> agents

	for agent, agentCfg := range config {
		for _, mcp := range agentCfg.MCPs {
			currentMCPs[mcp.Name] = append(currentMCPs[mcp.Name], agent)
		}
	}

	// Find and remove orphans
	if previousState != nil {
		for _, prevMCP := range previousState.MCPs {
			if _, exists := currentMCPs[prevMCP.Name]; !exists {
				for _, agent := range prevMCP.Agents {
					provider, ok := o.providers[agent]
					if !ok {
						continue
					}
					if err := provider.RemoveMCP(prevMCP.Name); err != nil {
						o.reporter.Error(fmt.Sprintf("failed to remove orphaned MCP %s from %s: %v", prevMCP.Name, agent, err))
					}
				}
				o.reporter.Success(fmt.Sprintf("removed orphaned MCP %s", prevMCP.Name))
			}
		}
	}

	// Register current MCPs
	// Build a map of MCP name -> ResolvedMCP (from any agent config, they're the same)
	mcpDefs := make(map[string]ResolvedMCP)
	for _, agentCfg := range config {
		for _, mcp := range agentCfg.MCPs {
			mcpDefs[mcp.Name] = mcp
		}
	}

	for name, agents := range currentMCPs {
		mcp := mcpDefs[name]
		var succeededAgents []string

		for _, agent := range agents {
			provider, ok := o.providers[agent]
			if !ok {
				continue
			}
			if err := provider.RegisterMCP(mcp); err != nil {
				o.reporter.Error(fmt.Sprintf("failed to register MCP %s for %s: %v", name, agent, err))
			} else {
				succeededAgents = append(succeededAgents, agent)
			}
		}

		if len(succeededAgents) > 0 {
			o.reporter.Success(fmt.Sprintf("registered MCP %s", name))
			state.MCPs = append(state.MCPs, MCPState{
				Name:   name,
				Agents: succeededAgents,
			})
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Apply_Permissions -v`
Expected: PASS

- [ ] **Step 5: Write test for skills orphan removal**

Add to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_Apply_SkillOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		&mockMapper{mapping: map[string]map[string]string{"claude-code": {}}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Skills: []SkillState{
			{Source: "repo/a", Name: "skill-1", Agents: []string{"claude-code"}},
			{Source: "repo/a", Name: "skill-2", Agents: []string{"claude-code"}},
		},
		Permissions: map[string]PermissionState{},
	}

	// Only skill-1 remains in the new config
	config := EffectiveAIConfig{
		"claude-code": EffectiveAgentConfig{
			Skills: []ResolvedSkill{{Source: "repo/a", Name: "skill-1"}},
		},
	}

	_, err := orch.Apply(config, previousState)
	require.NoError(t, err)

	// skill-2 should have been removed
	require.Len(t, skillsMgr.removed, 1)
	assert.Equal(t, []string{"skill-2"}, skillsMgr.removed[0].skills)
}
```

- [ ] **Step 6: Run test**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Apply_SkillOrphan -v`
Expected: PASS

- [ ] **Step 7: Write test for MCP orphan removal**

Add to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_Apply_MCPOrphanRemoval(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		&mockMapper{mapping: map[string]map[string]string{"claude-code": {}}},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	previousState := &AIState{
		MCPs: []MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
			{Name: "github", Agents: []string{"claude-code"}},
		},
		Permissions: map[string]PermissionState{},
	}

	// Only playwright remains
	config := EffectiveAIConfig{
		"claude-code": EffectiveAgentConfig{
			MCPs: []ResolvedMCP{{Name: "playwright", Command: "npx"}},
		},
	}

	_, err := orch.Apply(config, previousState)
	require.NoError(t, err)

	// github should have been removed
	assert.Contains(t, provider.removedMCPs, "github")
	assert.NotContains(t, provider.removedMCPs, "playwright")
}
```

- [ ] **Step 8: Run test**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Apply_MCPOrphan -v`
Expected: PASS

- [ ] **Step 9: Write test for non-fatal error handling**

Add to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_Apply_NonFatalPermissionError(t *testing.T) {
	failProvider := &mockProvider{name: "claude-code", applyPermErr: assert.AnError}
	okProvider := &mockProvider{name: "cursor"}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": failProvider, "cursor": okProvider},
		&mockMapper{mapping: map[string]map[string]string{
			"claude-code": {"read": "Read"},
			"cursor":      {"read": "read"},
		}},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": EffectiveAgentConfig{Permissions: ResolvedPermissions{Allow: []string{"read"}}},
		"cursor":      EffectiveAgentConfig{Permissions: ResolvedPermissions{Allow: []string{"read"}}},
	}

	state, err := orch.Apply(config, nil)
	require.NoError(t, err) // non-fatal — should not error

	// claude-code failed, cursor succeeded
	assert.NotContains(t, state.Permissions, "claude-code")
	assert.Contains(t, state.Permissions, "cursor")
}
```

- [ ] **Step 10: Run test**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Apply_NonFatal -v`
Expected: PASS

- [ ] **Step 11: Commit**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "feat(ai): implement orchestrator with apply, orphan removal, and non-fatal error handling"
```

---

### Task 12: Orchestrator — Unapply

**Files:**
- Modify: `internal/ai/orchestrator.go`
- Modify: `internal/ai/orchestrator_test.go`

- [ ] **Step 1: Write test for Unapply**

Add to `internal/ai/orchestrator_test.go`:

```go
func TestOrchestrator_Unapply(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		&mockMapper{mapping: map[string]map[string]string{"claude-code": {}}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Skills: []SkillState{
			{Source: "repo/a", Name: "skill-1", Agents: []string{"claude-code"}},
		},
		MCPs: []MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
		},
		Permissions: map[string]PermissionState{
			"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{}},
		},
	}

	err := orch.Unapply(previousState)
	require.NoError(t, err)

	// MCPs removed
	assert.Contains(t, provider.removedMCPs, "playwright")

	// Skills removed
	require.Len(t, skillsMgr.removed, 1)
	assert.Equal(t, []string{"skill-1"}, skillsMgr.removed[0].skills)

	// Permissions removed
	require.NotNil(t, provider.removedPermissions)
}

func TestOrchestrator_Unapply_NilState(t *testing.T) {
	orch := NewOrchestrator(nil, nil, nil, &mockReporter{})
	err := orch.Unapply(nil)
	require.NoError(t, err) // no-op
}

func TestOrchestrator_Unapply_Order(t *testing.T) {
	// Verify unapply order: MCPs -> skills -> permissions
	var callOrder []string

	provider := &mockProvider{name: "claude-code"}
	// Override RemoveMCP to track call order
	origRemoveMCP := provider.RemoveMCP
	_ = origRemoveMCP // use interface-based tracking instead

	// Use a tracking reporter to observe the order of operations
	tracker := &orderTracker{}
	skillsMgr := &trackingSkillsMgr{tracker: tracker}
	trackProvider := &trackingProvider{name: "claude-code", tracker: tracker}

	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": trackProvider},
		&mockMapper{mapping: map[string]map[string]string{"claude-code": {}}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		MCPs:        []MCPState{{Name: "playwright", Agents: []string{"claude-code"}}},
		Skills:      []SkillState{{Source: "repo/a", Name: "skill-1", Agents: []string{"claude-code"}}},
		Permissions: map[string]PermissionState{"claude-code": {Allow: []string{"Read"}}},
	}

	err := orch.Unapply(previousState)
	require.NoError(t, err)

	callOrder = tracker.calls
	require.Len(t, callOrder, 3)
	assert.Equal(t, "remove-mcp", callOrder[0])
	assert.Equal(t, "remove-skill", callOrder[1])
	assert.Equal(t, "remove-permissions", callOrder[2])
}

// orderTracker records the order of operations.
type orderTracker struct {
	calls []string
}

type trackingProvider struct {
	name    string
	tracker *orderTracker
}

func (p *trackingProvider) Name() string            { return p.name }
func (p *trackingProvider) SettingsFilePath() string { return "/mock/" + p.name }
func (p *trackingProvider) ApplyPermissions(perms ResolvedPermissions) error { return nil }
func (p *trackingProvider) RemovePermissions(perms ResolvedPermissions) error {
	p.tracker.calls = append(p.tracker.calls, "remove-permissions")
	return nil
}
func (p *trackingProvider) RegisterMCP(mcp ResolvedMCP) error { return nil }
func (p *trackingProvider) RemoveMCP(name string) error {
	p.tracker.calls = append(p.tracker.calls, "remove-mcp")
	return nil
}

type trackingSkillsMgr struct {
	tracker *orderTracker
}

func (m *trackingSkillsMgr) Install(source string, skills, agents []string) error { return nil }
func (m *trackingSkillsMgr) Remove(skills, agents []string) error {
	m.tracker.calls = append(m.tracker.calls, "remove-skill")
	return nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator_Unapply -v`
Expected: FAIL

- [ ] **Step 3: Implement Unapply**

Add to `internal/ai/orchestrator.go`:

```go
// Unapply removes all facet-managed AI configuration.
// Order: MCPs -> skills -> permissions (reverse of apply).
func (o *Orchestrator) Unapply(previousState *AIState) error {
	if previousState == nil {
		return nil
	}

	// Step 1: Remove MCPs
	for _, mcp := range previousState.MCPs {
		for _, agent := range mcp.Agents {
			provider, ok := o.providers[agent]
			if !ok {
				continue
			}
			if err := provider.RemoveMCP(mcp.Name); err != nil {
				o.reporter.Error(fmt.Sprintf("failed to remove MCP %s from %s: %v", mcp.Name, agent, err))
			}
		}
	}

	// Step 2: Remove skills
	for _, skill := range previousState.Skills {
		if err := o.skillsManager.Remove([]string{skill.Name}, skill.Agents); err != nil {
			o.reporter.Error(fmt.Sprintf("failed to remove skill %s: %v", skill.Name, err))
		}
	}

	// Step 3: Remove permissions
	for agent, perms := range previousState.Permissions {
		provider, ok := o.providers[agent]
		if !ok {
			continue
		}
		if err := provider.RemovePermissions(ResolvedPermissions{Allow: perms.Allow, Deny: perms.Deny}); err != nil {
			o.reporter.Error(fmt.Sprintf("failed to remove permissions from %s: %v", agent, err))
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/ai/ -run TestOrchestrator -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/orchestrator.go internal/ai/orchestrator_test.go
git commit -m "feat(ai): implement orchestrator unapply"
```

---

## Chunk 6: App Integration (Tasks 13-16)

### Task 13: State tracking extension

**Files:**
- Modify: `internal/app/state.go:18-24`
- Modify: `internal/app/state_test.go`

- [ ] **Step 1: Write test for state JSON round-trip with AI field**

Add to `internal/app/state_test.go`:

```go
func TestApplyState_WithAI_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	state := &ApplyState{
		Profile:      "work",
		AppliedAt:    time.Now().Truncate(time.Second),
		FacetVersion: "0.2.0",
		AI: &ai.AIState{
			Skills: []ai.SkillState{
				{Source: "repo/a", Name: "skill-1", Agents: []string{"claude-code"}},
			},
			MCPs: []ai.MCPState{
				{Name: "playwright", Agents: []string{"claude-code", "cursor"}},
			},
			Permissions: map[string]ai.PermissionState{
				"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{}},
			},
		},
	}

	err := store.Write(dir, state)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)

	require.NotNil(t, loaded.AI)
	assert.Len(t, loaded.AI.Skills, 1)
	assert.Equal(t, "skill-1", loaded.AI.Skills[0].Name)
	assert.Len(t, loaded.AI.MCPs, 1)
	assert.Equal(t, "playwright", loaded.AI.MCPs[0].Name)
	assert.Contains(t, loaded.AI.Permissions, "claude-code")
}

func TestApplyState_WithoutAI_Backwards_Compatible(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	// Write state without AI (simulating old facet)
	state := &ApplyState{
		Profile:      "work",
		AppliedAt:    time.Now().Truncate(time.Second),
		FacetVersion: "0.1.0",
	}

	err := store.Write(dir, state)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, loaded.AI) // omitempty — nil when not present
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run TestApplyState_WithAI -v`
Expected: FAIL — AI field not on ApplyState

- [ ] **Step 3: Add AI field to ApplyState**

In `internal/app/state.go`, add the import and field:

```go
import "facet/internal/ai"

type ApplyState struct {
	Profile      string                   `json:"profile"`
	AppliedAt    time.Time                `json:"applied_at"`
	FacetVersion string                   `json:"facet_version"`
	Packages     []packages.PackageResult `json:"packages"`
	Configs      []deploy.ConfigResult    `json:"configs"`
	AI           *ai.AIState              `json:"ai,omitempty"`
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run TestApplyState -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/state.go internal/app/state_test.go
git commit -m "feat(app): extend ApplyState with AI field"
```

---

### Task 14: App-level interface and deps

**Files:**
- Modify: `internal/app/interfaces.go`
- Modify: `internal/app/app.go:4-36`

- [ ] **Step 1: Add AIOrchestrator interface**

In `internal/app/interfaces.go`, add:

```go
import "facet/internal/ai"

// AIOrchestrator manages AI coding tool configuration.
type AIOrchestrator interface {
	Apply(config ai.EffectiveAIConfig, previousState *ai.AIState) (*ai.AIState, error)
	Unapply(previousState *ai.AIState) error
}
```

- [ ] **Step 2: Add AIOrchestrator to Deps and App**

In `internal/app/app.go`, add `AIOrchestrator` to `Deps` and `App`:

```go
type Deps struct {
	Loader          ProfileLoader
	Installer       Installer
	Reporter        Reporter
	StateStore      StateStore
	DeployerFactory DeployerFactory
	AIOrchestrator  AIOrchestrator // nil if AI not configured
	Version         string
	OSName          string
}
```

Update `New()` to store the new field.

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/edocsss/aec/src/facet && go build ./...`
Expected: Success

- [ ] **Step 4: Run all existing tests to verify no regressions**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -v`
Expected: All PASS (existing tests pass nil for AIOrchestrator)

- [ ] **Step 5: Commit**

```bash
git add internal/app/interfaces.go internal/app/app.go
git commit -m "feat(app): add AIOrchestrator interface and wire into Deps"
```

---

### Task 15: Apply flow integration

**Files:**
- Modify: `internal/app/apply.go`
- Modify: `internal/app/apply_test.go`

- [ ] **Step 1: Write test for apply with AI config**

Add to `internal/app/apply_test.go`:

```go
// mockAIOrchestrator implements AIOrchestrator for testing.
type mockAIOrchestrator struct {
	applyCalled   bool
	unapplyCalled bool
	config        ai.EffectiveAIConfig
	prevState     *ai.AIState
	returnState   *ai.AIState
	returnErr     error
}

func (m *mockAIOrchestrator) Apply(config ai.EffectiveAIConfig, prev *ai.AIState) (*ai.AIState, error) {
	m.applyCalled = true
	m.config = config
	m.prevState = prev
	if m.returnState != nil {
		return m.returnState, m.returnErr
	}
	return &ai.AIState{Permissions: map[string]ai.PermissionState{}}, m.returnErr
}

func (m *mockAIOrchestrator) Unapply(prev *ai.AIState) error {
	m.unapplyCalled = true
	m.prevState = prev
	return m.returnErr
}

func TestApply_WithAI(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()

	// Write facet.yaml
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "facet.yaml"), []byte("min_version: \"0.1.0\"\n"), 0o644))

	// Write base.yaml
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "base.yaml"), []byte("vars: {}\n"), 0o644))

	// Write profile
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "profiles"), 0o755))
	profileYAML := `extends: base
ai:
  agents: [claude-code]
  permissions:
    allow: [read, edit]
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "profiles", "work.yaml"), []byte(profileYAML), 0o644))

	// Write .local.yaml
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte("vars: {}\n"), 0o644))

	mockAI := &mockAIOrchestrator{}

	application := app.New(app.Deps{
		Loader:          profile.NewLoader(),
		Installer:       &mockInstaller{},
		Reporter:        &mockReporter{},
		StateStore:      app.NewFileStateStore(),
		DeployerFactory: func(cd, hd string, v map[string]any, oc []deploy.ConfigResult) deploy.Service {
			return deploy.NewDeployer(cd, hd, v, oc)
		},
		AIOrchestrator: mockAI,
		Version:        "0.1.0",
		OSName:         "darwin",
	})

	err := application.Apply("work", app.ApplyOpts{
		ConfigDir: configDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	assert.True(t, mockAI.applyCalled, "AIOrchestrator.Apply should have been called")
	assert.Contains(t, mockAI.config, "claude-code", "effective config should contain claude-code")
}

func TestApply_WithoutAI(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(configDir, "facet.yaml"), []byte("min_version: \"0.1.0\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "base.yaml"), []byte("vars: {}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "profiles"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "profiles", "work.yaml"), []byte("extends: base\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte("vars: {}\n"), 0o644))

	mockAI := &mockAIOrchestrator{}
	application := app.New(app.Deps{
		Loader:          profile.NewLoader(),
		Installer:       &mockInstaller{},
		Reporter:        &mockReporter{},
		StateStore:      app.NewFileStateStore(),
		DeployerFactory: func(cd, hd string, v map[string]any, oc []deploy.ConfigResult) deploy.Service {
			return deploy.NewDeployer(cd, hd, v, oc)
		},
		AIOrchestrator: mockAI,
		Version:        "0.1.0",
		OSName:         "darwin",
	})

	err := application.Apply("work", app.ApplyOpts{
		ConfigDir: configDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	assert.False(t, mockAI.applyCalled, "AIOrchestrator.Apply should NOT be called when no AI config")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -run TestApply_WithAI -v`
Expected: FAIL

- [ ] **Step 3: Integrate AI into Apply flow**

In `internal/app/apply.go`, two changes are needed:

**Change 1: AI unapply — inside the `shouldUnapply` block (lines ~95-125), BEFORE `unapplyDeployer.Unapply(prevState.Configs)`**

Insert AI unapply so the order is: MCPs -> skills -> permissions -> configs (per design spec).

```go
// Inside the shouldUnapply block, BEFORE unapplyDeployer.Unapply(prevState.Configs):

// AI unapply (MCPs -> skills -> permissions, then configs)
if a.aiOrchestrator != nil && prevState.AI != nil {
	if err := a.aiOrchestrator.Unapply(prevState.AI); err != nil {
		a.reporter.Error(fmt.Sprintf("AI unapply failed: %v", err))
	}
}

// ... existing: unapplyDeployer.Unapply(prevState.Configs)
```

**Change 2: AI apply — AFTER the config deployment loop (~line 172), BEFORE `stateStore.Write()` (~line 184)**

This ensures `state.AI` is populated before the state is persisted.

```go
// Step 11-13: Apply AI configuration
if a.aiOrchestrator != nil && resolved.AI != nil {
	effectiveAI := ai.Resolve(resolved.AI)

	var prevAIState *ai.AIState
	if prevState != nil && prevState.AI != nil {
		prevAIState = prevState.AI
	}

	aiState, aiErr := a.aiOrchestrator.Apply(effectiveAI, prevAIState)
	if aiErr != nil {
		a.reporter.Error(fmt.Sprintf("AI configuration failed: %v", aiErr))
	}
	state.AI = aiState
}

// ... existing: stateStore.Write(stateDir, state)
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./internal/app/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/apply.go internal/app/apply_test.go
git commit -m "feat(app): integrate AI configuration into apply pipeline"
```

---

### Task 16: Reporting and status

**Files:**
- Modify: `internal/app/report.go`
- Modify: `internal/app/status.go`

- [ ] **Step 1: Extend printDryRun for AI section**

In `internal/app/report.go`, add AI dry-run output after config dry-run:

```go
// AI dry-run
if resolved.AI != nil {
	a.reporter.Header("AI Configuration")
	effectiveAI := ai.Resolve(resolved.AI)
	for agent, agentCfg := range effectiveAI {
		a.reporter.PrintLine(fmt.Sprintf("  Agent: %s", agent))
		if len(agentCfg.Permissions.Allow) > 0 {
			a.reporter.PrintLine(fmt.Sprintf("    Permissions allow: %v", agentCfg.Permissions.Allow))
		}
		for _, skill := range agentCfg.Skills {
			a.reporter.PrintLine(fmt.Sprintf("    Skill: %s/%s", skill.Source, skill.Name))
		}
		for _, mcp := range agentCfg.MCPs {
			a.reporter.PrintLine(fmt.Sprintf("    MCP: %s", mcp.Name))
		}
	}
}
```

- [ ] **Step 2: Extend Status() for AI state**

In `internal/app/status.go`, add AI status output after config status:

```go
// AI status
if s.AI != nil {
	a.reporter.Header("AI Configuration")

	if len(s.AI.Permissions) > 0 {
		a.reporter.PrintLine("Permissions:")
		for agent, perms := range s.AI.Permissions {
			a.reporter.PrintLine(fmt.Sprintf("  %s: allow=%v deny=%v", agent, perms.Allow, perms.Deny))
		}
	}

	if len(s.AI.Skills) > 0 {
		a.reporter.PrintLine("Skills:")
		for _, skill := range s.AI.Skills {
			a.reporter.PrintLine(fmt.Sprintf("  %s/%s → %v", skill.Source, skill.Name, skill.Agents))
		}
	}

	if len(s.AI.MCPs) > 0 {
		a.reporter.PrintLine("MCPs:")
		for _, mcp := range s.AI.MCPs {
			a.reporter.PrintLine(fmt.Sprintf("  %s → %v", mcp.Name, mcp.Agents))
		}
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/app/report.go internal/app/status.go
git commit -m "feat(app): extend dry-run and status output for AI configuration"
```

---

### Task 17: Main.go wiring

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Wire AI dependencies in main.go**

Add to `main.go`:

```go
import "facet/internal/ai"

// In main():

// AI wiring
permMapper := ai.NewDefaultPermissionMapper()
skillsRunner := packages.NewShellRunner() // reuse ShellRunner for skills CLI
skillsMgr := ai.NewNPXSkillsManager(skillsRunner)

homeDir, _ := os.UserHomeDir()
providers := map[string]ai.AgentProvider{
	"claude-code": ai.NewClaudeCodeProvider(
		filepath.Join(homeDir, ".claude", "settings.json"),
		runner,
	),
	"cursor": ai.NewCursorProvider(
		filepath.Join(homeDir, ".cursor", "settings.json"),
		filepath.Join(homeDir, ".cursor", "mcp.json"),
	),
	"codex": ai.NewCodexProvider(
		filepath.Join(homeDir, ".codex", "settings.json"),
		filepath.Join(homeDir, ".codex", "mcp.json"),
	),
}

aiOrchestrator := ai.NewOrchestrator(providers, permMapper, skillsMgr, r)

// Update app creation:
application := app.New(app.Deps{
	// ... existing deps ...
	AIOrchestrator: aiOrchestrator,
})
```

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/edocsss/aec/src/facet && go build -o /dev/null .`
Expected: Success

- [ ] **Step 3: Run all tests**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire AI configuration dependencies in main.go"
```

---

## Chunk 7: E2E Tests (Task 18)

### Task 18: Hermetic E2E with stub CLIs

**Files:**
- Create: `e2e/suites/10-ai-config.sh`
- Create: `e2e/fixtures/ai-config/` (fixture files)
- Create: `e2e/stubs/` (stub CLI scripts)

- [ ] **Step 1: Create stub CLI scripts**

```bash
# e2e/stubs/skills
#!/usr/bin/env bash
# Stub for `npx skills` — records invocations and simulates success
echo "$0 $@" >> "${FACET_STUB_LOG:-/tmp/facet-stub.log}"
exit 0
```

```bash
# e2e/stubs/claude
#!/usr/bin/env bash
# Stub for `claude` CLI — records invocations and simulates success
echo "$0 $@" >> "${FACET_STUB_LOG:-/tmp/facet-stub.log}"
exit 0
```

```bash
# e2e/stubs/npx
#!/usr/bin/env bash
# Stub for `npx` — delegates to stubs/skills if invoked as `npx skills`
if [ "$1" = "skills" ]; then
    shift
    echo "npx skills $@" >> "${FACET_STUB_LOG:-/tmp/facet-stub.log}"
    exit 0
fi
# Fallback to real npx
exec /usr/bin/env npx.real "$@"
```

- [ ] **Step 2: Create E2E fixture**

```yaml
# e2e/fixtures/ai-config/base.yaml
vars:
  github_token: test-token-123

packages: []

configs: {}

ai:
  agents: [claude-code, cursor]
  permissions:
    allow: [read, edit, bash]
  skills:
    - source: vercel-labs/agent-skills
      skills: [frontend-design]
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
    - name: github
      command: npx
      args: ["@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${facet:github_token}"
      agents: [claude-code]
  overrides:
    cursor:
      permissions:
        allow: [read, edit]
```

```yaml
# e2e/fixtures/ai-config/profiles/work.yaml
extends: base
```

- [ ] **Step 3: Create E2E test suite**

```bash
# e2e/suites/10-ai-config.sh
#!/usr/bin/env bash
set -euo pipefail

SUITE="AI Config"
source "$(dirname "$0")/../harness.sh"

# Setup stubs
STUB_DIR="$(dirname "$0")/../stubs"
chmod +x "$STUB_DIR"/*
export PATH="$STUB_DIR:$PATH"
export FACET_STUB_LOG="$TMPDIR/facet-stub.log"
: > "$FACET_STUB_LOG"

# Setup config dir
CONFIG_DIR="$TMPDIR/ai-config"
cp -r "$(dirname "$0")/../fixtures/ai-config" "$CONFIG_DIR"
cp "$(dirname "$0")/../fixtures/facet.yaml" "$CONFIG_DIR/"

# Setup state dir
STATE_DIR="$TMPDIR/ai-state"
mkdir -p "$STATE_DIR"
echo "vars: {}" > "$STATE_DIR/.local.yaml"

# Test 1: Apply with AI config
begin_test "facet apply with AI config"
$FACET apply work -c "$CONFIG_DIR" -s "$STATE_DIR"
assert_file_exists "$STATE_DIR/.state.json"

# Verify stub was called for skills
assert_file_contains "$FACET_STUB_LOG" "npx skills add vercel-labs/agent-skills"
assert_file_contains "$FACET_STUB_LOG" "--skill frontend-design"

# Verify permissions files were created
assert_file_exists "$HOME/.claude/settings.json"

# Verify state has AI section (use grep since assert_json_has_key may not exist in harness)
grep -q '"ai"' "$STATE_DIR/.state.json" || fail "state.json missing ai section"
end_test

# Test 2: Verify env var resolution in MCP
begin_test "MCP env var resolution"
assert_file_contains "$FACET_STUB_LOG" "claude mcp add github"
# The env should have the resolved token
assert_file_contains "$FACET_STUB_LOG" "GITHUB_TOKEN=test-token-123"
end_test

# Test 3: Verify settings file content
begin_test "Claude Code permissions written correctly"
grep -q '"allowedTools"' "$HOME/.claude/settings.json" || fail "missing allowedTools"
grep -q '"Read"' "$HOME/.claude/settings.json" || fail "missing Read permission"
end_test

# Test 4: Orphan removal on reapply with reduced config
begin_test "Orphan skill removal on reapply"
# Create a reduced profile (no skills)
cat > "$CONFIG_DIR/profiles/minimal.yaml" << 'PROFILE_EOF'
extends: base
ai:
  agents: [claude-code]
  permissions:
    allow: [read]
PROFILE_EOF

: > "$FACET_STUB_LOG"  # clear stub log
$FACET apply minimal -c "$CONFIG_DIR" -s "$STATE_DIR"
# The previous skill (frontend-design) should have been removed
assert_file_contains "$FACET_STUB_LOG" "npx skills remove frontend-design"
end_test
```

- [ ] **Step 4: Run E2E test**

Run: `cd /Users/edocsss/aec/src/facet && bash e2e/suites/10-ai-config.sh`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add e2e/stubs/ e2e/fixtures/ai-config/ e2e/suites/10-ai-config.sh
git commit -m "test(e2e): add hermetic E2E tests for AI configuration with stub CLIs"
```

---

## Final Verification

- [ ] **Run all tests**

```bash
cd /Users/edocsss/aec/src/facet && go test ./... -v
```
Expected: All PASS

- [ ] **Run linter**

```bash
cd /Users/edocsss/aec/src/facet && go vet ./...
```
Expected: No issues

- [ ] **Final commit (if any remaining changes)**

```bash
git add -A
git commit -m "feat: AI configuration support for Claude Code, Cursor, and Codex"
```
