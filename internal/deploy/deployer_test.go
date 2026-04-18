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

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.zshrc"), false)
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, strategy)
}

func TestDetectStrategy_TemplateFile(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.gitconfig"), "[user]\n  email = ${facet:git.email}")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.gitconfig"), false)
	require.NoError(t, err)
	assert.Equal(t, StrategyTemplate, strategy)
}

func TestDetectStrategy_Directory(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "configs/nvim"), 0o755))
	writeTestFile(t, filepath.Join(configDir, "configs/nvim/init.lua"), "-- nvim config")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/nvim"), false)
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, strategy)
}

func TestDetectStrategy_CopyWhenMaterialized(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.zshrc"), true)
	require.NoError(t, err)
	assert.Equal(t, StrategyCopy, strategy)
}

func TestDeploy_Symlink_NewTarget(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	d := NewDeployer(configDir, homeDir, nil, nil)
	result, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		SourceSpec{
			DisplaySource: "configs/.zshrc",
			ResolvedPath:  filepath.Join(configDir, "configs/.zshrc"),
		},
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
	result, err := d.DeployOne(dest, SourceSpec{
		DisplaySource: "configs/.zshrc",
		ResolvedPath:  source,
	}, false)
	require.NoError(t, err)
	assert.Equal(t, StrategySymlink, result.Strategy)
}

func TestDeploy_ExistingUnmanagedSymlink_ErrorWithoutForce(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "export EDITOR=nvim")
	other := filepath.Join(homeDir, "other-target")
	writeTestFile(t, other, "user owned")

	dest := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(other, dest))

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(dest, SourceSpec{
		DisplaySource: "configs/.zshrc",
		ResolvedPath:  source,
	}, false)
	require.Error(t, err)
	currentTarget, readErr := os.Readlink(dest)
	require.NoError(t, readErr)
	assert.Equal(t, other, currentTarget)
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
		SourceSpec{
			DisplaySource: "configs/.gitconfig",
			ResolvedPath:  filepath.Join(configDir, "configs/.gitconfig"),
		},
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
		SourceSpec{
			DisplaySource: "configs/nvim/init.lua",
			ResolvedPath:  filepath.Join(configDir, "configs/nvim/init.lua"),
		},
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
		SourceSpec{
			DisplaySource: "configs/.zshrc",
			ResolvedPath:  filepath.Join(configDir, "configs/.zshrc"),
		},
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
		SourceSpec{
			DisplaySource: "configs/.zshrc",
			ResolvedPath:  source,
		},
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

func TestUnapply_RefusesToDeleteUnexpectedDirectory(t *testing.T) {
	_, homeDir := setupConfigDir(t)
	target := filepath.Join(homeDir, ".gitconfig")
	require.NoError(t, os.MkdirAll(target, 0o755))
	writeTestFile(t, filepath.Join(target, "nested"), "keep me")

	d := NewDeployer("", homeDir, nil, nil)
	err := d.Unapply([]ConfigResult{
		{Target: target, Source: "configs/.gitconfig", Strategy: StrategyCopy, IsDir: false},
	})
	require.Error(t, err)
	assert.DirExists(t, target)
	assert.FileExists(t, filepath.Join(target, "nested"))
}

func TestUnapply_RefusesToDeleteRepointedSymlink(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	target := filepath.Join(homeDir, ".gitconfig")
	expectedSource := filepath.Join(configDir, "configs", ".gitconfig")
	writeTestFile(t, expectedSource, "expected")
	otherSource := filepath.Join(homeDir, "other")
	writeTestFile(t, otherSource, "other")
	require.NoError(t, os.Symlink(otherSource, target))

	d := NewDeployer(configDir, homeDir, nil, nil)
	err := d.Unapply([]ConfigResult{
		{Target: target, Source: "configs/.gitconfig", SourcePath: expectedSource, Strategy: StrategySymlink, IsDir: false},
	})
	require.Error(t, err)
	currentTarget, readErr := os.Readlink(target)
	require.NoError(t, readErr)
	assert.Equal(t, otherSource, currentTarget)
}

func TestUnapply_RefusesToDeleteRegularFileReplacingSymlink(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	target := filepath.Join(homeDir, ".gitconfig")
	expectedSource := filepath.Join(configDir, "configs", ".gitconfig")
	writeTestFile(t, expectedSource, "expected")
	writeTestFile(t, target, "user file")

	d := NewDeployer(configDir, homeDir, nil, nil)
	err := d.Unapply([]ConfigResult{
		{Target: target, Source: "configs/.gitconfig", SourcePath: expectedSource, Strategy: StrategySymlink, IsDir: false},
	})
	require.Error(t, err)
	content, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "user file", string(content))
}

