package deploy

import (
	"path/filepath"
	"testing"

	"facet/internal/profile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath_Tilde(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("~/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}

func TestExpandPath_TildeAlone(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("~")
	require.NoError(t, err)
	assert.Equal(t, homeDir, result)
}

func TestExpandPath_DollarHOME(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("$HOME/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}

func TestExpandPath_DollarBraceHOME(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	result, err := ExpandPath("${HOME}/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".gitconfig"), result)
}

func TestExpandPath_CustomEnvVar(t *testing.T) {
	t.Setenv("MY_CONFIG_DIR", "/opt/configs")
	result, err := ExpandPath("$MY_CONFIG_DIR/app.conf")
	require.NoError(t, err)
	assert.Equal(t, "/opt/configs/app.conf", result)
}

func TestExpandPath_CustomBraceEnvVar(t *testing.T) {
	t.Setenv("MY_DIR", "/custom")
	result, err := ExpandPath("${MY_DIR}/file")
	require.NoError(t, err)
	assert.Equal(t, "/custom/file", result)
}

func TestExpandPath_UndefinedEnvVar(t *testing.T) {
	_, err := ExpandPath("$UNDEFINED_VAR_12345/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNDEFINED_VAR_12345")
}

func TestExpandPath_AlreadyAbsolute(t *testing.T) {
	result, err := ExpandPath("/usr/local/bin/tool")
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/tool", result)
}

func TestExpandPath_RelativePath_Error(t *testing.T) {
	_, err := ExpandPath("relative/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestResolveSourcePath_LocalRelative(t *testing.T) {
	configDir := t.TempDir()
	spec, err := ResolveSourcePath("configs/.gitconfig", profile.ConfigProvenance{}, configDir)
	require.NoError(t, err)
	assert.Equal(t, "configs/.gitconfig", spec.DisplaySource)
	assert.Equal(t, filepath.Join(configDir, "configs", ".gitconfig"), spec.ResolvedPath)
	assert.False(t, spec.Materialize)
}

func TestResolveSourcePath_RemoteResolvedAbsolute(t *testing.T) {
	remoteRoot := t.TempDir()
	spec, err := ResolveSourcePath("configs/.gitconfig", profile.ConfigProvenance{
		SourceRoot:  remoteRoot,
		Materialize: true,
	}, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "configs/.gitconfig", spec.DisplaySource)
	assert.Equal(t, filepath.Join(remoteRoot, "configs", ".gitconfig"), spec.ResolvedPath)
	assert.True(t, spec.Materialize)
}

func TestResolveSourcePath_TraversalError(t *testing.T) {
	configDir := t.TempDir()
	_, err := ResolveSourcePath("../outside/.gitconfig", profile.ConfigProvenance{}, configDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func TestResolveSourcePath_AbsolutePreserved(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "configs", ".gitconfig")
	spec, err := ResolveSourcePath(sourcePath, profile.ConfigProvenance{}, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, sourcePath, spec.DisplaySource)
	assert.Equal(t, sourcePath, spec.ResolvedPath)
	assert.False(t, spec.Materialize)
}

func TestResolveSourcePath_RemoteAbsoluteRejected(t *testing.T) {
	_, err := ResolveSourcePath("/etc/passwd", profile.ConfigProvenance{
		SourceRoot:  t.TempDir(),
		Materialize: true,
	}, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be relative")
}
