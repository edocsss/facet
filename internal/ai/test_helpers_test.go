package ai

import "strings"

// mockRunner records commands and optionally returns an error.
type mockRunner struct {
	commands            []string
	interactiveCommands []string
	err                 error
}

func (m *mockRunner) Run(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	return m.err
}

func (m *mockRunner) RunInteractive(name string, args ...string) error {
	cmd := strings.Join(append([]string{name}, args...), " ")
	m.commands = append(m.commands, cmd)
	m.interactiveCommands = append(m.interactiveCommands, cmd)
	return m.err
}

// sequentialMockRunner records commands and returns errors from a pre-defined
// sequence, one per call. If the call index exceeds the errors slice, nil is returned.
type sequentialMockRunner struct {
	commands            []string
	interactiveCommands []string
	errors              []error
	callIdx             int
}

func (m *sequentialMockRunner) Run(name string, args ...string) error {
	m.commands = append(m.commands, strings.Join(append([]string{name}, args...), " "))
	var err error
	if m.callIdx < len(m.errors) {
		err = m.errors[m.callIdx]
	}
	m.callIdx++
	return err
}

func (m *sequentialMockRunner) RunInteractive(name string, args ...string) error {
	cmd := strings.Join(append([]string{name}, args...), " ")
	m.commands = append(m.commands, cmd)
	m.interactiveCommands = append(m.interactiveCommands, cmd)
	var err error
	if m.callIdx < len(m.errors) {
		err = m.errors[m.callIdx]
	}
	m.callIdx++
	return err
}
