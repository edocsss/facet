package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExtends_HTTPSDefaultBranch(t *testing.T) {
	spec, err := ParseExtends("https://github.com/me/personal-dotfiles.git")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "https://github.com/me/personal-dotfiles.git", spec.Locator)
	assert.Empty(t, spec.Ref)
}

func TestParseExtends_HTTPSWithBranch(t *testing.T) {
	spec, err := ParseExtends("https://github.com/me/personal-dotfiles.git@main")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "https://github.com/me/personal-dotfiles.git", spec.Locator)
	assert.Equal(t, "main", spec.Ref)
}

func TestParseExtends_SSHWithTag(t *testing.T) {
	spec, err := ParseExtends("git@github.com:me/personal-dotfiles.git@v1.2.0")
	require.NoError(t, err)
	assert.Equal(t, ExtendsGit, spec.Kind)
	assert.Equal(t, "git@github.com:me/personal-dotfiles.git", spec.Locator)
	assert.Equal(t, "v1.2.0", spec.Ref)
}

func TestParseExtends_LocalDirectory(t *testing.T) {
	spec, err := ParseExtends("./personal-dotfiles")
	require.NoError(t, err)
	assert.Equal(t, ExtendsDir, spec.Kind)
	assert.Equal(t, "./personal-dotfiles", spec.Locator)
}

func TestParseExtends_LocalFile(t *testing.T) {
	spec, err := ParseExtends("profiles/shared/base.yaml")
	require.NoError(t, err)
	assert.Equal(t, ExtendsFile, spec.Kind)
	assert.Equal(t, "profiles/shared/base.yaml", spec.Locator)
}

func TestParseExtends_LocalDirectoryWithAtSign(t *testing.T) {
	spec, err := ParseExtends("/tmp/with@sign")
	require.NoError(t, err)
	assert.Equal(t, ExtendsDir, spec.Kind)
	assert.Equal(t, "/tmp/with@sign", spec.Locator)
	assert.Empty(t, spec.Ref)
}

func TestParseExtends_RejectsEmpty(t *testing.T) {
	_, err := ParseExtends("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extends")
}

func TestParseExtends_RejectsBareToken(t *testing.T) {
	_, err := ParseExtends("something_else")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extends")
}
