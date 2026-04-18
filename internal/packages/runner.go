package packages

import (
	"os"
	"os/exec"
)

// CommandRunner abstracts shell command execution.
type CommandRunner interface {
	Run(command string) error
}

// ShellRunner executes commands via sh -c.
type ShellRunner struct{}

func NewShellRunner() *ShellRunner {
	return &ShellRunner{}
}

func (r *ShellRunner) Run(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
