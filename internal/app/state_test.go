package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/ai"
	"facet/internal/deploy"
	"facet/internal/packages"
)

func TestFileStateStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	s := &ApplyState{
		Profile:      "acme",
		AppliedAt:    time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC),
		FacetVersion: "0.1.0",
		Packages: []packages.PackageResult{
			{Name: "git", Install: "brew install git", Status: "ok"},
			{Name: "docker", Install: "brew install docker", Status: "failed", Error: "not found"},
		},
		Configs: []deploy.ConfigResult{
			{Target: "~/.gitconfig", Source: "configs/.gitconfig", Strategy: deploy.StrategyTemplate},
			{Target: "~/.zshrc", Source: "configs/.zshrc", Strategy: deploy.StrategySymlink},
		},
	}

	err := store.Write(dir, s)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "acme", loaded.Profile)
	assert.Equal(t, "0.1.0", loaded.FacetVersion)
	assert.Len(t, loaded.Packages, 2)
	assert.Len(t, loaded.Configs, 2)
	assert.Equal(t, "failed", loaded.Packages[1].Status)
	assert.Equal(t, deploy.StrategyTemplate, loaded.Configs[0].Strategy)
}

func TestFileStateStore_Read_Missing(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()
	s, err := store.Read(dir)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestFileStateStore_Read_Corrupted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".state.json"), []byte("{{{bad json"), 0o644))

	store := NewFileStateStore()
	_, err := store.Read(dir)
	assert.Error(t, err)
}

func TestFileStateStore_CanaryWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()
	err := store.CanaryWrite(dir)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".state.json"))
	assert.NoError(t, err)
}

func TestFileStateStore_CanaryWrite_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o444))
	defer os.Chmod(dir, 0o755) // cleanup

	store := NewFileStateStore()
	err := store.CanaryWrite(dir)
	assert.Error(t, err)
}

func TestApplyState_WithAI_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	s := &ApplyState{
		Profile:      "work",
		AppliedAt:    time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		FacetVersion: "0.1.0",
		AI: &ai.AIState{
			Skills: []ai.SkillState{
				{Source: "@anthropic/skills", Name: "code-review", Agents: []string{"claude-code"}},
			},
			MCPs: []ai.MCPState{
				{Name: "my-mcp", Agents: []string{"claude-code", "cursor"}},
			},
			Permissions: map[string]ai.PermissionState{
				"claude-code": {Allow: []string{"Read"}, Deny: []string{"Execute"}},
			},
		},
	}

	err := store.Write(dir, s)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded.AI)

	assert.Len(t, loaded.AI.Skills, 1)
	assert.Equal(t, "code-review", loaded.AI.Skills[0].Name)
	assert.Equal(t, "@anthropic/skills", loaded.AI.Skills[0].Source)
	assert.Equal(t, []string{"claude-code"}, loaded.AI.Skills[0].Agents)

	assert.Len(t, loaded.AI.MCPs, 1)
	assert.Equal(t, "my-mcp", loaded.AI.MCPs[0].Name)
	assert.Equal(t, []string{"claude-code", "cursor"}, loaded.AI.MCPs[0].Agents)

	assert.Len(t, loaded.AI.Permissions, 1)
	assert.Equal(t, []string{"Read"}, loaded.AI.Permissions["claude-code"].Allow)
	assert.Equal(t, []string{"Execute"}, loaded.AI.Permissions["claude-code"].Deny)
}

func TestApplyState_WithoutAI_BackwardsCompatible(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStateStore()

	s := &ApplyState{
		Profile:      "personal",
		AppliedAt:    time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		FacetVersion: "0.1.0",
		Packages: []packages.PackageResult{
			{Name: "git", Install: "brew install git", Status: "ok"},
		},
	}

	err := store.Write(dir, s)
	require.NoError(t, err)

	loaded, err := store.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, loaded.AI)
	assert.Equal(t, "personal", loaded.Profile)
	assert.Len(t, loaded.Packages, 1)
}
