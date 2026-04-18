package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestLoadMeta_Valid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "facet.yaml"), `min_version: "0.1.0"`)

	loader := NewLoader()
	meta, err := loader.LoadMeta(dir)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", meta.MinVersion)
}

func TestLoadMeta_Missing(t *testing.T) {
	dir := t.TempDir()
	loader := NewLoader()
	_, err := loader.LoadMeta(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "facet.yaml")
}

func TestLoadConfig_Base(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.yaml"), `
vars:
  git_name: Sarah
packages:
  - name: git
    install: brew install git
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	loader := NewLoader()
	cfg, err := loader.LoadConfig(filepath.Join(dir, "base.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "Sarah", cfg.Vars["git_name"])
	assert.Len(t, cfg.Packages, 1)
	assert.Equal(t, "git", cfg.Packages[0].Name)
	assert.Equal(t, "configs/.gitconfig", cfg.Configs["~/.gitconfig"])
}

func TestLoadConfig_Profile_ExtendsBase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "profiles", "work.yaml"), `
extends: base
vars:
  git_email: sarah@acme.com
`)

	loader := NewLoader()
	cfg, err := loader.LoadConfig(filepath.Join(dir, "profiles", "work.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "base", cfg.Extends)
	assert.Equal(t, "sarah@acme.com", cfg.Vars["git_email"])
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{invalid yaml`)

	loader := NewLoader()
	_, err := loader.LoadConfig(filepath.Join(dir, "bad.yaml"))
	assert.Error(t, err)
}

func TestLoadConfig_NestedVars(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.yaml"), `
vars:
  git:
    name: Sarah
    email: sarah@hey.com
  aws:
    region: us-east-1
`)

	loader := NewLoader()
	cfg, err := loader.LoadConfig(filepath.Join(dir, "base.yaml"))
	require.NoError(t, err)

	gitVars, ok := cfg.Vars["git"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Sarah", gitVars["name"])
	assert.Equal(t, "sarah@hey.com", gitVars["email"])
}

func TestValidateProfile_InvalidExtends(t *testing.T) {
	cfg := &FacetConfig{Extends: "other"}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base")
}

func TestValidateProfile_MissingExtends(t *testing.T) {
	cfg := &FacetConfig{}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
}

func TestValidateProfile_Valid(t *testing.T) {
	cfg := &FacetConfig{Extends: "base"}
	err := ValidateProfile(cfg)
	assert.NoError(t, err)
}

func TestValidateProfile_AIEmptyAgents(t *testing.T) {
	cfg := &FacetConfig{
		Extends: "base",
		AI:      &AIConfig{Agents: []string{}},
	}
	err := ValidateMergedConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agents")
}

func TestValidateProfile_AIPermissionsUnknownAgent(t *testing.T) {
	cfg := &FacetConfig{
		Extends: "base",
		AI: &AIConfig{
			Agents: []string{"claude"},
			Permissions: map[string]*PermissionsConfig{
				"unknown-agent": {Allow: []string{"Read"}},
			},
		},
	}
	err := ValidateMergedConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-agent")
}

func TestValidateProfile_AINoSection(t *testing.T) {
	cfg := &FacetConfig{Extends: "base"}
	err := ValidateProfile(cfg)
	assert.NoError(t, err)
}
