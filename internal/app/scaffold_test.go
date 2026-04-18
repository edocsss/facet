package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffold_CreatesConfigRepo(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	r := &mockReporter{}

	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(cfgDir, "facet.yaml"))
	assert.FileExists(t, filepath.Join(cfgDir, "base.yaml"))
	assert.DirExists(t, filepath.Join(cfgDir, "profiles"))
	assert.DirExists(t, filepath.Join(cfgDir, "configs"))
	assert.FileExists(t, filepath.Join(stDir, ".local.yaml"))
}

func TestScaffold_AlreadyInitialized(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "facet.yaml"), []byte("existing"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")
}

func TestScaffold_PreservesExistingLocalYaml(t *testing.T) {
	cfgDir := t.TempDir()
	stDir := t.TempDir()
	localPath := filepath.Join(stDir, ".local.yaml")
	require.NoError(t, os.MkdirAll(stDir, 0o755))
	require.NoError(t, os.WriteFile(localPath, []byte("custom content"), 0o644))

	r := &mockReporter{}
	a := New(Deps{Reporter: r})
	err := a.Scaffold(ScaffoldOpts{ConfigDir: cfgDir, StateDir: stDir})
	require.NoError(t, err)

	content, _ := os.ReadFile(localPath)
	assert.Equal(t, "custom content", string(content))
}
