package packages

import (
	"fmt"
	"runtime"

	"facet/internal/profile"
)

// Package install status constants.
const (
	StatusOK               = "ok"
	StatusFailed           = "failed"
	StatusSkipped          = "skipped"
	StatusAlreadyInstalled = "already_installed"
)

// PackageResult records the result of a single package install.
type PackageResult struct {
	Name    string `json:"name"`
	Install string `json:"install"`
	Status  string `json:"status"` // StatusOK, StatusFailed, or StatusSkipped
	Error   string `json:"error,omitempty"`
}

// DetectOS returns "macos" or "linux".
func DetectOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "linux"
}

// GetInstallCommand returns the install command for the given OS.
// Returns the command and whether to skip (true = skip, no command for this OS).
func GetInstallCommand(pkg profile.PackageEntry, osName string) (string, bool) {
	cmd, ok := pkg.Install.ForOS(osName)
	if !ok {
		return "", true
	}
	return cmd, false
}

// GetCheckCommand returns the check command for the given OS.
// Returns the command and whether a check is defined.
// If no check is defined, returns ("", false).
func GetCheckCommand(pkg profile.PackageEntry, osName string) (string, bool) {
	if pkg.Check.Command == "" && pkg.Check.PerOS == nil {
		return "", false
	}
	cmd, ok := pkg.Check.ForOS(osName)
	return cmd, ok
}

// Installer handles package installation using an injected CommandRunner.
type Installer struct {
	runner CommandRunner
	osName string
}

// NewInstaller creates an Installer with the given runner and OS name.
func NewInstaller(runner CommandRunner, osName string) *Installer {
	return &Installer{runner: runner, osName: osName}
}

// InstallAll runs install commands for all packages.
// If a package has a check command that succeeds, installation is skipped.
// Failed installs are recorded but do not stop other installations.
func (inst *Installer) InstallAll(pkgs []profile.PackageEntry) []PackageResult {
	results := make([]PackageResult, 0, len(pkgs))

	for _, pkg := range pkgs {
		cmd, skip := GetInstallCommand(pkg, inst.osName)

		pr := PackageResult{
			Name:    pkg.Name,
			Install: cmd,
		}

		if skip {
			pr.Status = StatusSkipped
			pr.Error = fmt.Sprintf("no install command for OS %q", inst.osName)
			results = append(results, pr)
			continue
		}

		// Run check command if defined — skip install if check succeeds
		checkCmd, hasCheck := GetCheckCommand(pkg, inst.osName)
		if hasCheck && inst.runner.Run(checkCmd) == nil {
			pr.Status = StatusAlreadyInstalled
			results = append(results, pr)
			continue
		}

		err := inst.runner.Run(cmd)
		if err != nil {
			pr.Status = StatusFailed
			pr.Error = err.Error()
		} else {
			pr.Status = StatusOK
		}

		results = append(results, pr)
	}

	return results
}
