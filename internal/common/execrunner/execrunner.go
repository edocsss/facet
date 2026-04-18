package execrunner

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner executes commands directly without going through a shell.
type Runner struct{}

// New constructs a Runner.
func New() *Runner {
	return &Runner{}
}

// Run executes the given command and argv.
func (r *Runner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// RunInteractive executes the given command with stdout and stderr connected
// directly to the parent process, allowing real-time streaming output.
func (r *Runner) RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
