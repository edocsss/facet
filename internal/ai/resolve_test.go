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

	claudeCfg := result["claude-code"]
	assert.Equal(t, []string{"Read", "Edit"}, claudeCfg.Permissions.Allow)
	assert.Equal(t, []string{"Bash"}, claudeCfg.Permissions.Deny)
	require.Len(t, claudeCfg.Skills, 2)
	require.Len(t, claudeCfg.MCPs, 1)

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

func TestResolve_SkillDefaultAgents_ExcludesNonDefault(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "cursor", "codex", "copilot", "gemini"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/skills", Skills: []string{"shared-skill"}},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)
	assert.Len(t, result, 5)

	// Default agents get the skill.
	assert.Len(t, result["claude-code"].Skills, 1)
	assert.Len(t, result["cursor"].Skills, 1)
	assert.Len(t, result["codex"].Skills, 1)

	// Non-default agents do NOT get the skill.
	assert.Len(t, result["copilot"].Skills, 0)
	assert.Len(t, result["gemini"].Skills, 0)
}

func TestResolve_SkillExplicitAgents_OverridesDefault(t *testing.T) {
	cfg := &profile.AIConfig{
		Agents: []string{"claude-code", "copilot"},
		Skills: []profile.SkillEntry{
			{Source: "github.com/example/skills", Skills: []string{"my-skill"}, Agents: []string{"copilot"}},
		},
	}

	result := Resolve(cfg)
	require.NotNil(t, result)

	// Explicit agents list overrides defaults — copilot gets the skill.
	assert.Len(t, result["copilot"].Skills, 1)
	// claude-code is NOT in the explicit list, so it doesn't get it.
	assert.Len(t, result["claude-code"].Skills, 0)
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
