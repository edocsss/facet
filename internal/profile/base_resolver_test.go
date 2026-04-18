package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseResolver_LocalFile(t *testing.T) {
	configDir := t.TempDir()
	writeFile(t, filepath.Join(configDir, "base.yaml"), `
vars:
  source: local-file
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	resolver := NewBaseResolver(NewLoader(), nil)
	result, err := resolver.Resolve("base.yaml", configDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, result.Cleanup())
	})

	assert.Equal(t, "local-file", result.Config.Vars["source"])
	assert.Equal(t, ConfigProvenance{
		SourceRoot:  configDir,
		Materialize: false,
	}, result.Config.ConfigMeta["~/.gitconfig"])
}

func TestBaseResolver_LocalDirectory(t *testing.T) {
	configDir := t.TempDir()
	sharedDir := filepath.Join(configDir, "shared")
	writeFile(t, filepath.Join(sharedDir, "base.yaml"), `
vars:
  source: local-dir
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	resolver := NewBaseResolver(NewLoader(), nil)
	result, err := resolver.Resolve("./shared", configDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, result.Cleanup())
	})

	assert.Equal(t, "local-dir", result.Config.Vars["source"])
	assert.Equal(t, ConfigProvenance{
		SourceRoot:  sharedDir,
		Materialize: false,
	}, result.Config.ConfigMeta["~/.gitconfig"])
}

func TestBaseResolver_GitDefaultBranch(t *testing.T) {
	repo := newGitRepoWithHistory(t)
	resolver := NewBaseResolver(NewLoader(), &gitTestRunner{})

	result, err := resolver.Resolve("file://"+repo.Path, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, result.Cleanup())
	})

	assert.Equal(t, "commit", result.Config.Vars["source"])
	meta := result.Config.ConfigMeta["~/.gitconfig"]
	assert.True(t, meta.Materialize)
	assert.DirExists(t, meta.SourceRoot)
	assert.NotEqual(t, repo.Path, meta.SourceRoot)
}

func TestBaseResolver_GitBranchTagAndCommit(t *testing.T) {
	repo := newGitRepoWithHistory(t)
	resolver := NewBaseResolver(NewLoader(), &gitTestRunner{})

	branchResult, err := resolver.Resolve(fmt.Sprintf("file://%s@%s", repo.Path, repo.Branch), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, branchResult.Cleanup())
	})
	assert.Equal(t, repo.Branch, branchResult.Config.Vars["source"])

	tagResult, err := resolver.Resolve(fmt.Sprintf("file://%s@%s", repo.Path, repo.Tag), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, tagResult.Cleanup())
	})
	assert.Equal(t, "main", tagResult.Config.Vars["source"])

	commitResult, err := resolver.Resolve(fmt.Sprintf("file://%s@%s", repo.Path, repo.Commit), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, commitResult.Cleanup())
	})
	assert.Equal(t, "commit", commitResult.Config.Vars["source"])
}

func TestBaseResolver_CleanupRemovesClone(t *testing.T) {
	repo := newGitRepoWithHistory(t)
	resolver := NewBaseResolver(NewLoader(), &gitTestRunner{})

	result, err := resolver.Resolve("file://"+repo.Path, t.TempDir())
	require.NoError(t, err)

	root := result.Config.ConfigMeta["~/.gitconfig"].SourceRoot
	require.DirExists(t, root)
	require.NoError(t, result.Cleanup())
	assert.NoDirExists(t, root)
}

func TestBaseResolver_FileURLWithAtSignPathDoesNotSplitRef(t *testing.T) {
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "team@repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))
	runGit(t, "", "init", "-b", "main", repoDir)
	runGit(t, repoDir, "config", "user.name", "Facet Test")
	runGit(t, repoDir, "config", "user.email", "facet@example.com")
	writeFile(t, filepath.Join(repoDir, "base.yaml"), `
vars:
  source: at-path
configs:
  ~/.gitconfig: configs/.gitconfig
`)
	writeFile(t, filepath.Join(repoDir, "configs", ".gitconfig"), "[user]\n  name = Facet Test\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "main")

	resolver := NewBaseResolver(NewLoader(), &gitTestRunner{})
	result, err := resolver.Resolve("file://"+repoDir, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, result.Cleanup())
	})

	assert.Equal(t, "at-path", result.Config.Vars["source"])
}

func TestBaseResolver_ExpandsTildeForLocalFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeFile(t, filepath.Join(homeDir, "shared", "base.yaml"), `
vars:
  source: tilde
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	resolver := NewBaseResolver(NewLoader(), nil)
	result, err := resolver.Resolve("~/shared/base.yaml", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, result.Cleanup())
	})

	assert.Equal(t, "tilde", result.Config.Vars["source"])
	assert.Equal(t, filepath.Join(homeDir, "shared"), result.Config.ConfigMeta["~/.gitconfig"].SourceRoot)
}

type gitRepoFixture struct {
	Path   string
	Branch string
	Tag    string
	Commit string
}

func newGitRepoWithHistory(t *testing.T) gitRepoFixture {
	t.Helper()

	repoDir := t.TempDir()
	runGit(t, "", "init", "-b", "main", repoDir)
	runGit(t, repoDir, "config", "user.name", "Facet Test")
	runGit(t, repoDir, "config", "user.email", "facet@example.com")

	writeFile(t, filepath.Join(repoDir, "base.yaml"), `
vars:
  source: main
configs:
  ~/.gitconfig: configs/.gitconfig
`)
	writeFile(t, filepath.Join(repoDir, "configs", ".gitconfig"), "[user]\n  name = Facet Test\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "main")
	runGit(t, repoDir, "tag", "v1.0.0")

	runGit(t, repoDir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(repoDir, "base.yaml"), `
vars:
  source: feature
configs:
  ~/.gitconfig: configs/.gitconfig
`)
	runGit(t, repoDir, "add", "base.yaml")
	runGit(t, repoDir, "commit", "-m", "feature")

	runGit(t, repoDir, "checkout", "main")
	writeFile(t, filepath.Join(repoDir, "base.yaml"), `
vars:
  source: commit
configs:
  ~/.gitconfig: configs/.gitconfig
`)
	runGit(t, repoDir, "add", "base.yaml")
	runGit(t, repoDir, "commit", "-m", "commit")

	commit := strings.TrimSpace(runGitCapture(t, repoDir, "rev-parse", "HEAD"))
	return gitRepoFixture{
		Path:   repoDir,
		Branch: "feature",
		Tag:    "v1.0.0",
		Commit: commit,
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := args
	if dir != "" {
		fullArgs = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", fullArgs, string(output))
}

func runGitCapture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", fullArgs, string(output))
	return string(output)
}

type gitTestRunner struct{}

func (r *gitTestRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
