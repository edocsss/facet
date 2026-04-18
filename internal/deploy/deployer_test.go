package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConfigDir(t *testing.T) (configDir, homeDir string) {
	t.Helper()
	configDir = t.TempDir()
	homeDir = t.TempDir()
	return
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestDetectStrategy_StaticFile(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.zshrc"))
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, strategy)
}

func TestDetectStrategy_TemplateFile(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.gitconfig"), "[user]\n  email = ${facet:git.email}")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, StrategyTemplate, strategy)
}

func TestDetectStrategy_Directory(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "configs/nvim"), 0o755))
	writeTestFile(t, filepath.Join(configDir, "configs/nvim/init.lua"), "-- nvim config")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/nvim"))
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, strategy)
}

func TestDeploy_Symlink_NewTarget(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	d := NewDeployer(configDir, homeDir, nil, nil)
	result, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, result.Strategy)

	// Verify symlink
	target, err := os.Readlink(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, "configs/.zshrc"), target)
}

func TestDeploy_Symlink_AlreadyCorrect(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "export EDITOR=nvim")

	// Pre-create correct symlink
	dest := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, dest))

	d := NewDeployer(configDir, homeDir, nil, nil)
	result, err := d.DeployOne(dest, "configs/.zshrc", false)
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, result.Strategy)
}

func TestDeploy_Template_RendersVars(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.gitconfig"),
		"[user]\n  email = ${facet:git.email}\n  name = ${facet:git.name}")

	vars := map[string]any{
		"git": map[string]any{
			"email": "sarah@acme.com",
			"name":  "Sarah",
		},
	}

	d := NewDeployer(configDir, homeDir, vars, nil)
	result, err := d.DeployOne(
		filepath.Join(homeDir, ".gitconfig"),
		"configs/.gitconfig",
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, StrategyTemplate, result.Strategy)

	content, err := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "sarah@acme.com")
	assert.Contains(t, string(content), "Sarah")
	assert.NotContains(t, string(content), "${facet:")
}

func TestDeploy_CreateParentDirs(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/nvim/init.lua"), "-- config")

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".config", "nvim", "init.lua"),
		"configs/nvim/init.lua",
		false,
	)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"))
}

func TestDeploy_ExistingRegularFile_ErrorWithoutForce(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "new content")
	writeTestFile(t, filepath.Join(homeDir, ".zshrc"), "existing user content")

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		false, // no force
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exists")
}

func TestDeploy_ExistingRegularFile_ReplacedWithForce(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "new content")
	writeTestFile(t, filepath.Join(homeDir, ".zshrc"), "existing user content")

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		true, // force
	)
	require.NoError(t, err)

	// Should be a symlink now
	target, err := os.Readlink(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, source, target)
}

func TestUnapply_RemovesSymlinks(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "content")
	dest := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, dest))

	configs := []ConfigResult{
		{Target: dest, Source: "configs/.zshrc", Strategy: StrategySymlink},
	}

	d := NewDeployer(configDir, homeDir, nil, nil)
	err := d.Unapply(configs)
	require.NoError(t, err)
	assert.NoFileExists(t, dest)
}

func TestUnapply_RemovesTemplatedFiles(t *testing.T) {
	_, homeDir := setupConfigDir(t)
	dest := filepath.Join(homeDir, ".gitconfig")
	writeTestFile(t, dest, "rendered content")

	configs := []ConfigResult{
		{Target: dest, Source: "configs/.gitconfig", Strategy: StrategyTemplate},
	}

	d := NewDeployer("", homeDir, nil, nil)
	err := d.Unapply(configs)
	require.NoError(t, err)
	assert.NoFileExists(t, dest)
}

func TestUnapply_NoState_NoOp(t *testing.T) {
	d := NewDeployer("", t.TempDir(), nil, nil)
	err := d.Unapply(nil)
	assert.NoError(t, err)
}

func TestRollback(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "content")

	d := NewDeployer(configDir, homeDir, nil, nil)

	// Deploy one file
	_, err := d.DeployOne(filepath.Join(homeDir, ".zshrc"), "configs/.zshrc", false)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(homeDir, ".zshrc"))

	// Rollback should remove it
	err = d.Rollback()
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(homeDir, ".zshrc"))
}
