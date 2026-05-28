package pi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	commands []string
	fail     map[string]error
}

func (m *mockRunner) Run(name string, args ...string) error {
	cmd := name
	for _, arg := range args {
		cmd += " " + arg
	}
	m.commands = append(m.commands, cmd)
	if err, ok := m.fail[cmd]; ok {
		return err
	}
	return nil
}

func (m *mockRunner) RunInteractive(name string, args ...string) error {
	return m.Run(name, args...)
}

type mockReporter struct {
	warnings []string
	success  []string
}

func (m *mockReporter) Success(msg string) { m.success = append(m.success, msg) }
func (m *mockReporter) Warning(msg string) { m.warnings = append(m.warnings, msg) }

func TestManagerApply_InstallsCurrentAndRemovesOrphans(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	state, err := mgr.Apply(&Config{Extensions: []string{"pi-lens", "pi-subagents"}}, &PiState{Extensions: []string{"pi-lens", "old-ext"}})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"pi extension remove old-ext",
		"pi extension install pi-lens",
		"pi extension install pi-subagents",
	}, runner.commands)
	assert.Equal(t, []string{"pi-lens", "pi-subagents"}, state.Extensions)
}

func TestManagerApply_RecordsOnlySuccessfulInstalls(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{
		"pi extension install broken-ext": fmt.Errorf("boom"),
	}}
	reporter := &mockReporter{}
	mgr := NewManager(runner, reporter)

	state, err := mgr.Apply(&Config{Extensions: []string{"ok-ext", "broken-ext"}}, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"ok-ext"}, state.Extensions)
	assert.Len(t, reporter.warnings, 1)
}

func TestManagerApply_NilConfigRemovesPreviousManagedExtensions(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	state, err := mgr.Apply(nil, &PiState{Extensions: []string{"pi-lens"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"pi extension remove pi-lens"}, runner.commands)
	assert.Nil(t, state)
}

func TestManagerUnapply_RemovesPreviousManagedExtensions(t *testing.T) {
	runner := &mockRunner{fail: map[string]error{}}
	mgr := NewManager(runner, &mockReporter{})

	require.NoError(t, mgr.Unapply(&PiState{Extensions: []string{"pi-lens", "pi-subagents"}}))
	assert.Equal(t, []string{
		"pi extension remove pi-lens",
		"pi extension remove pi-subagents",
	}, runner.commands)
}
