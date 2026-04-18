package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.NotNil(t, result["claude-code"])
	assert.Equal(t, []string{"Read"}, result["claude-code"].Allow)
	assert.Equal(t, []string{"Bash"}, result["claude-code"].Deny)
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

func TestMergeSkills_AllFromSource(t *testing.T) {
	base := []SkillEntry{
		{Source: "source-a"},
	}
	result := mergeSkills(base, nil)

	require.Len(t, result, 1)
	assert.Equal(t, "source-a", result[0].Source)
	assert.Nil(t, result[0].Skills)
}

func TestMergeSkills_AllInBaseSpecificInOverlay(t *testing.T) {
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
	assert.Equal(t, []string{"claude-code", "cursor"}, result.Agents)

	require.NotNil(t, result.Permissions)
	require.Len(t, result.Permissions, 2)
	assert.Equal(t, []string{"Read"}, result.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{"Bash"}, result.Permissions["claude-code"].Deny)
	assert.Equal(t, []string{"Read(**)"}, result.Permissions["cursor"].Allow)

	require.NotNil(t, result.Skills)
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
	require.Len(t, result.Permissions, 2)
	assert.Equal(t, []string{"Read"}, result.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{"Read(**)"}, result.Permissions["cursor"].Allow)
}

func TestMergeAI_DropsPermissionsForAgentsNoLongerInScope(t *testing.T) {
	base := &AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{}},
			"cursor":      {Allow: []string{"Read(**)"}, Deny: []string{}},
		},
	}
	overlay := &AIConfig{
		Agents: []string{"claude-code"},
	}

	result := mergeAI(base, overlay)

	require.NotNil(t, result)
	assert.Equal(t, []string{"claude-code"}, result.Agents)
	require.Len(t, result.Permissions, 1)
	assert.Contains(t, result.Permissions, "claude-code")
	assert.NotContains(t, result.Permissions, "cursor")
}

func TestMergeAI_DoesNotMutateInputPermissions(t *testing.T) {
	base := &AIConfig{
		Agents: []string{"claude-code", "cursor"},
		Permissions: map[string]*PermissionsConfig{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{}},
			"cursor":      {Allow: []string{"Read(**)"}, Deny: []string{}},
		},
	}
	overlay := &AIConfig{
		Agents: []string{"claude-code"},
	}

	result := mergeAI(base, overlay)

	require.NotNil(t, result)
	require.Len(t, result.Permissions, 1)
	require.Len(t, base.Permissions, 2)
	assert.Contains(t, base.Permissions, "claude-code")
	assert.Contains(t, base.Permissions, "cursor")
	assert.Equal(t, []string{"Read(**)"}, base.Permissions["cursor"].Allow)
}
