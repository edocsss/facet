package packages

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/profile"
)

// mockRunner records commands and returns preset errors.
type mockRunner struct {
	commands []string
	failOn   map[string]error
}

func newMockRunner() *mockRunner {
	return &mockRunner{failOn: make(map[string]error)}
}

func (m *mockRunner) Run(command string) error {
	m.commands = append(m.commands, command)
	if err, ok := m.failOn[command]; ok {
		return err
	}
	return nil
}

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	if runtime.GOOS == "darwin" {
		assert.Equal(t, "macos", os)
	} else {
		assert.Equal(t, "linux", os)
	}
}

func TestGetInstallCommand_Simple(t *testing.T) {
	pkg := profile.PackageEntry{
		Name:    "ripgrep",
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, skip := GetInstallCommand(pkg, "macos")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)

	cmd, skip = GetInstallCommand(pkg, "linux")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)
}

func TestGetInstallCommand_PerOS(t *testing.T) {
	pkg := profile.PackageEntry{
		Name: "lazydocker",
		Install: profile.InstallCmd{
			PerOS: map[string]string{
				"macos": "brew install lazydocker",
				"linux": "go install github.com/jesseduffield/lazydocker@latest",
			},
		},
	}

	cmd, skip := GetInstallCommand(pkg, "macos")
	assert.Equal(t, "brew install lazydocker", cmd)
	assert.False(t, skip)

	cmd, skip = GetInstallCommand(pkg, "linux")
	assert.Equal(t, "go install github.com/jesseduffield/lazydocker@latest", cmd)
	assert.False(t, skip)
}

func TestGetInstallCommand_MissingOS(t *testing.T) {
	pkg := profile.PackageEntry{
		Name: "xcode-tools",
		Install: profile.InstallCmd{
			PerOS: map[string]string{
				"macos": "xcode-select --install",
			},
		},
	}

	_, skip := GetInstallCommand(pkg, "linux")
	assert.True(t, skip)
}

func TestInstallAll_CollectsResults(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{Name: "echo-test", Install: profile.InstallCmd{Command: "echo hello"}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, "echo-test", results[0].Name)
	assert.Equal(t, "ok", results[0].Status)
	assert.Equal(t, []string{"echo hello"}, runner.commands)
}

func TestInstallAll_FailureContinues(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["will-fail-cmd"] = errors.New("exit status 1")
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{Name: "will-fail", Install: profile.InstallCmd{Command: "will-fail-cmd"}},
		{Name: "will-pass", Install: profile.InstallCmd{Command: "echo ok"}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 2)
	assert.Equal(t, "failed", results[0].Status)
	assert.NotEmpty(t, results[0].Error)
	assert.Equal(t, "ok", results[1].Status)
}

func TestInstallAll_SkippedOS(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "linux")

	pkgs := []profile.PackageEntry{
		{Name: "mac-only", Install: profile.InstallCmd{
			PerOS: map[string]string{"macos": "echo mac"},
		}},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Empty(t, runner.commands) // runner was never called
}

func TestGetCheckCommand_NoCheck(t *testing.T) {
	pkg := profile.PackageEntry{
		Name:    "ripgrep",
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, hasCheck := GetCheckCommand(pkg, "macos")
	assert.False(t, hasCheck)
	assert.Empty(t, cmd)
}

func TestGetCheckCommand_SimpleCheck(t *testing.T) {
	pkg := profile.PackageEntry{
		Name:    "ripgrep",
		Check:   profile.InstallCmd{Command: "which rg"},
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, hasCheck := GetCheckCommand(pkg, "macos")
	assert.True(t, hasCheck)
	assert.Equal(t, "which rg", cmd)
}

func TestGetCheckCommand_PerOS(t *testing.T) {
	pkg := profile.PackageEntry{
		Name: "ripgrep",
		Check: profile.InstallCmd{
			PerOS: map[string]string{
				"macos": "which rg",
			},
		},
		Install: profile.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, hasCheck := GetCheckCommand(pkg, "macos")
	assert.True(t, hasCheck)
	assert.Equal(t, "which rg", cmd)

	cmd, hasCheck = GetCheckCommand(pkg, "linux")
	assert.False(t, hasCheck)
	assert.Empty(t, cmd)
}

func TestInstallAll_CheckPassesSkipsInstall(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{
			Name:    "ripgrep",
			Check:   profile.InstallCmd{Command: "which rg"},
			Install: profile.InstallCmd{Command: "brew install ripgrep"},
		},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, StatusAlreadyInstalled, results[0].Status)
	assert.Equal(t, "brew install ripgrep", results[0].Install)
	// Only the check command should have been run, not the install
	assert.Equal(t, []string{"which rg"}, runner.commands)
}

func TestInstallAll_CheckFailsRunsInstall(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["which rg"] = errors.New("exit status 1")
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{
			Name:    "ripgrep",
			Check:   profile.InstallCmd{Command: "which rg"},
			Install: profile.InstallCmd{Command: "brew install ripgrep"},
		},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, StatusOK, results[0].Status)
	// Both check and install commands should have been run
	assert.Equal(t, []string{"which rg", "brew install ripgrep"}, runner.commands)
}

func TestInstallAll_NoCheckRunsInstall(t *testing.T) {
	runner := newMockRunner()
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{
			Name:    "ripgrep",
			Install: profile.InstallCmd{Command: "brew install ripgrep"},
		},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 1)
	assert.Equal(t, StatusOK, results[0].Status)
	// Only the install command should have been run
	assert.Equal(t, []string{"brew install ripgrep"}, runner.commands)
}

func TestInstallAll_MixedCheckResults(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["which lazydocker"] = errors.New("exit status 1")
	inst := NewInstaller(runner, "macos")

	pkgs := []profile.PackageEntry{
		{
			Name:    "ripgrep",
			Check:   profile.InstallCmd{Command: "which rg"},
			Install: profile.InstallCmd{Command: "brew install ripgrep"},
		},
		{
			Name:    "lazydocker",
			Check:   profile.InstallCmd{Command: "which lazydocker"},
			Install: profile.InstallCmd{Command: "brew install lazydocker"},
		},
	}

	results := inst.InstallAll(pkgs)
	require.Len(t, results, 2)
	assert.Equal(t, StatusAlreadyInstalled, results[0].Status)
	assert.Equal(t, StatusOK, results[1].Status)
}

func TestInstallResultFields(t *testing.T) {
	r := PackageResult{Name: "git", Install: "brew install git", Status: "ok"}
	assert.Equal(t, "ok", r.Status)
}