func TestUnapply_RefusesToDeleteDirectoryReplacingSymlink(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	target := filepath.Join(homeDir, ".gitconfig")
	expectedSource := filepath.Join(configDir, "configs", ".gitconfig")
	writeTestFile(t, expectedSource, "expected")
	require.NoError(t, os.MkdirAll(target, 0o755))
	writeTestFile(t, filepath.Join(target, "nested"), "keep me")

	d := NewDeployer(configDir, homeDir, nil, nil)
	err := d.Unapply([]ConfigResult{
		{Target: target, Source: "configs/.gitconfig", SourcePath: expectedSource, Strategy: StrategySymlink, IsDir: false},
	})
	require.Error(t, err)
	assert.DirExists(t, target)
	assert.FileExists(t, filepath.Join(target, "nested"))
}

func TestRollback(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "content")

	d := NewDeployer(configDir, homeDir, nil, nil)

	// Deploy one file
	_, err := d.DeployOne(filepath.Join(homeDir, ".zshrc"), SourceSpec{
		DisplaySource: "configs/.zshrc",
		ResolvedPath:  source,
	}, false)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(homeDir, ".zshrc"))

	// Rollback should remove it
	err = d.Rollback()
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(homeDir, ".zshrc"))
}

func TestDeploy_OwnedFileReplacedByDirectoryRefusesDelete(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "new content")

	target := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.MkdirAll(target, 0o755))
	writeTestFile(t, filepath.Join(target, "nested"), "keep me")

	d := NewDeployer(configDir, homeDir, nil, []ConfigResult{
		{Target: target, Source: "configs/.zshrc", Strategy: StrategySymlink, IsDir: false},
	})
	_, err := d.DeployOne(target, SourceSpec{
		DisplaySource: "configs/.zshrc",
		ResolvedPath:  source,
	}, false)
	require.Error(t, err)
	assert.DirExists(t, target)
	assert.FileExists(t, filepath.Join(target, "nested"))
}

func TestDeploy_CopyMaterializedFile(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs", ".remote")
	writeTestFile(t, source, "remote content")

	d := NewDeployer(configDir, homeDir, nil, nil)
	result, err := d.DeployOne(filepath.Join(homeDir, ".remote"), SourceSpec{
		DisplaySource: "configs/.remote",
		ResolvedPath:  source,
		Materialize:   true,
	}, false)
	require.NoError(t, err)
	assert.Equal(t, StrategyCopy, result.Strategy)
	assert.Equal(t, source, result.SourcePath)
	assert.False(t, isSymlink(t, filepath.Join(homeDir, ".remote")))
	content, readErr := os.ReadFile(filepath.Join(homeDir, ".remote"))
	require.NoError(t, readErr)
	assert.Equal(t, "remote content", string(content))
}

func TestDeploy_CopyMaterializedFileRejectsEscapedParentSymlink(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	outsideDir := filepath.Join(homeDir, "outside")
	writeTestFile(t, filepath.Join(outsideDir, ".remote"), "secret")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.Symlink(outsideDir, filepath.Join(configDir, "configs")))

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(filepath.Join(homeDir, ".remote"), SourceSpec{
		DisplaySource: "configs/.remote",
		ResolvedPath:  filepath.Join(configDir, "configs", ".remote"),
		Materialize:   true,
		SourceRoot:    configDir,
	}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes source root")
	assert.NoFileExists(t, filepath.Join(homeDir, ".remote"))
}

func TestDeploy_CopyMaterializedDirectory(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	sourceDir := filepath.Join(configDir, "configs", "remote-dir")
	writeTestFile(t, filepath.Join(sourceDir, "init.lua"), "print('remote')")

	d := NewDeployer(configDir, homeDir, nil, nil)
	result, err := d.DeployOne(filepath.Join(homeDir, ".config", "remote-dir"), SourceSpec{
		DisplaySource: "configs/remote-dir",
		ResolvedPath:  sourceDir,
		Materialize:   true,
	}, false)
	require.NoError(t, err)
	assert.Equal(t, StrategyCopy, result.Strategy)
	assert.FileExists(t, filepath.Join(homeDir, ".config", "remote-dir", "init.lua"))
	assert.False(t, isSymlink(t, filepath.Join(homeDir, ".config", "remote-dir")))
}

func TestDeploy_CopyMaterializedDirectoryRejectsNestedSymlink(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	sourceDir := filepath.Join(configDir, "configs", "remote-dir")
	writeTestFile(t, filepath.Join(sourceDir, "init.lua"), "print('remote')")
	outside := filepath.Join(homeDir, "outside.txt")
	writeTestFile(t, outside, "secret")
	require.NoError(t, os.Symlink(outside, filepath.Join(sourceDir, "linked.txt")))

	d := NewDeployer(configDir, homeDir, nil, nil)
	_, err := d.DeployOne(filepath.Join(homeDir, ".config", "remote-dir"), SourceSpec{
		DisplaySource: "configs/remote-dir",
		ResolvedPath:  sourceDir,
		Materialize:   true,
		SourceRoot:    configDir,
	}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes source root")
	assert.NoFileExists(t, filepath.Join(homeDir, ".config", "remote-dir"))
}

func isSymlink(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	return info.Mode()&os.ModeSymlink != 0
}
