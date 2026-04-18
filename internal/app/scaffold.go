package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldOpts holds options for the Scaffold operation.
type ScaffoldOpts struct {
	ConfigDir string
	StateDir  string
}

// Scaffold creates a new facet config repository.
func (a *App) Scaffold(opts ScaffoldOpts) error {
	// Check if already initialized
	if _, err := os.Stat(filepath.Join(opts.ConfigDir, "facet.yaml")); err == nil {
		return fmt.Errorf("facet.yaml already exists in %s — already initialized", opts.ConfigDir)
	}

	// Create config repo files
	if err := createConfigRepo(opts.ConfigDir); err != nil {
		return fmt.Errorf("failed to create config repo: %w", err)
	}

	// Create state directory and .local.yaml
	if err := createStateDir(opts.StateDir); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	a.reporter.Success(fmt.Sprintf("Config repo initialized in %s", opts.ConfigDir))
	a.reporter.Success(fmt.Sprintf("State directory at %s", opts.StateDir))
	a.reporter.PrintLine("\nNext steps:")
	a.reporter.PrintLine("  1. Edit base.yaml to add your shared packages and configs")
	a.reporter.PrintLine("  2. Create a profile in profiles/ for this machine")
	a.reporter.PrintLine("  3. Edit ~/.facet/.local.yaml to add machine-specific secrets")
	a.reporter.PrintLine("  4. Run: facet apply <profile>")

	return nil
}

func createConfigRepo(dir string) error {
	facetYAML := `min_version: "0.1.0"
`
	if err := os.WriteFile(filepath.Join(dir, "facet.yaml"), []byte(facetYAML), 0o644); err != nil {
		return err
	}

	baseYAML := `# Base configuration — shared across all profiles.
# Profiles can extend this with 'extends: ./base.yaml' or another supported locator.

# vars:
#   git_name: Your Name

# packages:
#   Common install patterns:
#     Homebrew:         brew install <name>
#     Homebrew cask:    brew install --cask <name>
#     apt (auto-yes):   sudo apt-get install -y <name>
#     npm global:       npm install -g <package>
#     go install:       go install github.com/user/repo@latest
#     pip:              pip install <package>
#     cargo:            cargo install <name>
#
#   - name: ripgrep
#     install: brew install ripgrep
#
#   - name: lazydocker
#     install:
#       macos: brew install lazydocker
#       linux: go install github.com/jesseduffield/lazydocker@latest

# configs:
#   ~/.gitconfig: configs/.gitconfig
#   ~/.zshrc: configs/.zshrc
`
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(baseYAML), 0o644); err != nil {
		return err
	}

	for _, d := range []string{"profiles", "configs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}

	return nil
}

func createStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	localPath := filepath.Join(dir, ".local.yaml")
	if _, err := os.Stat(localPath); errors.Is(err, os.ErrNotExist) {
		localYAML := `# Machine-specific variables (secrets, local paths, etc.)
# This file must exist. Add your machine-specific vars here.
#
# vars:
#   acme_db_url: postgres://user:pass@localhost:5432/acme
`
		if err := os.WriteFile(localPath, []byte(localYAML), 0o644); err != nil {
			return err
		}
	}

	return nil
}
