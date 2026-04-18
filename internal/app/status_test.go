package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/deploy"
)

type mockStateStore struct {
	state    *ApplyState
	readErr  error
	written  *ApplyState
	writeErr error
}

func (m *mockStateStore) Read(stateDir string) (*ApplyState, error) {
	return m.state, m.readErr
}

func (m *mockStateStore) Write(stateDir string, s *ApplyState) error {
	m.written = s
	return m.writeErr
}

func (m *mockStateStore) CanaryWrite(stateDir string) error {
	return nil
}

func TestStatus_NoState(t *testing.T) {
	r := &mockReporter{}
	store := &mockStateStore{state: nil}
	a := New(Deps{Reporter: r, StateStore: store})

	err := a.Status(StatusOpts{StateDir: t.TempDir()})
	require.NoError(t, err)

	assert.Contains(t, r.messages[0], "No profile has been applied")
}

func TestStatus_WithState(t *testing.T) {
	r := &mockReporter{}
	store := &mockStateStore{
		state: &ApplyState{
			Profile:   "work",
			AppliedAt: time.Now(),
			Configs: []deploy.ConfigResult{
				{Target: "/tmp/does-not-exist", Source: "configs/.zshrc", Strategy: deploy.StrategySymlink},
			},
		},
	}
	a := New(Deps{Reporter: r, StateStore: store})

	err := a.Status(StatusOpts{StateDir: t.TempDir()})
	require.NoError(t, err)

	// Should have printed profile header
	found := false
	for _, msg := range r.messages {
		if msg == "header: Profile: work" {
			found = true
		}
	}
	assert.True(t, found, "expected header with profile name")
}

func TestRunValidityChecks_SymlinkValid(t *testing.T) {
	cfgDir := t.TempDir()
	homeDir := t.TempDir()

	source := filepath.Join(cfgDir, "configs/.zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))

	target := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, target))

	s := &ApplyState{
		Configs: []deploy.ConfigResult{
			{Target: target, Source: "configs/.zshrc", Strategy: deploy.StrategySymlink},
		},
	}

	checks := runValidityChecks(s, cfgDir)
	require.Len(t, checks, 1)
	assert.True(t, checks[0].Valid)
}

func TestRunValidityChecks_FileMissing(t *testing.T) {
	s := &ApplyState{
		Configs: []deploy.ConfigResult{
			{Target: "/tmp/nonexistent-facet-test-path", Source: "configs/.zshrc", Strategy: deploy.StrategySymlink},
		},
	}

	checks := runValidityChecks(s, "")
	require.Len(t, checks, 1)
	assert.False(t, checks[0].Valid)
	assert.Equal(t, "file missing", checks[0].Error)
}
