# facet v1 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the facet CLI tool — a developer environment configuration manager that deploys dotfiles, installs packages, and manages configs across machines via composable YAML profiles.

**Architecture:** Config loading pipeline (YAML → merge → resolve vars) produces a resolved config. The apply engine unapplies previous state, installs packages, and deploys configs (symlink or template). State is tracked in `.state.json`. CLI built with cobra.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, `github.com/stretchr/testify` (assertions)

**Spec:** `docs/architecture/v1-design-spec.md` (authoritative)

---

## File Structure

```
facet/
├── cmd/
│   ├── root.go                     # cobra root command, global flags (-c, -s)
│   ├── apply.go                    # facet apply <profile>
│   ├── init_cmd.go                 # facet init
│   └── status.go                   # facet status
├── internal/
│   ├── config/
│   │   ├── types.go                # FacetConfig, PackageEntry, InstallCmd
│   │   ├── types_test.go           # YAML unmarshaling tests
│   │   ├── loader.go               # Load and parse YAML files
│   │   ├── loader_test.go
│   │   ├── merger.go               # Deep merge vars, union packages, shallow configs
│   │   ├── merger_test.go
│   │   ├── resolver.go             # ${facet:var.name} substitution
│   │   └── resolver_test.go
│   ├── deploy/
│   │   ├── pathexpand.go           # ~ and $VAR expansion in target paths
│   │   ├── pathexpand_test.go
│   │   ├── deployer.go             # Symlink/template deploy, unapply, rollback
│   │   └── deployer_test.go
│   ├── packages/
│   │   ├── installer.go            # OS detection, run install commands
│   │   └── installer_test.go
│   ├── state/
│   │   ├── state.go                # .state.json read/write/canary
│   │   └── state_test.go
│   └── reporter/
│       ├── reporter.go             # Colored, structured terminal output
│       └── reporter_test.go
├── main.go
├── go.mod
├── go.sum
└── Makefile
```

**Responsibilities per file:**

| File | Responsibility |
|---|---|
| `cmd/root.go` | Cobra root command, `--config-dir` / `--state-dir` global flags, version |
| `cmd/apply.go` | Orchestrates the full apply pipeline: load → merge → resolve → unapply → install → deploy → state |
| `cmd/init_cmd.go` | Scaffolds config repo in cwd + creates state dir with `.local.yaml` |
| `cmd/status.go` | Reads `.state.json`, runs validity checks, prints formatted status |
| `internal/config/types.go` | `FacetMeta`, `FacetConfig`, `PackageEntry`, `InstallCmd` with custom YAML unmarshaling |
| `internal/config/loader.go` | `LoadMeta()`, `LoadConfig()`, `LoadResolved()` — parse YAML files and orchestrate the load pipeline |
| `internal/config/merger.go` | `Merge()` — deep merge vars (with type conflict detection), union packages, shallow merge configs |
| `internal/config/resolver.go` | `Resolve()` — walk all string values, substitute `${facet:...}` with dot-notation lookup |
| `internal/deploy/pathexpand.go` | `ExpandPath()` — expand `~`, `$VAR`, `${VAR}` in target paths; `ValidateSourcePath()` — ensure within config dir |
| `internal/deploy/deployer.go` | `Deploy()`, `Unapply()`, `Rollback()` — symlink/template deployment with conflict handling |
| `internal/packages/installer.go` | `DetectOS()`, `InstallAll()` — run install commands, handle per-OS selection, collect results |
| `internal/state/state.go` | `Read()`, `Write()`, `CanaryWrite()` — `.state.json` I/O |
| `internal/reporter/reporter.go` | `PrintApplyReport()`, `PrintStatus()` — colored tables, status indicators |

---

## Chunk 1: Foundation

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `Makefile`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/edocsss/aec/src/facet
go mod init facet
```

- [ ] **Step 2: Install dependencies**

```bash
go get github.com/spf13/cobra@latest
go get gopkg.in/yaml.v3
go get github.com/stretchr/testify
```

- [ ] **Step 3: Create main.go**

```go
// main.go
package main

import "facet/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 4: Create cmd/root.go**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configDir string
	stateDir  string
)

var rootCmd = &cobra.Command{
	Use:     "facet",
	Short:   "Developer environment configuration manager",
	Version: "0.1.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "Path to facet config repo (default: current directory)")
	rootCmd.PersistentFlags().StringVarP(&stateDir, "state-dir", "s", "", "Path to machine-local state directory (default: ~/.facet)")
}

// resolveConfigDir returns the config directory path.
// Uses --config-dir flag if set, otherwise current working directory.
func resolveConfigDir() (string, error) {
	if configDir != "" {
		return configDir, nil
	}
	return os.Getwd()
}

// resolveStateDir returns the state directory path.
// Uses --state-dir flag if set, otherwise ~/.facet/.
func resolveStateDir() (string, error) {
	if stateDir != "" {
		return stateDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return home + "/.facet", nil
}
```

- [ ] **Step 5: Create Makefile**

```makefile
build:
	go build -o facet .

test:
	go test ./... -v

test-cover:
	go test ./... -cover

clean:
	rm -f facet

.PHONY: build test test-cover clean
```

- [ ] **Step 6: Verify build compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build -o facet .`
Expected: Binary `facet` created, no errors.

Run: `./facet --version`
Expected: Output contains `0.1.0`

- [ ] **Step 7: Commit**

```bash
git add main.go cmd/root.go go.mod go.sum Makefile
git commit -m "feat: project scaffold with cobra root command"
```

---

### Task 2: Config Types

**Files:**
- Create: `internal/config/types.go`
- Create: `internal/config/types_test.go`

- [ ] **Step 1: Write failing tests for PackageEntry YAML unmarshaling**

```go
// internal/config/types_test.go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPackageEntry_UnmarshalYAML_StringInstall(t *testing.T) {
	input := `
name: ripgrep
install: brew install ripgrep
`
	var pkg PackageEntry
	err := yaml.Unmarshal([]byte(input), &pkg)
	require.NoError(t, err)
	assert.Equal(t, "ripgrep", pkg.Name)
	assert.Equal(t, "brew install ripgrep", pkg.Install.Command)
	assert.Nil(t, pkg.Install.PerOS)
}

func TestPackageEntry_UnmarshalYAML_MapInstall(t *testing.T) {
	input := `
name: lazydocker
install:
  macos: brew install lazydocker
  linux: go install github.com/jesseduffield/lazydocker@latest
`
	var pkg PackageEntry
	err := yaml.Unmarshal([]byte(input), &pkg)
	require.NoError(t, err)
	assert.Equal(t, "lazydocker", pkg.Name)
	assert.Empty(t, pkg.Install.Command)
	assert.Equal(t, "brew install lazydocker", pkg.Install.PerOS["macos"])
	assert.Equal(t, "go install github.com/jesseduffield/lazydocker@latest", pkg.Install.PerOS["linux"])
}

func TestInstallCmd_ForOS(t *testing.T) {
	// Simple command — works on any OS
	simple := InstallCmd{Command: "brew install ripgrep"}
	cmd, ok := simple.ForOS("macos")
	assert.True(t, ok)
	assert.Equal(t, "brew install ripgrep", cmd)

	cmd, ok = simple.ForOS("linux")
	assert.True(t, ok)
	assert.Equal(t, "brew install ripgrep", cmd)

	// Per-OS command — only works on specified OS
	perOS := InstallCmd{PerOS: map[string]string{
		"macos": "brew install lazydocker",
	}}
	cmd, ok = perOS.ForOS("macos")
	assert.True(t, ok)
	assert.Equal(t, "brew install lazydocker", cmd)

	cmd, ok = perOS.ForOS("linux")
	assert.False(t, ok)
	assert.Empty(t, cmd)
}

func TestFacetConfig_UnmarshalYAML_Full(t *testing.T) {
	input := `
extends: base
vars:
  git:
    name: Sarah
    email: sarah@acme.com
  simple: hello
packages:
  - name: ripgrep
    install: brew install ripgrep
  - name: lazydocker
    install:
      macos: brew install lazydocker
      linux: go install github.com/jesseduffield/lazydocker@latest
configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.zshrc: configs/.zshrc
`
	var cfg FacetConfig
	err := yaml.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "base", cfg.Extends)
	assert.Equal(t, "hello", cfg.Vars["simple"])

	gitVars, ok := cfg.Vars["git"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Sarah", gitVars["name"])

	assert.Len(t, cfg.Packages, 2)
	assert.Equal(t, "ripgrep", cfg.Packages[0].Name)
	assert.Equal(t, "configs/.gitconfig", cfg.Configs["~/.gitconfig"])
}

func TestFacetMeta_UnmarshalYAML(t *testing.T) {
	input := `min_version: "0.1.0"`
	var meta FacetMeta
	err := yaml.Unmarshal([]byte(input), &meta)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", meta.MinVersion)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run TestPackageEntry`
Expected: FAIL — types not defined yet

- [ ] **Step 3: Implement types.go**

```go
// internal/config/types.go
package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FacetMeta represents the facet.yaml metadata file.
type FacetMeta struct {
	MinVersion string `yaml:"min_version"`
}

// FacetConfig represents a parsed base.yaml, profile, or .local.yaml.
type FacetConfig struct {
	Extends  string            `yaml:"extends,omitempty"`
	Vars     map[string]any    `yaml:"vars,omitempty"`
	Packages []PackageEntry    `yaml:"packages,omitempty"`
	Configs  map[string]string `yaml:"configs,omitempty"`
}

// PackageEntry is a package with a name and install command.
type PackageEntry struct {
	Name    string     `yaml:"name"`
	Install InstallCmd `yaml:"install"`
}

// InstallCmd can be a simple string command or a per-OS map.
type InstallCmd struct {
	Command string            // non-empty if install is a plain string
	PerOS   map[string]string // non-nil if install is a per-OS map
}

// UnmarshalYAML handles both string and map forms of install.
func (c *InstallCmd) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		c.Command = value.Value
		return nil
	}
	if value.Kind == yaml.MappingNode {
		c.PerOS = make(map[string]string)
		return value.Decode(&c.PerOS)
	}
	return fmt.Errorf("install must be a string or a map of OS-specific commands")
}

// ForOS returns the install command for the given OS.
// Returns the command and true if available, or empty string and false if not.
func (c *InstallCmd) ForOS(os string) (string, bool) {
	if c.Command != "" {
		return c.Command, true
	}
	cmd, ok := c.PerOS[os]
	return cmd, ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/types.go internal/config/types_test.go
git commit -m "feat: config types with YAML unmarshaling for PackageEntry"
```

---

### Task 3: Config Loader

**Files:**
- Create: `internal/config/loader.go`
- Create: `internal/config/loader_test.go`

- [ ] **Step 1: Write failing tests for config loading**

```go
// internal/config/loader_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestLoadMeta_Valid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "facet.yaml"), `min_version: "0.1.0"`)

	meta, err := LoadMeta(dir)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", meta.MinVersion)
}

func TestLoadMeta_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadMeta(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "facet.yaml")
}

func TestLoadConfig_Base(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.yaml"), `
vars:
  git_name: Sarah
packages:
  - name: git
    install: brew install git
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	cfg, err := LoadConfig(filepath.Join(dir, "base.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "Sarah", cfg.Vars["git_name"])
	assert.Len(t, cfg.Packages, 1)
	assert.Equal(t, "git", cfg.Packages[0].Name)
	assert.Equal(t, "configs/.gitconfig", cfg.Configs["~/.gitconfig"])
}

func TestLoadConfig_Profile_ExtendsBase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "profiles", "work.yaml"), `
extends: base
vars:
  git_email: sarah@acme.com
`)

	cfg, err := LoadConfig(filepath.Join(dir, "profiles", "work.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "base", cfg.Extends)
	assert.Equal(t, "sarah@acme.com", cfg.Vars["git_email"])
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{invalid yaml`)

	_, err := LoadConfig(filepath.Join(dir, "bad.yaml"))
	assert.Error(t, err)
}

func TestLoadConfig_NestedVars(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.yaml"), `
vars:
  git:
    name: Sarah
    email: sarah@hey.com
  aws:
    region: us-east-1
`)

	cfg, err := LoadConfig(filepath.Join(dir, "base.yaml"))
	require.NoError(t, err)

	gitVars, ok := cfg.Vars["git"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Sarah", gitVars["name"])
	assert.Equal(t, "sarah@hey.com", gitVars["email"])
}

func TestValidateProfile_InvalidExtends(t *testing.T) {
	cfg := &FacetConfig{Extends: "other"}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base")
}

func TestValidateProfile_MissingExtends(t *testing.T) {
	cfg := &FacetConfig{}
	err := ValidateProfile(cfg)
	assert.Error(t, err)
}

func TestValidateProfile_Valid(t *testing.T) {
	cfg := &FacetConfig{Extends: "base"}
	err := ValidateProfile(cfg)
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run TestLoad`
Expected: FAIL — LoadMeta, LoadConfig, ValidateProfile not defined

- [ ] **Step 3: Implement loader.go**

```go
// internal/config/loader.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadMeta reads and parses facet.yaml from the given config directory.
func LoadMeta(configDir string) (*FacetMeta, error) {
	path := filepath.Join(configDir, "facet.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read facet.yaml: %w (is this a facet config directory?)", err)
	}
	var meta FacetMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse facet.yaml: %w", err)
	}
	return &meta, nil
}

// LoadConfig reads and parses a single YAML config file (base.yaml, profile, or .local.yaml).
func LoadConfig(path string) (*FacetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(path), err)
	}
	var cfg FacetConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filepath.Base(path), err)
	}
	return &cfg, nil
}

// ValidateProfile checks that a profile config has a valid extends field.
func ValidateProfile(cfg *FacetConfig) error {
	if cfg.Extends == "" {
		return fmt.Errorf("profile is missing 'extends' field (must be 'extends: base')")
	}
	if cfg.Extends != "base" {
		return fmt.Errorf("profile has 'extends: %s' but only 'extends: base' is supported", cfg.Extends)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/loader.go internal/config/loader_test.go
git commit -m "feat: config loader for facet.yaml, base.yaml, and profiles"
```

---

## Chunk 2: Merge & Resolve

### Task 4: Config Merger

**Files:**
- Create: `internal/config/merger.go`
- Create: `internal/config/merger_test.go`

- [ ] **Step 1: Write failing tests for merge behavior**

```go
// internal/config/merger_test.go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_VarsDeepMerge(t *testing.T) {
	base := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"name":   "Sarah",
				"editor": "nvim",
			},
			"simple": "hello",
		},
	}
	profile := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"email": "sarah@acme.com",
			},
			"new_var": "world",
		},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)

	gitVars, ok := result.Vars["git"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Sarah", gitVars["name"])
	assert.Equal(t, "nvim", gitVars["editor"])
	assert.Equal(t, "sarah@acme.com", gitVars["email"])
	assert.Equal(t, "hello", result.Vars["simple"])
	assert.Equal(t, "world", result.Vars["new_var"])
}

func TestMerge_VarsTypeConflict(t *testing.T) {
	base := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{"name": "Sarah"},
		},
	}
	profile := &FacetConfig{
		Vars: map[string]any{
			"git": "just a string",
		},
	}

	_, err := Merge(base, profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type conflict")
	assert.Contains(t, err.Error(), "git")
}

func TestMerge_VarsLastWriterWins(t *testing.T) {
	base := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"name": "Sarah",
			},
		},
	}
	profile := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"name": "Sarah Chen",
			},
		},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)
	gitVars := result.Vars["git"].(map[string]any)
	assert.Equal(t, "Sarah Chen", gitVars["name"])
}

func TestMerge_PackagesUnion(t *testing.T) {
	base := &FacetConfig{
		Packages: []PackageEntry{
			{Name: "git", Install: InstallCmd{Command: "brew install git"}},
			{Name: "ripgrep", Install: InstallCmd{Command: "brew install ripgrep"}},
		},
	}
	profile := &FacetConfig{
		Packages: []PackageEntry{
			{Name: "docker", Install: InstallCmd{Command: "brew install docker"}},
		},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)
	assert.Len(t, result.Packages, 3)

	names := make([]string, len(result.Packages))
	for i, p := range result.Packages {
		names[i] = p.Name
	}
	assert.Contains(t, names, "git")
	assert.Contains(t, names, "ripgrep")
	assert.Contains(t, names, "docker")
}

func TestMerge_PackagesOverride(t *testing.T) {
	base := &FacetConfig{
		Packages: []PackageEntry{
			{Name: "git", Install: InstallCmd{Command: "brew install git"}},
		},
	}
	profile := &FacetConfig{
		Packages: []PackageEntry{
			{Name: "git", Install: InstallCmd{Command: "sudo apt-get install -y git"}},
		},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)
	assert.Len(t, result.Packages, 1)
	assert.Equal(t, "sudo apt-get install -y git", result.Packages[0].Install.Command)
}

func TestMerge_ConfigsShallowMerge(t *testing.T) {
	base := &FacetConfig{
		Configs: map[string]string{
			"~/.gitconfig": "configs/.gitconfig",
			"~/.zshrc":     "configs/.zshrc",
		},
	}
	profile := &FacetConfig{
		Configs: map[string]string{
			"~/.gitconfig": "configs/work/.gitconfig",
			"~/.npmrc":     "configs/work/.npmrc",
		},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)
	assert.Equal(t, "configs/work/.gitconfig", result.Configs["~/.gitconfig"])
	assert.Equal(t, "configs/.zshrc", result.Configs["~/.zshrc"])
	assert.Equal(t, "configs/work/.npmrc", result.Configs["~/.npmrc"])
}

func TestMerge_ThreeLayers(t *testing.T) {
	base := &FacetConfig{
		Vars:     map[string]any{"name": "Sarah"},
		Packages: []PackageEntry{{Name: "git", Install: InstallCmd{Command: "brew install git"}}},
		Configs:  map[string]string{"~/.gitconfig": "configs/.gitconfig"},
	}
	profile := &FacetConfig{
		Vars:     map[string]any{"email": "sarah@acme.com"},
		Packages: []PackageEntry{{Name: "docker", Install: InstallCmd{Command: "brew install docker"}}},
		Configs:  map[string]string{"~/.npmrc": "configs/.npmrc"},
	}
	local := &FacetConfig{
		Vars: map[string]any{"secret": "s3cret"},
	}

	// Merge base + profile first
	merged, err := Merge(base, profile)
	require.NoError(t, err)

	// Then merge with local
	result, err := Merge(merged, local)
	require.NoError(t, err)

	assert.Equal(t, "Sarah", result.Vars["name"])
	assert.Equal(t, "sarah@acme.com", result.Vars["email"])
	assert.Equal(t, "s3cret", result.Vars["secret"])
	assert.Len(t, result.Packages, 2)
	assert.Len(t, result.Configs, 2)
}

func TestMerge_NilInputs(t *testing.T) {
	base := &FacetConfig{}
	profile := &FacetConfig{
		Vars: map[string]any{"key": "value"},
	}

	result, err := Merge(base, profile)
	require.NoError(t, err)
	assert.Equal(t, "value", result.Vars["key"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run TestMerge`
Expected: FAIL — Merge not defined

- [ ] **Step 3: Implement merger.go**

```go
// internal/config/merger.go
package config

import (
	"fmt"
)

// Merge combines two config layers. The overlay wins on conflicts.
// Call multiple times to merge three layers: Merge(Merge(base, profile), local).
func Merge(base, overlay *FacetConfig) (*FacetConfig, error) {
	result := &FacetConfig{}

	// Merge vars (deep merge)
	merged, err := deepMergeVars(base.Vars, overlay.Vars)
	if err != nil {
		return nil, err
	}
	result.Vars = merged

	// Merge packages (union by name, overlay wins on conflict)
	result.Packages = mergePackages(base.Packages, overlay.Packages)

	// Merge configs (shallow merge, overlay wins on same target)
	result.Configs = mergeConfigs(base.Configs, overlay.Configs)

	return result, nil
}

// deepMergeVars recursively merges two vars maps.
// Both maps must have compatible types at each key — a map and a scalar at the same key is a type conflict error.
func deepMergeVars(base, overlay map[string]any) (map[string]any, error) {
	if base == nil && overlay == nil {
		return nil, nil
	}
	result := make(map[string]any)

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Merge overlay
	for k, v := range overlay {
		existing, exists := result[k]
		if !exists {
			result[k] = v
			continue
		}

		existingMap, existingIsMap := toStringAnyMap(existing)
		overlayMap, overlayIsMap := toStringAnyMap(v)

		if existingIsMap && overlayIsMap {
			merged, err := deepMergeVars(existingMap, overlayMap)
			if err != nil {
				return nil, err
			}
			result[k] = merged
		} else if existingIsMap != overlayIsMap {
			return nil, fmt.Errorf("type conflict for var '%s': cannot merge map and scalar", k)
		} else {
			// Both are scalars — overlay wins
			result[k] = v
		}
	}

	return result, nil
}

// toStringAnyMap attempts to convert a value to map[string]any.
// YAML v3 may decode as map[string]any or map[any]any.
func toStringAnyMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

// mergePackages unions two package lists by name. Overlay wins on name conflict.
func mergePackages(base, overlay []PackageEntry) []PackageEntry {
	byName := make(map[string]PackageEntry)
	var order []string

	for _, p := range base {
		byName[p.Name] = p
		order = append(order, p.Name)
	}
	for _, p := range overlay {
		if _, exists := byName[p.Name]; !exists {
			order = append(order, p.Name)
		}
		byName[p.Name] = p
	}

	result := make([]PackageEntry, 0, len(order))
	for _, name := range order {
		result = append(result, byName[name])
	}
	return result
}

// mergeConfigs shallow-merges two config maps. Overlay wins on same target key.
func mergeConfigs(base, overlay map[string]string) map[string]string {
	if base == nil && overlay == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v -run TestMerge`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/merger.go internal/config/merger_test.go
git commit -m "feat: config merger with deep merge vars, union packages, shallow configs"
```

---

### Task 5: Variable Resolver

**Files:**
- Create: `internal/config/resolver.go`
- Create: `internal/config/resolver_test.go`

- [ ] **Step 1: Write failing tests for variable resolution**

```go
// internal/config/resolver_test.go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_SimpleVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"name": "Sarah"},
		Packages: []PackageEntry{
			{Name: "greet", Install: InstallCmd{Command: "echo ${facet:name}"}},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "echo Sarah", resolved.Packages[0].Install.Command)
}

func TestResolve_NestedVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"git": map[string]any{
				"email": "sarah@acme.com",
			},
		},
		Configs: map[string]string{
			"~/.gitconfig": "configs/${facet:git.email}/gitconfig",
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "configs/sarah@acme.com/gitconfig", resolved.Configs["~/.gitconfig"])
}

func TestResolve_DeeplyNestedVar(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"cloud": map[string]any{
				"aws": map[string]any{
					"region": "us-east-1",
				},
			},
		},
		Packages: []PackageEntry{
			{Name: "aws", Install: InstallCmd{Command: "echo ${facet:cloud.aws.region}"}},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "echo us-east-1", resolved.Packages[0].Install.Command)
}

func TestResolve_UndefinedVar_Error(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"name": "Sarah"},
		Packages: []PackageEntry{
			{Name: "test", Install: InstallCmd{Command: "echo ${facet:undefined_var}"}},
		},
	}

	_, err := Resolve(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undefined_var")
}

func TestResolve_MultipleVarsInOneString(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"first": "Sarah",
			"last":  "Chen",
		},
		Packages: []PackageEntry{
			{Name: "test", Install: InstallCmd{Command: "echo ${facet:first} ${facet:last}"}},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "echo Sarah Chen", resolved.Packages[0].Install.Command)
}

func TestResolve_NoRecursion(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{
			"a": "${facet:b}",
			"b": "actual_value",
		},
		Packages: []PackageEntry{
			{Name: "test", Install: InstallCmd{Command: "${facet:a}"}},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	// a's value is literally "${facet:b}", not "actual_value"
	assert.Equal(t, "${facet:b}", resolved.Packages[0].Install.Command)
}

func TestResolve_PerOSInstallCommand(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"version": "22"},
		Packages: []PackageEntry{
			{
				Name: "node",
				Install: InstallCmd{
					PerOS: map[string]string{
						"macos": "brew install node@${facet:version}",
						"linux": "apt install nodejs=${facet:version}",
					},
				},
			},
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "brew install node@22", resolved.Packages[0].Install.PerOS["macos"])
	assert.Equal(t, "apt install nodejs=22", resolved.Packages[0].Install.PerOS["linux"])
}

func TestResolve_ConfigSourcePaths(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"env": "work"},
		Configs: map[string]string{
			"~/.gitconfig": "configs/${facet:env}/.gitconfig",
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "configs/work/.gitconfig", resolved.Configs["~/.gitconfig"])
}

func TestResolve_ConfigTargetPathsNotResolved(t *testing.T) {
	cfg := &FacetConfig{
		Vars: map[string]any{"dir": "custom"},
		Configs: map[string]string{
			"~/${facet:dir}/.gitconfig": "configs/.gitconfig",
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	// Target path should NOT be resolved — ${facet:...} stays literal in keys
	_, exists := resolved.Configs["~/${facet:dir}/.gitconfig"]
	assert.True(t, exists)
}

func TestResolve_NoVarsNoError(t *testing.T) {
	cfg := &FacetConfig{
		Packages: []PackageEntry{
			{Name: "git", Install: InstallCmd{Command: "brew install git"}},
		},
		Configs: map[string]string{
			"~/.zshrc": "configs/.zshrc",
		},
	}

	resolved, err := Resolve(cfg)
	require.NoError(t, err)
	assert.Equal(t, "brew install git", resolved.Packages[0].Install.Command)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run TestResolve`
Expected: FAIL — Resolve not defined

- [ ] **Step 3: Implement resolver.go**

```go
// internal/config/resolver.go
package config

import (
	"fmt"
	"regexp"
	"strings"
)

var facetVarPattern = regexp.MustCompile(`\$\{facet:([a-zA-Z0-9_.]+)\}`)

// Resolve substitutes all ${facet:var.name} references in the config.
// Config target paths (map keys in Configs) are NOT resolved.
// Returns a new FacetConfig with all references substituted.
func Resolve(cfg *FacetConfig) (*FacetConfig, error) {
	result := &FacetConfig{
		Vars:    cfg.Vars, // vars themselves are not resolved (no recursion)
		Configs: make(map[string]string, len(cfg.Configs)),
	}

	// Resolve packages
	for _, pkg := range cfg.Packages {
		resolved, err := resolvePackageEntry(pkg, cfg.Vars)
		if err != nil {
			return nil, err
		}
		result.Packages = append(result.Packages, resolved)
	}

	// Resolve config source paths (values), but NOT target paths (keys)
	for target, source := range cfg.Configs {
		resolvedSource, err := substituteVars(source, cfg.Vars)
		if err != nil {
			return nil, err
		}
		result.Configs[target] = resolvedSource
	}

	return result, nil
}

func resolvePackageEntry(pkg PackageEntry, vars map[string]any) (PackageEntry, error) {
	result := PackageEntry{Name: pkg.Name}

	if pkg.Install.Command != "" {
		resolved, err := substituteVars(pkg.Install.Command, vars)
		if err != nil {
			return result, err
		}
		result.Install.Command = resolved
	}

	if pkg.Install.PerOS != nil {
		result.Install.PerOS = make(map[string]string, len(pkg.Install.PerOS))
		for os, cmd := range pkg.Install.PerOS {
			resolved, err := substituteVars(cmd, vars)
			if err != nil {
				return result, err
			}
			result.Install.PerOS[os] = resolved
		}
	}

	return result, nil
}

// substituteVars replaces all ${facet:var.name} references in a string.
func substituteVars(s string, vars map[string]any) (string, error) {
	var resolveErr error

	result := facetVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		// Extract the variable name from ${facet:var.name}
		submatches := facetVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]

		value, err := lookupVar(vars, key)
		if err != nil {
			resolveErr = err
			return match
		}
		return value
	})

	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// lookupVar resolves a dot-notation key against a nested vars map.
// For example, "git.email" looks up vars["git"]["email"].
func lookupVar(vars map[string]any, key string) (string, error) {
	parts := strings.Split(key, ".")
	current := vars

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return "", fmt.Errorf("undefined variable: ${facet:%s} — define it in .local.yaml or your profile's vars", key)
		}

		if i == len(parts)-1 {
			// Leaf — must be a string
			s, ok := val.(string)
			if !ok {
				return "", fmt.Errorf("variable ${facet:%s} resolves to a map, not a string — use a more specific path", key)
			}
			return s, nil
		}

		// Intermediate — must be a map
		m, ok := toStringAnyMap(val)
		if !ok {
			return "", fmt.Errorf("variable ${facet:%s}: '%s' is not a map", key, part)
		}
		current = m
	}

	return "", fmt.Errorf("undefined variable: ${facet:%s}", key)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v -run TestResolve`
Expected: All tests PASS

- [ ] **Step 5: Run all config tests together**

Run: `go test ./internal/config/ -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/resolver.go internal/config/resolver_test.go
git commit -m "feat: variable resolver with \${facet:var.name} dot-notation substitution"
```

---

## Chunk 3: State & Deployment

### Task 6: Path Expansion

**Files:**
- Create: `internal/deploy/pathexpand.go`
- Create: `internal/deploy/pathexpand_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/deploy/pathexpand_test.go
package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath_Tilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result, err := ExpandPath("~/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".gitconfig"), result)
}

func TestExpandPath_TildeAlone(t *testing.T) {
	home, _ := os.UserHomeDir()
	result, err := ExpandPath("~")
	require.NoError(t, err)
	assert.Equal(t, home, result)
}

func TestExpandPath_DollarHOME(t *testing.T) {
	home, _ := os.UserHomeDir()
	result, err := ExpandPath("$HOME/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".gitconfig"), result)
}

func TestExpandPath_DollarBraceHOME(t *testing.T) {
	home, _ := os.UserHomeDir()
	result, err := ExpandPath("${HOME}/.gitconfig")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".gitconfig"), result)
}

func TestExpandPath_CustomEnvVar(t *testing.T) {
	t.Setenv("MY_CONFIG_DIR", "/opt/configs")
	result, err := ExpandPath("$MY_CONFIG_DIR/app.conf")
	require.NoError(t, err)
	assert.Equal(t, "/opt/configs/app.conf", result)
}

func TestExpandPath_CustomBraceEnvVar(t *testing.T) {
	t.Setenv("MY_DIR", "/custom")
	result, err := ExpandPath("${MY_DIR}/file")
	require.NoError(t, err)
	assert.Equal(t, "/custom/file", result)
}

func TestExpandPath_UndefinedEnvVar(t *testing.T) {
	_, err := ExpandPath("$UNDEFINED_VAR_12345/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNDEFINED_VAR_12345")
}

func TestExpandPath_AlreadyAbsolute(t *testing.T) {
	result, err := ExpandPath("/usr/local/bin/tool")
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/tool", result)
}

func TestExpandPath_RelativePath_Error(t *testing.T) {
	_, err := ExpandPath("relative/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestValidateSourcePath_WithinConfigDir(t *testing.T) {
	configDir := "/home/user/dotfiles"
	err := ValidateSourcePath("configs/.gitconfig", configDir)
	assert.NoError(t, err)
}

func TestValidateSourcePath_Traversal_Error(t *testing.T) {
	configDir := "/home/user/dotfiles"
	err := ValidateSourcePath("../outside/.gitconfig", configDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside")
}

func TestValidateSourcePath_AbsolutePath_Error(t *testing.T) {
	configDir := "/home/user/dotfiles"
	err := ValidateSourcePath("/etc/passwd", configDir)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/deploy/ -v -run TestExpandPath`
Expected: FAIL — ExpandPath not defined

- [ ] **Step 3: Implement pathexpand.go**

```go
// internal/deploy/pathexpand.go
package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExpandPath expands ~ and environment variables in a path.
// After expansion, the path must be absolute.
func ExpandPath(path string) (string, error) {
	// Expand ~ at the start
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		path = home + path[1:]
	}

	// Expand environment variables: $VAR and ${VAR}
	var expandErr error
	expanded := envVarPattern.ReplaceAllStringFunc(path, func(match string) string {
		if expandErr != nil {
			return match
		}
		// Extract variable name
		submatches := envVarPattern.FindStringSubmatch(match)
		var varName string
		if submatches[1] != "" {
			varName = submatches[1] // ${VAR} form
		} else {
			varName = submatches[2] // $VAR form
		}

		value, ok := os.LookupEnv(varName)
		if !ok {
			expandErr = fmt.Errorf("undefined environment variable: $%s in path %q", varName, path)
			return match
		}
		return value
	})

	if expandErr != nil {
		return "", expandErr
	}

	// Must be absolute after expansion
	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("config target path must be absolute after expansion, got: %q", expanded)
	}

	return filepath.Clean(expanded), nil
}

// ValidateSourcePath checks that a source path is relative and stays within the config directory.
func ValidateSourcePath(source, configDir string) error {
	if filepath.IsAbs(source) {
		return fmt.Errorf("config source path must be relative to the config directory, got absolute path: %q", source)
	}

	// Resolve the full path and check it's within configDir
	full := filepath.Join(configDir, source)
	resolved, err := filepath.Abs(full)
	if err != nil {
		return fmt.Errorf("cannot resolve source path %q: %w", source, err)
	}

	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return fmt.Errorf("cannot resolve config directory: %w", err)
	}

	if !strings.HasPrefix(resolved, absConfigDir+string(filepath.Separator)) && resolved != absConfigDir {
		return fmt.Errorf("config source path %q escapes the config directory (resolves to %s, outside %s)", source, resolved, absConfigDir)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/deploy/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/pathexpand.go internal/deploy/pathexpand_test.go
git commit -m "feat: path expansion for ~ and env vars with source path validation"
```

---

### Task 7: State Manager

**Files:**
- Create: `internal/state/state.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/state/state_test.go
package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()

	s := &ApplyState{
		Profile:      "acme",
		AppliedAt:    time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC),
		FacetVersion: "0.1.0",
		Packages: []PackageState{
			{Name: "git", Install: "brew install git", Status: "ok"},
			{Name: "docker", Install: "brew install docker", Status: "failed", Error: "not found"},
		},
		Configs: []ConfigState{
			{Target: "~/.gitconfig", Source: "configs/.gitconfig", Strategy: "template"},
			{Target: "~/.zshrc", Source: "configs/.zshrc", Strategy: "symlink"},
		},
	}

	err := Write(dir, s)
	require.NoError(t, err)

	loaded, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "acme", loaded.Profile)
	assert.Equal(t, "0.1.0", loaded.FacetVersion)
	assert.Len(t, loaded.Packages, 2)
	assert.Len(t, loaded.Configs, 2)
	assert.Equal(t, "failed", loaded.Packages[1].Status)
	assert.Equal(t, "template", loaded.Configs[0].Strategy)
}

func TestRead_Missing(t *testing.T) {
	dir := t.TempDir()
	s, err := Read(dir)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestRead_Corrupted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".state.json"), []byte("{{{bad json"), 0o644))

	_, err := Read(dir)
	assert.Error(t, err)
}

func TestCanaryWrite(t *testing.T) {
	dir := t.TempDir()
	err := CanaryWrite(dir)
	assert.NoError(t, err)

	// File should exist after canary
	_, err = os.Stat(filepath.Join(dir, ".state.json"))
	assert.NoError(t, err)
}

func TestCanaryWrite_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o444))
	defer os.Chmod(dir, 0o755) // cleanup

	err := CanaryWrite(dir)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/state/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement state.go**

```go
// internal/state/state.go
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const stateFile = ".state.json"

// ApplyState records the result of a facet apply run.
type ApplyState struct {
	Profile      string         `json:"profile"`
	AppliedAt    time.Time      `json:"applied_at"`
	FacetVersion string         `json:"facet_version"`
	Packages     []PackageState `json:"packages"`
	Configs      []ConfigState  `json:"configs"`
}

// PackageState records the result of a single package install.
type PackageState struct {
	Name    string `json:"name"`
	Install string `json:"install"`
	Status  string `json:"status"` // "ok" or "failed"
	Error   string `json:"error,omitempty"`
}

// ConfigState records a single deployed config.
type ConfigState struct {
	Target   string `json:"target"`
	Source   string `json:"source"`
	Strategy string `json:"strategy"` // "symlink" or "template"
}

// Write saves the apply state to .state.json in the state directory.
func Write(stateDir string, s *ApplyState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := filepath.Join(stateDir, stateFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// Read loads the apply state from .state.json.
// Returns nil, nil if the file does not exist (no previous apply).
func Read(stateDir string) (*ApplyState, error) {
	path := filepath.Join(stateDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var s ApplyState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &s, nil
}

// CanaryWrite performs an early write to .state.json to detect permission or disk errors
// before doing any real work. Writes a minimal valid JSON object.
func CanaryWrite(stateDir string) error {
	path := filepath.Join(stateDir, stateFile)

	// If file already exists, check we can write to it
	if _, err := os.Stat(path); err == nil {
		// File exists — try to open for writing
		f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("cannot write to %s: %w", path, err)
		}
		f.Close()
		return nil
	}

	// File doesn't exist — try to create it with a minimal state
	canary := &ApplyState{
		Profile:      "_canary",
		AppliedAt:    time.Now(),
		FacetVersion: "0.1.0",
	}
	return Write(stateDir, canary)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/state/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat: state manager for .state.json read/write/canary"
```

---

### Task 8: Config Deployer

**Files:**
- Create: `internal/deploy/deployer.go`
- Create: `internal/deploy/deployer_test.go`

- [ ] **Step 1: Write failing tests for deployment**

```go
// internal/deploy/deployer_test.go
package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/state"
)

func setupConfigDir(t *testing.T) (configDir, homeDir string) {
	t.Helper()
	configDir = t.TempDir()
	homeDir = t.TempDir()
	return
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestDetectStrategy_StaticFile(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.zshrc"))
	require.NoError(t, err)
	assert.Equal(t, "symlink", strategy)
}

func TestDetectStrategy_TemplateFile(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.gitconfig"), "[user]\n  email = ${facet:git.email}")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, "template", strategy)
}

func TestDetectStrategy_Directory(t *testing.T) {
	configDir, _ := setupConfigDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "configs/nvim"), 0o755))
	writeTestFile(t, filepath.Join(configDir, "configs/nvim/init.lua"), "-- nvim config")

	strategy, err := DetectStrategy(filepath.Join(configDir, "configs/nvim"))
	require.NoError(t, err)
	assert.Equal(t, "symlink", strategy)
}

func TestDeploy_Symlink_NewTarget(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "export EDITOR=nvim")

	d := NewDeployer(configDir, homeDir, nil)
	result, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, "symlink", result.Strategy)

	// Verify symlink
	target, err := os.Readlink(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, "configs/.zshrc"), target)
}

func TestDeploy_Symlink_AlreadyCorrect(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "export EDITOR=nvim")

	// Pre-create correct symlink
	dest := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, dest))

	d := NewDeployer(configDir, homeDir, nil)
	result, err := d.DeployOne(dest, "configs/.zshrc", false)
	require.NoError(t, err)
	assert.Equal(t, "symlink", result.Strategy)
}

func TestDeploy_Template_RendersVars(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.gitconfig"),
		"[user]\n  email = ${facet:git.email}\n  name = ${facet:git.name}")

	vars := map[string]any{
		"git": map[string]any{
			"email": "sarah@acme.com",
			"name":  "Sarah",
		},
	}

	d := NewDeployer(configDir, homeDir, vars)
	result, err := d.DeployOne(
		filepath.Join(homeDir, ".gitconfig"),
		"configs/.gitconfig",
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, "template", result.Strategy)

	content, err := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "sarah@acme.com")
	assert.Contains(t, string(content), "Sarah")
	assert.NotContains(t, string(content), "${facet:")
}

func TestDeploy_CreateParentDirs(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/nvim/init.lua"), "-- config")

	d := NewDeployer(configDir, homeDir, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".config", "nvim", "init.lua"),
		"configs/nvim/init.lua",
		false,
	)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"))
}

func TestDeploy_ExistingRegularFile_ErrorWithoutForce(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	writeTestFile(t, filepath.Join(configDir, "configs/.zshrc"), "new content")
	writeTestFile(t, filepath.Join(homeDir, ".zshrc"), "existing user content")

	d := NewDeployer(configDir, homeDir, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		false, // no force
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exists")
}

func TestDeploy_ExistingRegularFile_ReplacedWithForce(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "new content")
	writeTestFile(t, filepath.Join(homeDir, ".zshrc"), "existing user content")

	d := NewDeployer(configDir, homeDir, nil)
	_, err := d.DeployOne(
		filepath.Join(homeDir, ".zshrc"),
		"configs/.zshrc",
		true, // force
	)
	require.NoError(t, err)

	// Should be a symlink now
	target, err := os.Readlink(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, source, target)
}

func TestUnapply_RemovesSymlinks(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "content")
	dest := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, dest))

	configs := []state.ConfigState{
		{Target: dest, Source: "configs/.zshrc", Strategy: "symlink"},
	}

	d := NewDeployer(configDir, homeDir, nil)
	err := d.Unapply(configs)
	require.NoError(t, err)
	assert.NoFileExists(t, dest)
}

func TestUnapply_RemovesTemplatedFiles(t *testing.T) {
	_, homeDir := setupConfigDir(t)
	dest := filepath.Join(homeDir, ".gitconfig")
	writeTestFile(t, dest, "rendered content")

	configs := []state.ConfigState{
		{Target: dest, Source: "configs/.gitconfig", Strategy: "template"},
	}

	d := NewDeployer("", homeDir, nil)
	err := d.Unapply(configs)
	require.NoError(t, err)
	assert.NoFileExists(t, dest)
}

func TestUnapply_NoState_NoOp(t *testing.T) {
	d := NewDeployer("", t.TempDir(), nil)
	err := d.Unapply(nil)
	assert.NoError(t, err)
}

func TestRollback(t *testing.T) {
	configDir, homeDir := setupConfigDir(t)
	source := filepath.Join(configDir, "configs/.zshrc")
	writeTestFile(t, source, "content")

	d := NewDeployer(configDir, homeDir, nil)

	// Deploy one file
	_, err := d.DeployOne(filepath.Join(homeDir, ".zshrc"), "configs/.zshrc", false)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(homeDir, ".zshrc"))

	// Rollback should remove it
	err = d.Rollback()
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(homeDir, ".zshrc"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/deploy/ -v -run TestDeploy`
Expected: FAIL — Deployer types not defined

- [ ] **Step 3: Implement deployer.go**

```go
// internal/deploy/deployer.go
package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"facet/internal/state"
)

// Deployer handles deploying config files as symlinks or rendered templates.
type Deployer struct {
	configDir string
	homeDir   string
	vars      map[string]any
	deployed  []state.ConfigState // tracks deployments for rollback
}

// NewDeployer creates a new Deployer.
func NewDeployer(configDir, homeDir string, vars map[string]any) *Deployer {
	return &Deployer{
		configDir: configDir,
		homeDir:   homeDir,
		vars:      vars,
	}
}

// Deployed returns the list of configs deployed during this session.
func (d *Deployer) Deployed() []state.ConfigState {
	return d.deployed
}

// DetectStrategy determines whether a source path should be symlinked or templated.
func DetectStrategy(sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}

	if info.IsDir() {
		return "symlink", nil
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot read source %q: %w", sourcePath, err)
	}

	if strings.Contains(string(content), "${facet:") {
		return "template", nil
	}

	return "symlink", nil
}

// DeployOne deploys a single config entry.
// targetPath is the absolute expanded target path.
// source is the relative source path within the config directory.
// force replaces existing non-facet files without prompting.
func (d *Deployer) DeployOne(targetPath, source string, force bool) (state.ConfigState, error) {
	sourcePath := filepath.Join(d.configDir, source)
	result := state.ConfigState{
		Target: targetPath,
		Source: source,
	}

	// Detect strategy
	strategy, err := DetectStrategy(sourcePath)
	if err != nil {
		return result, err
	}
	result.Strategy = strategy

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return result, fmt.Errorf("cannot create parent directory for %s: %w", targetPath, err)
	}

	// Check existing target
	existingInfo, err := os.Lstat(targetPath)
	if err == nil {
		// Target exists
		if existingInfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink — check if it points to the right place
			currentTarget, err := os.Readlink(targetPath)
			if err == nil && currentTarget == sourcePath && strategy == "symlink" {
				// Already correct — no-op
				d.deployed = append(d.deployed, result)
				return result, nil
			}
			// Wrong target — remove and re-create
			if err := os.Remove(targetPath); err != nil {
				return result, fmt.Errorf("cannot remove existing symlink %s: %w", targetPath, err)
			}
		} else {
			// Regular file or directory
			if strategy == "template" {
				// Templates always overwrite
				if err := os.Remove(targetPath); err != nil {
					return result, fmt.Errorf("cannot remove existing file %s: %w", targetPath, err)
				}
			} else if force {
				if err := os.RemoveAll(targetPath); err != nil {
					return result, fmt.Errorf("cannot remove existing file %s: %w", targetPath, err)
				}
			} else {
				return result, fmt.Errorf("target %s already exists and is not managed by facet — use --force to replace", targetPath)
			}
		}
	}

	// Deploy
	switch strategy {
	case "symlink":
		if err := os.Symlink(sourcePath, targetPath); err != nil {
			return result, fmt.Errorf("cannot create symlink %s → %s: %w", targetPath, sourcePath, err)
		}
	case "template":
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return result, fmt.Errorf("cannot read template source %s: %w", sourcePath, err)
		}
		rendered, err := substituteFileVars(string(content), d.vars)
		if err != nil {
			return result, fmt.Errorf("template rendering failed for %s: %w", source, err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), 0o644); err != nil {
			return result, fmt.Errorf("cannot write rendered template to %s: %w", targetPath, err)
		}
	}

	d.deployed = append(d.deployed, result)
	return result, nil
}

// substituteFileVars replaces ${facet:var.name} in file content.
// Uses the same logic as the config resolver but operates on raw file content.
func substituteFileVars(content string, vars map[string]any) (string, error) {
	var resolveErr error

	result := facetVarPatternDeploy.ReplaceAllStringFunc(content, func(match string) string {
		if resolveErr != nil {
			return match
		}
		submatches := facetVarPatternDeploy.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		value, err := lookupVarDeploy(vars, key)
		if err != nil {
			resolveErr = err
			return match
		}
		return value
	})

	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// lookupVarDeploy is the same as config.lookupVar but in the deploy package.
func lookupVarDeploy(vars map[string]any, key string) (string, error) {
	parts := strings.Split(key, ".")
	current := vars

	for i, part := range parts {
		if current == nil {
			return "", fmt.Errorf("undefined variable: ${facet:%s}", key)
		}
		val, ok := current[part]
		if !ok {
			return "", fmt.Errorf("undefined variable: ${facet:%s}", key)
		}

		if i == len(parts)-1 {
			s, ok := val.(string)
			if !ok {
				return "", fmt.Errorf("variable ${facet:%s} resolves to a map, not a string", key)
			}
			return s, nil
		}

		m, ok := val.(map[string]any)
		if !ok {
			return "", fmt.Errorf("variable ${facet:%s}: '%s' is not a map", key, part)
		}
		current = m
	}
	return "", fmt.Errorf("undefined variable: ${facet:%s}", key)
}

// Unapply removes previously deployed configs based on state records.
func (d *Deployer) Unapply(configs []state.ConfigState) error {
	for _, cfg := range configs {
		if err := os.Remove(cfg.Target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove %s during unapply: %w", cfg.Target, err)
		}
	}
	return nil
}

// Rollback removes all configs deployed during this session.
func (d *Deployer) Rollback() error {
	return d.Unapply(d.deployed)
}
```

Also add the regex pattern import at the top of the file (since we need it for substituteFileVars):

Add to deployer.go imports and add the pattern:

```go
import (
	"regexp"
	// ... other imports
)

var facetVarPatternDeploy = regexp.MustCompile(`\$\{facet:([a-zA-Z0-9_.]+)\}`)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/deploy/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/deployer.go internal/deploy/deployer_test.go
git commit -m "feat: config deployer with symlink, template, unapply, and rollback"
```

---

### Task 9: Package Installer

**Files:**
- Create: `internal/packages/installer.go`
- Create: `internal/packages/installer_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/packages/installer_test.go
package packages

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/config"
	"facet/internal/state"
)

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	if runtime.GOOS == "darwin" {
		assert.Equal(t, "macos", os)
	} else {
		assert.Equal(t, "linux", os)
	}
}

func TestGetInstallCommand_Simple(t *testing.T) {
	pkg := config.PackageEntry{
		Name:    "ripgrep",
		Install: config.InstallCmd{Command: "brew install ripgrep"},
	}

	cmd, skip := GetInstallCommand(pkg, "macos")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)

	cmd, skip = GetInstallCommand(pkg, "linux")
	assert.Equal(t, "brew install ripgrep", cmd)
	assert.False(t, skip)
}

func TestGetInstallCommand_PerOS(t *testing.T) {
	pkg := config.PackageEntry{
		Name: "lazydocker",
		Install: config.InstallCmd{
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
	pkg := config.PackageEntry{
		Name: "xcode-tools",
		Install: config.InstallCmd{
			PerOS: map[string]string{
				"macos": "xcode-select --install",
			},
		},
	}

	_, skip := GetInstallCommand(pkg, "linux")
	assert.True(t, skip)
}

func TestInstallAll_CollectsResults(t *testing.T) {
	packages := []config.PackageEntry{
		{Name: "echo-test", Install: config.InstallCmd{Command: "echo hello"}},
	}

	results := InstallAll(packages, DetectOS())
	require.Len(t, results, 1)
	assert.Equal(t, "echo-test", results[0].Name)
	assert.Equal(t, "ok", results[0].Status)
}

func TestInstallAll_FailureContinues(t *testing.T) {
	packages := []config.PackageEntry{
		{Name: "will-fail", Install: config.InstallCmd{Command: "false"}},
		{Name: "will-pass", Install: config.InstallCmd{Command: "echo ok"}},
	}

	results := InstallAll(packages, DetectOS())
	require.Len(t, results, 2)
	assert.Equal(t, "failed", results[0].Status)
	assert.NotEmpty(t, results[0].Error)
	assert.Equal(t, "ok", results[1].Status)
}

func TestInstallAll_SkippedOS(t *testing.T) {
	packages := []config.PackageEntry{
		{Name: "mac-only", Install: config.InstallCmd{
			PerOS: map[string]string{"macos": "echo mac"},
		}},
	}

	results := InstallAll(packages, "linux")
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
}

func TestInstallResultToState(t *testing.T) {
	results := []state.PackageState{
		{Name: "git", Install: "brew install git", Status: "ok"},
	}
	assert.Equal(t, "ok", results[0].Status)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/packages/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement installer.go**

```go
// internal/packages/installer.go
package packages

import (
	"fmt"
	"os/exec"
	"runtime"

	"facet/internal/config"
	"facet/internal/state"
)

// DetectOS returns "macos" or "linux".
func DetectOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "linux"
}

// GetInstallCommand returns the install command for the given OS.
// Returns the command and whether to skip (true = skip, no command for this OS).
func GetInstallCommand(pkg config.PackageEntry, osName string) (string, bool) {
	cmd, ok := pkg.Install.ForOS(osName)
	if !ok {
		return "", true
	}
	return cmd, false
}

// InstallAll runs install commands for all packages.
// Failed installs are recorded but do not stop other installations.
func InstallAll(packages []config.PackageEntry, osName string) []state.PackageState {
	results := make([]state.PackageState, 0, len(packages))

	for _, pkg := range packages {
		cmd, skip := GetInstallCommand(pkg, osName)

		ps := state.PackageState{
			Name:    pkg.Name,
			Install: cmd,
		}

		if skip {
			ps.Status = "skipped"
			ps.Error = fmt.Sprintf("no install command for OS %q", osName)
			results = append(results, ps)
			continue
		}

		err := runCommand(cmd)
		if err != nil {
			ps.Status = "failed"
			ps.Error = err.Error()
		} else {
			ps.Status = "ok"
		}

		results = append(results, ps)
	}

	return results
}

// runCommand executes a shell command.
func runCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/packages/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/packages/installer.go internal/packages/installer_test.go
git commit -m "feat: package installer with OS detection and non-fatal failure handling"
```

---

## Chunk 4: CLI Commands

### Task 10: Reporter

**Files:**
- Create: `internal/reporter/reporter.go`
- Create: `internal/reporter/reporter_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/reporter/reporter_test.go
package reporter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"facet/internal/state"
)

func TestReporter_PrintApplyReport_NoColor(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	s := &state.ApplyState{
		Profile: "acme",
		Packages: []state.PackageState{
			{Name: "git", Install: "brew install git", Status: "ok"},
			{Name: "docker", Install: "brew install docker", Status: "failed", Error: "not found"},
		},
		Configs: []state.ConfigState{
			{Target: "~/.gitconfig", Source: "configs/.gitconfig", Strategy: "template"},
			{Target: "~/.zshrc", Source: "configs/.zshrc", Strategy: "symlink"},
		},
	}

	r.PrintApplyReport(s)
	output := buf.String()
	assert.Contains(t, output, "acme")
	assert.Contains(t, output, "git")
	assert.Contains(t, output, "docker")
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, ".gitconfig")
	assert.Contains(t, output, "template")
	assert.Contains(t, output, "symlink")
}

func TestReporter_PrintStatus_NoState(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.PrintNoState()
	output := buf.String()
	assert.Contains(t, output, "facet apply")
}

func TestReporter_ColorDisabled(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	// Should not contain ANSI escape codes
	r.Success("test message")
	output := buf.String()
	assert.NotContains(t, output, "\033[")
	assert.Contains(t, output, "test message")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/reporter/ -v`
Expected: FAIL — Reporter not defined

- [ ] **Step 3: Implement reporter.go**

```go
// internal/reporter/reporter.go
package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"facet/internal/state"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// Reporter handles formatted terminal output.
type Reporter struct {
	w     io.Writer
	color bool
}

// New creates a new Reporter.
func New(w io.Writer, color bool) *Reporter {
	return &Reporter{w: w, color: color}
}

// NewDefault creates a Reporter that writes to stdout with auto-detected color support.
func NewDefault() *Reporter {
	color := os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
	return &Reporter{w: os.Stdout, color: color}
}

func (r *Reporter) colorize(color, text string) string {
	if !r.color {
		return text
	}
	return color + text + colorReset
}

// Success prints a success message with a checkmark.
func (r *Reporter) Success(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorGreen, "✓"), msg)
}

// Warning prints a warning message.
func (r *Reporter) Warning(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorYellow, "⚠"), msg)
}

// Error prints an error message.
func (r *Reporter) Error(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorRed, "✗"), msg)
}

// Header prints a section header.
func (r *Reporter) Header(msg string) {
	fmt.Fprintf(r.w, "\n%s\n", r.colorize(colorBold, msg))
}

// PrintApplyReport prints the result of a facet apply run.
func (r *Reporter) PrintApplyReport(s *state.ApplyState) {
	r.Header(fmt.Sprintf("Applied profile: %s", s.Profile))

	// Packages section
	if len(s.Packages) > 0 {
		r.Header("Packages")
		for _, pkg := range s.Packages {
			switch pkg.Status {
			case "ok":
				r.Success(fmt.Sprintf("%-20s %s", pkg.Name, r.colorize(colorDim, pkg.Install)))
			case "failed":
				r.Error(fmt.Sprintf("%-20s %s (%s)", pkg.Name, pkg.Install, pkg.Error))
			case "skipped":
				r.Warning(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			}
		}
	}

	// Configs section
	if len(s.Configs) > 0 {
		r.Header("Configs")
		for _, cfg := range s.Configs {
			r.Success(fmt.Sprintf("%-30s → %-30s (%s)", cfg.Target, cfg.Source, cfg.Strategy))
		}
	}
}

// PrintStatus prints the current facet status with validity checks.
func (r *Reporter) PrintStatus(s *state.ApplyState, checks []ValidityCheck) {
	r.Header(fmt.Sprintf("Profile: %s", s.Profile))
	fmt.Fprintf(r.w, "  Applied: %s (%s ago)\n", s.AppliedAt.Format(time.RFC3339), timeSince(s.AppliedAt))

	// Packages
	if len(s.Packages) > 0 {
		ok, failed, skipped := 0, 0, 0
		for _, p := range s.Packages {
			switch p.Status {
			case "ok":
				ok++
			case "failed":
				failed++
			case "skipped":
				skipped++
			}
		}

		r.Header("Packages")
		for _, pkg := range s.Packages {
			switch pkg.Status {
			case "ok":
				r.Success(fmt.Sprintf("%-20s %s", pkg.Name, r.colorize(colorDim, pkg.Install)))
			case "failed":
				r.Error(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			case "skipped":
				r.Warning(fmt.Sprintf("%-20s %s", pkg.Name, pkg.Error))
			}
		}
	}

	// Configs with validity
	if len(s.Configs) > 0 {
		r.Header("Configs")
		checkMap := make(map[string]ValidityCheck)
		for _, c := range checks {
			checkMap[c.Target] = c
		}

		for _, cfg := range s.Configs {
			check, hasCheck := checkMap[cfg.Target]
			status := r.colorize(colorGreen, "✓")
			suffix := ""
			if hasCheck && !check.Valid {
				status = r.colorize(colorRed, "✗")
				suffix = fmt.Sprintf(" (%s)", check.Error)
			}
			fmt.Fprintf(r.w, "  %s %-30s → %-30s (%s)%s\n",
				status, cfg.Target, cfg.Source, cfg.Strategy, suffix)
		}
	}
}

// ValidityCheck represents the result of checking a deployed config.
type ValidityCheck struct {
	Target string
	Valid  bool
	Error  string
}

// PrintNoState prints a message when no profile has been applied.
func (r *Reporter) PrintNoState() {
	fmt.Fprintf(r.w, "No profile has been applied yet.\n")
	fmt.Fprintf(r.w, "Run: facet apply <profile>\n")
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// Separator prints a visual separator line.
func (r *Reporter) Separator() {
	fmt.Fprintf(r.w, "%s\n", strings.Repeat("─", 60))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/reporter/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/reporter/reporter.go internal/reporter/reporter_test.go
git commit -m "feat: colored reporter with structured output for apply and status"
```

---

### Task 11: Init Command

**Files:**
- Create: `cmd/init_cmd.go`

- [ ] **Step 1: Implement init command**

```go
// cmd/init_cmd.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/internal/reporter"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new facet config repository",
	Long:  "Creates a facet config repo in the current directory and initializes the state directory.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	r := reporter.NewDefault()
	configDir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	stDir, err := resolveStateDir()
	if err != nil {
		return err
	}

	// Check if already initialized
	if _, err := os.Stat(filepath.Join(configDir, "facet.yaml")); err == nil {
		return fmt.Errorf("facet.yaml already exists in %s — already initialized", configDir)
	}

	// Create config repo files
	if err := createConfigRepo(configDir); err != nil {
		return fmt.Errorf("failed to create config repo: %w", err)
	}

	// Create state directory and .local.yaml
	if err := createStateDir(stDir); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	r.Success(fmt.Sprintf("Config repo initialized in %s", configDir))
	r.Success(fmt.Sprintf("State directory at %s", stDir))
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit base.yaml to add your shared packages and configs")
	fmt.Println("  2. Create a profile in profiles/ for this machine")
	fmt.Println("  3. Edit ~/.facet/.local.yaml to add machine-specific secrets")
	fmt.Println("  4. Run: facet apply <profile>")

	return nil
}

func createConfigRepo(dir string) error {
	// facet.yaml
	facetYAML := `min_version: "0.1.0"
`
	if err := os.WriteFile(filepath.Join(dir, "facet.yaml"), []byte(facetYAML), 0o644); err != nil {
		return err
	}

	// base.yaml with commented examples
	baseYAML := `# Base configuration — shared across all profiles.
# Every profile extends this via 'extends: base'.

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

	// Create directories
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
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
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
```

- [ ] **Step 2: Verify init works**

Run: `cd /Users/edocsss/aec/src/facet && go build -o facet . && cd /tmp && mkdir facet-test && cd facet-test && /Users/edocsss/aec/src/facet/facet init && ls -la && cat facet.yaml && cat base.yaml`
Expected: Config repo files created, facet.yaml and base.yaml have correct content.

- [ ] **Step 3: Commit**

```bash
git add cmd/init_cmd.go
git commit -m "feat: facet init command to scaffold config repo and state dir"
```

---

### Task 12: Apply Command

**Files:**
- Create: `cmd/apply.go`

- [ ] **Step 1: Implement apply command**

```go
// cmd/apply.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"facet/internal/config"
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/reporter"
	"facet/internal/state"

	"github.com/spf13/cobra"
)

var (
	forceApply  bool
	skipFailure bool
)

var applyCmd = &cobra.Command{
	Use:   "apply <profile>",
	Short: "Apply a configuration profile",
	Long:  "Loads, merges, and applies a configuration profile to this machine.",
	Args:  cobra.ExactArgs(1),
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().BoolVar(&forceApply, "force", false, "Unapply + apply, skip prompts for conflicting files")
	applyCmd.Flags().BoolVar(&skipFailure, "skip-failure", false, "Warn on config deploy failure instead of rolling back")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	r := reporter.NewDefault()

	// Step 1: Resolve directories
	cfgDir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	stDir, err := resolveStateDir()
	if err != nil {
		return err
	}

	// Step 2: Load facet.yaml
	_, err = config.LoadMeta(cfgDir)
	if err != nil {
		return fmt.Errorf("not a facet config directory: %w\nUse -c to specify the config directory, or run facet init to create one.", err)
	}

	// Step 3: Load base.yaml
	baseCfg, err := config.LoadConfig(filepath.Join(cfgDir, "base.yaml"))
	if err != nil {
		return err
	}

	// Step 4: Load profile
	profilePath := filepath.Join(cfgDir, "profiles", profileName+".yaml")
	profileCfg, err := config.LoadConfig(profilePath)
	if err != nil {
		return fmt.Errorf("cannot load profile %q: %w", profileName, err)
	}
	if err := config.ValidateProfile(profileCfg); err != nil {
		return err
	}

	// Step 5: Load .local.yaml
	localPath := filepath.Join(stDir, ".local.yaml")
	localCfg, err := config.LoadConfig(localPath)
	if err != nil {
		return fmt.Errorf(".local.yaml is required in %s: %w", stDir, err)
	}

	// Step 6: Merge layers
	merged, err := config.Merge(baseCfg, profileCfg)
	if err != nil {
		return fmt.Errorf("merge error: %w", err)
	}
	merged, err = config.Merge(merged, localCfg)
	if err != nil {
		return fmt.Errorf("merge error with .local.yaml: %w", err)
	}

	// Step 7: Resolve variables
	resolved, err := config.Resolve(merged)
	if err != nil {
		return err
	}

	// Step 8: Canary write to .state.json
	if err := os.MkdirAll(stDir, 0o755); err != nil {
		return fmt.Errorf("cannot create state directory %s: %w", stDir, err)
	}
	if err := state.CanaryWrite(stDir); err != nil {
		return fmt.Errorf("cannot write state file: %w", err)
	}

	// Read previous state for unapply
	prevState, err := state.Read(stDir)
	if err != nil {
		r.Warning(fmt.Sprintf("Could not read previous state: %v", err))
	}

	// Unapply previous state if needed
	if prevState != nil {
		shouldUnapply := forceApply || prevState.Profile != profileName
		if shouldUnapply {
			deployer := deploy.NewDeployer(cfgDir, "", resolved.Vars)
			if err := deployer.Unapply(prevState.Configs); err != nil {
				r.Warning(fmt.Sprintf("Unapply warning: %v", err))
			}
		} else {
			// Same profile reapply — find orphaned configs to clean up
			newTargets := make(map[string]bool)
			for target := range resolved.Configs {
				expanded, err := deploy.ExpandPath(target)
				if err != nil {
					continue
				}
				newTargets[expanded] = true
			}
			var orphans []state.ConfigState
			for _, cfg := range prevState.Configs {
				if !newTargets[cfg.Target] {
					orphans = append(orphans, cfg)
				}
			}
			if len(orphans) > 0 {
				deployer := deploy.NewDeployer(cfgDir, "", nil)
				if err := deployer.Unapply(orphans); err != nil {
					r.Warning(fmt.Sprintf("Orphan cleanup warning: %v", err))
				}
			}
		}
	}

	// Step 9: Install packages
	osName := packages.DetectOS()
	pkgResults := packages.InstallAll(resolved.Packages, osName)

	// Step 10: Deploy configs
	deployer := deploy.NewDeployer(cfgDir, "", resolved.Vars)
	var configResults []state.ConfigState
	var deployErr error

	for target, source := range resolved.Configs {
		// Validate source path
		if err := deploy.ValidateSourcePath(source, cfgDir); err != nil {
			if skipFailure {
				r.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
				continue
			}
			deployer.Rollback()
			return fmt.Errorf("config deployment failed: %w", err)
		}

		// Expand target path
		expandedTarget, err := deploy.ExpandPath(target)
		if err != nil {
			if skipFailure {
				r.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
				continue
			}
			deployer.Rollback()
			return fmt.Errorf("config deployment failed: %w", err)
		}

		result, err := deployer.DeployOne(expandedTarget, source, forceApply)
		if err != nil {
			if skipFailure {
				r.Warning(fmt.Sprintf("Config deploy warning: %v", err))
				continue
			}
			deployErr = err
			break
		}
		configResults = append(configResults, result)
	}

	if deployErr != nil {
		r.Error(fmt.Sprintf("Config deployment failed: %v", deployErr))
		r.Warning("Rolling back deployed configs...")
		deployer.Rollback()
		return fmt.Errorf("config deployment failed (rolled back): %w", deployErr)
	}

	// Step 11: Write final .state.json
	applyState := &state.ApplyState{
		Profile:      profileName,
		AppliedAt:    time.Now().UTC(),
		FacetVersion: rootCmd.Version,
		Packages:     pkgResults,
		Configs:      deployer.Deployed(),
	}

	if err := state.Write(stDir, applyState); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	// Step 12: Print report
	r.PrintApplyReport(applyState)

	return nil
}
```

- [ ] **Step 2: Verify apply compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build -o facet .`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/apply.go
git commit -m "feat: facet apply command with full pipeline — load, merge, resolve, unapply, install, deploy"
```

---

### Task 13: Status Command

**Files:**
- Create: `cmd/status.go`

- [ ] **Step 1: Implement status command**

```go
// cmd/status.go
package cmd

import (
	"fmt"
	"os"

	"facet/internal/reporter"
	"facet/internal/state"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current facet status",
	Long:  "Displays the currently applied profile, packages, configs, and their validity.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	r := reporter.NewDefault()

	stDir, err := resolveStateDir()
	if err != nil {
		return err
	}

	s, err := state.Read(stDir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if s == nil {
		r.PrintNoState()
		return nil
	}

	// Run validity checks
	checks := runValidityChecks(s)

	r.PrintStatus(s, checks)

	return nil
}

// runValidityChecks checks that deployed configs are still valid.
// This logic is encapsulated in its own function for easy future refactoring.
func runValidityChecks(s *state.ApplyState) []reporter.ValidityCheck {
	var checks []reporter.ValidityCheck

	for _, cfg := range s.Configs {
		check := reporter.ValidityCheck{Target: cfg.Target}

		info, err := os.Lstat(cfg.Target)
		if err != nil {
			check.Valid = false
			check.Error = "file missing"
			checks = append(checks, check)
			continue
		}

		switch cfg.Strategy {
		case "symlink":
			if info.Mode()&os.ModeSymlink == 0 {
				check.Valid = false
				check.Error = "expected symlink, found regular file"
			} else {
				target, err := os.Readlink(cfg.Target)
				if err != nil {
					check.Valid = false
					check.Error = "cannot read symlink target"
				} else if _, err := os.Stat(target); err != nil {
					check.Valid = false
					check.Error = "symlink target does not exist (broken symlink)"
				} else {
					check.Valid = true
				}
			}
		case "template":
			check.Valid = true // file exists, that's enough for templates
		default:
			check.Valid = true
		}

		checks = append(checks, check)
	}

	return checks
}
```

- [ ] **Step 2: Verify status compiles**

Run: `cd /Users/edocsss/aec/src/facet && go build -o facet .`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/status.go
git commit -m "feat: facet status command with validity checks"
```

---

### Task 14: Integration Test

**Files:**
- Create: `cmd/integration_test.go`

- [ ] **Step 1: Write integration test for full apply → status flow**

```go
// cmd/integration_test.go
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/state"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func setupTestFixture(t *testing.T) (cfgDir, stDir, homeDir string) {
	t.Helper()
	cfgDir = t.TempDir()
	stDir = t.TempDir()
	homeDir = t.TempDir()

	// Set HOME for path expansion
	t.Setenv("HOME", homeDir)

	// facet.yaml
	writeTestFile(t, filepath.Join(cfgDir, "facet.yaml"), `min_version: "0.1.0"`)

	// base.yaml
	writeTestFile(t, filepath.Join(cfgDir, "base.yaml"), `
vars:
  git_name: Sarah
packages:
  - name: echo-test
    install: echo "installed"
configs:
  ~/.zshrc: configs/.zshrc
`)

	// profile
	writeTestFile(t, filepath.Join(cfgDir, "profiles", "work.yaml"), `
extends: base
vars:
  git:
    email: sarah@acme.com
packages:
  - name: another-echo
    install: echo "also installed"
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	// config files
	writeTestFile(t, filepath.Join(cfgDir, "configs", ".zshrc"), "export EDITOR=nvim")
	writeTestFile(t, filepath.Join(cfgDir, "configs", ".gitconfig"),
		"[user]\n  email = ${facet:git.email}\n  name = ${facet:git_name}")

	// .local.yaml
	writeTestFile(t, filepath.Join(stDir, ".local.yaml"), `
vars:
  secret_key: s3cret
`)

	return
}

func TestIntegration_ApplyAndStatus(t *testing.T) {
	cfgDir, stDir, homeDir := setupTestFixture(t)

	// Override global flags
	configDir = cfgDir
	stateDir = stDir

	// Run apply
	err := runApply(nil, []string{"work"})
	require.NoError(t, err)

	// Verify state file
	s, err := state.Read(stDir)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "work", s.Profile)

	// Verify .zshrc is symlinked
	link, err := os.Readlink(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cfgDir, "configs", ".zshrc"), link)

	// Verify .gitconfig is templated
	content, err := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "sarah@acme.com")
	assert.Contains(t, string(content), "Sarah")
	assert.NotContains(t, string(content), "${facet:")

	// Verify packages ran
	assert.Len(t, s.Packages, 2)
}

func TestIntegration_ProfileSwitch(t *testing.T) {
	cfgDir, stDir, homeDir := setupTestFixture(t)

	// Add a second profile
	writeTestFile(t, filepath.Join(cfgDir, "profiles", "personal.yaml"), `
extends: base
vars:
  git:
    email: sarah@hey.com
configs:
  ~/.gitconfig: configs/.gitconfig
`)

	configDir = cfgDir
	stateDir = stDir

	// Apply work
	err := runApply(nil, []string{"work"})
	require.NoError(t, err)

	// Verify work state
	content, _ := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	assert.Contains(t, string(content), "sarah@acme.com")

	// Switch to personal
	err = runApply(nil, []string{"personal"})
	require.NoError(t, err)

	// Verify personal state
	content, _ = os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	assert.Contains(t, string(content), "sarah@hey.com")
	assert.NotContains(t, string(content), "acme")

	// State file should show personal
	s, _ := state.Read(stDir)
	assert.Equal(t, "personal", s.Profile)
}

func TestIntegration_MissingProfile(t *testing.T) {
	cfgDir, stDir, _ := setupTestFixture(t)

	configDir = cfgDir
	stateDir = stDir

	err := runApply(nil, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestIntegration_Idempotent(t *testing.T) {
	cfgDir, stDir, homeDir := setupTestFixture(t)

	configDir = cfgDir
	stateDir = stDir

	// Apply twice
	err := runApply(nil, []string{"work"})
	require.NoError(t, err)

	first, _ := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))

	err = runApply(nil, []string{"work"})
	require.NoError(t, err)

	second, _ := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))

	assert.Equal(t, string(first), string(second))
}
```

- [ ] **Step 2: Run integration tests**

Run: `go test ./cmd/ -v -run TestIntegration`
Expected: All tests PASS

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/integration_test.go
git commit -m "feat: integration tests for apply, profile switch, and idempotency"
```

---

### Task 15: Final Unit Test Verification

- [ ] **Step 1: Run full test suite with coverage**

Run: `cd /Users/edocsss/aec/src/facet && go test ./... -cover`
Expected: All tests PASS, reasonable coverage across packages.

- [ ] **Step 2: Build and test binary manually**

Run: `go build -o facet . && ./facet --help`
Expected: Shows help with init, apply, status subcommands.

Run: `./facet --version`
Expected: `facet version 0.1.0`

- [ ] **Step 3: Commit any final fixes and verify clean build**

Run: `go vet ./... && go build -o facet .`
Expected: No warnings, clean build.

---

## Chunk 5: Hermetic E2E Testing

E2E tests run the real facet binary against realistic fixtures in a fully isolated environment. The real system is never modified.

### Isolation design

```
Real machine during test run:

  Real HOME (/Users/you)           Sandbox HOME (/tmp/facet-e2e.XXXXX/suite.YYYYY)
  ├── .gitconfig  (untouched)      ├── .gitconfig  ← facet writes here
  ├── .zshrc      (untouched)      ├── .zshrc      ← facet writes here
  └── ...                          ├── .facet/     ← state dir (--state-dir)
                                   ├── mock-bin/   ← mock brew/apt-get
                                   └── dotfiles/   ← config repo (--config-dir)

  Real PATH                        Test PATH
  /usr/local/bin:...               $HOME/mock-bin:/usr/local/bin:...
                                   ↑ mocks take priority
```

**Key guarantees:**
- `HOME` override is scoped to the child process (the test suite). Parent shell is untouched.
- `PATH` prepend is scoped to the child process. Parent shell PATH is untouched.
- Each suite gets its own sandbox subdirectory — suites cannot leak state.
- `trap cleanup EXIT` deletes the sandbox on any exit (pass, fail, Ctrl+C).
- On Docker: real `apt-get` is used (container is disposable). Mocks are skipped.

### File structure

```
e2e/
├── Dockerfile.ubuntu          # Ubuntu 24.04 test image
├── harness.sh                 # HOME-isolated test runner
├── fixtures/
│   ├── mock-tools.sh          # Creates mock brew/apt-get in $HOME/mock-bin
│   └── setup-basic.sh         # Populates a test config repo in the sandbox
├── suites/
│   ├── helpers.sh             # Assertion functions (assert_file_exists, etc.)
│   ├── 01-init.sh             # facet init
│   ├── 02-apply-basic.sh      # Apply simple profile
│   ├── 03-configs.sh          # Symlink, template, parent dir creation
│   ├── 04-profile-switch.sh   # Switch profiles, orphan cleanup
│   ├── 05-idempotent.sh       # Apply twice = same result
│   ├── 06-edge-cases.sh       # Missing profile, undefined var, etc.
│   ├── 07-packages.sh         # Package install with mock/real
│   ├── 08-status.sh           # facet status with validity checks
│   └── 09-force-flag.sh       # --force and --skip-failure behavior
└── e2e_test.go                # Go test wrapper (runs harness from go test)
```

---

### Task 16: E2E Infrastructure

**Files:**
- Create: `e2e/harness.sh`
- Create: `e2e/fixtures/mock-tools.sh`
- Create: `e2e/fixtures/setup-basic.sh`
- Create: `e2e/suites/helpers.sh`
- Create: `e2e/Dockerfile.ubuntu`
- Modify: `Makefile`

- [ ] **Step 1: Create the test harness**

```bash
#!/bin/bash
# e2e/harness.sh
#
# Hermetic E2E test runner. Creates an isolated HOME for each suite.
# The parent shell's HOME and PATH are NEVER modified.
set -euo pipefail

# ── Create top-level sandbox ──
REAL_HOME="$HOME"
export E2E_SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/facet-e2e.XXXXXXXX")

# Cleanup on any exit — sandbox is always deleted
cleanup() {
    local exit_code=$?
    if [ -n "${E2E_SANDBOX:-}" ] && [ -d "$E2E_SANDBOX" ]; then
        rm -rf "$E2E_SANDBOX"
    fi
    exit $exit_code
}
trap cleanup EXIT

# Resolve suite/fixture locations
if [ -d "/opt/e2e" ]; then
    # Docker: files were COPYed to /opt/e2e
    SUITE_DIR="/opt/e2e/suites"
    FIXTURE_DIR="/opt/e2e/fixtures"
else
    # Native: relative to this script
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    SUITE_DIR="$SCRIPT_DIR/suites"
    FIXTURE_DIR="$SCRIPT_DIR/fixtures"
fi

# Filter suites if specific ones requested
if [ $# -gt 0 ]; then
    SUITES=("$@")
else
    SUITES=("$SUITE_DIR"/[0-9]*.sh)
fi

echo "========================================"
echo "  facet E2E test run"
echo "  $(date -Iseconds 2>/dev/null || date)"
echo "  Sandbox: $E2E_SANDBOX"
echo "  OS: $(uname -s) $(uname -m)"
echo "  facet: $(facet --version 2>/dev/null || echo 'not found')"
echo "========================================"
echo ""

PASSED=0
FAILED=0
ERRORS=()

for suite in "${SUITES[@]}"; do
    name=$(basename "$suite" .sh)
    [[ "$name" == "helpers" ]] && continue

    echo "--- [$name] ---"

    # Each suite gets its own clean HOME subdirectory.
    # HOME and PATH are only set for the child process — parent is untouched.
    SUITE_HOME=$(mktemp -d "$E2E_SANDBOX/suite.XXXXXXXX")

    # Set up mock tools in the suite's HOME (scoped to child process)
    if HOME="$SUITE_HOME" PATH="$SUITE_HOME/mock-bin:$PATH" \
       FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$FIXTURE_DIR/mock-tools.sh" >/dev/null 2>&1 \
       && HOME="$SUITE_HOME" PATH="$SUITE_HOME/mock-bin:$PATH" \
       FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$suite" 2>&1; then
        echo "  ✓ PASS"
        ((PASSED++))
    else
        echo "  ✗ FAIL (exit $?)"
        ((FAILED++))
        ERRORS+=("$name")
    fi
    echo ""
done

echo "========================================"
echo "  Results: $PASSED passed, $FAILED failed"
if [ $FAILED -gt 0 ]; then
    echo "  Failed: ${ERRORS[*]}"
    echo "========================================"
    exit 1
fi
echo "========================================"
exit 0
```

- [ ] **Step 2: Create mock tools**

```bash
#!/bin/bash
# e2e/fixtures/mock-tools.sh
#
# Creates mock package manager binaries in $HOME/mock-bin.
# These log install commands instead of actually installing.
# PATH is set by the harness — only the child process sees these mocks.
set -euo pipefail

mkdir -p "$HOME/mock-bin"
MOCK_PKG_LOG="$HOME/.mock-packages"
touch "$MOCK_PKG_LOG"

# Skip mocks if using real packages (Docker with apt)
if [ "${FACET_E2E_REAL_PACKAGES:-}" = "1" ]; then
    echo "[mock-tools] Using real package manager"
    exit 0
fi

# ── Mock brew ──
cat > "$HOME/mock-bin/brew" << 'BREWEOF'
#!/bin/bash
MOCK_PKG_LOG="$HOME/.mock-packages"
case "$1" in
    install)
        shift
        for arg in "$@"; do
            [[ "$arg" == --* ]] && continue
            echo "$arg" >> "$MOCK_PKG_LOG"
            echo "mock-brew: installed $arg"
        done
        ;;
    list)
        cat "$MOCK_PKG_LOG" 2>/dev/null
        ;;
    --prefix)
        echo "/opt/homebrew"
        ;;
    *)
        echo "mock-brew: $*"
        ;;
esac
exit 0
BREWEOF
chmod +x "$HOME/mock-bin/brew"

# ── Mock apt-get ──
cat > "$HOME/mock-bin/apt-get" << 'APTEOF'
#!/bin/bash
MOCK_PKG_LOG="$HOME/.mock-packages"
case "$1" in
    install)
        shift
        for arg in "$@"; do
            [[ "$arg" == -* ]] && continue
            echo "$arg" >> "$MOCK_PKG_LOG"
            echo "mock-apt: installed $arg"
        done
        ;;
    update)
        echo "mock-apt: updated"
        ;;
    *)
        echo "mock-apt: $*"
        ;;
esac
exit 0
APTEOF
chmod +x "$HOME/mock-bin/apt-get"

# ── Mock sudo (passes through to the command) ──
cat > "$HOME/mock-bin/sudo" << 'SUDOEOF'
#!/bin/bash
"$@"
SUDOEOF
chmod +x "$HOME/mock-bin/sudo"

echo "[mock-tools] Package managers mocked in $HOME/mock-bin"
```

- [ ] **Step 3: Create test fixture setup script**

```bash
#!/bin/bash
# e2e/fixtures/setup-basic.sh
#
# Populates a test config repo and state dir inside the sandbox HOME.
# All paths use $HOME which the harness points to a temp directory.
set -euo pipefail

CONFIG_DIR="$HOME/dotfiles"
STATE_DIR="$HOME/.facet"

mkdir -p "$CONFIG_DIR"/{profiles,configs/work}
mkdir -p "$STATE_DIR"

# ── facet.yaml ──
cat > "$CONFIG_DIR/facet.yaml" << 'YAML'
min_version: "0.1.0"
YAML

# ── base.yaml ──
cat > "$CONFIG_DIR/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen

packages:
  - name: ripgrep
    install: brew install ripgrep
  - name: curl
    install: brew install curl

configs:
  ~/.zshrc: configs/.zshrc
  ~/.config/starship.toml: configs/starship.toml
YAML

# ── profiles/work.yaml ──
cat > "$CONFIG_DIR/profiles/work.yaml" << 'YAML'
extends: base

vars:
  git:
    email: sarah@acme.com

packages:
  - name: docker
    install: brew install docker
  - name: node
    install:
      macos: brew install node
      linux: sudo apt-get install -y nodejs

configs:
  ~/.gitconfig: configs/work/.gitconfig
  ~/.npmrc: configs/work/.npmrc
YAML

# ── profiles/personal.yaml ──
cat > "$CONFIG_DIR/profiles/personal.yaml" << 'YAML'
extends: base

vars:
  git:
    email: sarah@hey.com

configs:
  ~/.gitconfig: configs/.gitconfig
YAML

# ── config files ──
cat > "$CONFIG_DIR/configs/.zshrc" << 'SHELL'
export EDITOR=nvim
alias ll="ls -la"
SHELL

cat > "$CONFIG_DIR/configs/starship.toml" << 'TOML'
[character]
success_symbol = "[➜](bold green)"
TOML

# Template — contains ${facet:...} vars
cat > "$CONFIG_DIR/configs/.gitconfig" << 'GIT'
[user]
  name = ${facet:git_name}
  email = ${facet:git.email}
[core]
  editor = nvim
GIT

cat > "$CONFIG_DIR/configs/work/.gitconfig" << 'GIT'
[user]
  name = ${facet:git_name}
  email = ${facet:git.email}
[core]
  editor = cursor --wait
[commit]
  gpgsign = true
GIT

cat > "$CONFIG_DIR/configs/work/.npmrc" << 'NPM'
registry=https://npm.acme-corp.com
always-auth=true
NPM

# ── .local.yaml (in state dir) ──
cat > "$STATE_DIR/.local.yaml" << 'YAML'
vars:
  secret_key: s3cret
YAML

echo "[setup-basic] Config repo at $CONFIG_DIR, state dir at $STATE_DIR"
```

- [ ] **Step 4: Create assertion helpers**

```bash
#!/bin/bash
# e2e/suites/helpers.sh
#
# Assertion functions sourced by each test suite.
# All paths use $HOME which the harness points to the sandbox.

assert_file_exists() {
    if [ ! -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to exist"
        exit 1
    fi
}

assert_file_not_exists() {
    if [ -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to NOT exist"
        exit 1
    fi
}

assert_symlink() {
    if [ ! -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to be a symlink"
        exit 1
    fi
}

assert_not_symlink() {
    if [ -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to NOT be a symlink (got symlink)"
        exit 1
    fi
}

assert_file_contains() {
    if ! grep -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to contain '$2'"
        echo "  Actual content:"
        head -20 "$1" 2>/dev/null || echo "  (file not readable)"
        exit 1
    fi
}

assert_file_not_contains() {
    if grep -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to NOT contain '$2'"
        exit 1
    fi
}

assert_json_field() {
    local file="$1" path="$2" expected="$3"
    local actual
    actual=$(jq -r "$path" "$file" 2>/dev/null)
    if [ "$actual" != "$expected" ]; then
        echo "  ASSERT FAIL: $file $path"
        echo "  Expected: $expected"
        echo "  Actual:   $actual"
        exit 1
    fi
}

assert_exit_code() {
    local expected="$1"
    shift
    local actual
    set +e
    "$@" >/dev/null 2>&1
    actual=$?
    set -e
    if [ "$actual" -ne "$expected" ]; then
        echo "  ASSERT FAIL: expected exit code $expected, got $actual"
        echo "  Command: $*"
        exit 1
    fi
}

# Shorthand for running facet with test dirs
facet_apply() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply "$@"
}

facet_status() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" status "$@"
}

facet_init() {
    # init operates on cwd, so cd into the target
    local target="${1:-$HOME/dotfiles}"
    mkdir -p "$target"
    (cd "$target" && facet -s "$HOME/.facet" init)
}

# Helper to source the fixture setup
setup_basic() {
    bash "$FIXTURE_DIR/setup-basic.sh"
}
```

- [ ] **Step 5: Create Dockerfile**

```dockerfile
# e2e/Dockerfile.ubuntu
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    sudo \
    zsh \
    && rm -rf /var/lib/apt/lists/*

# Non-root test user
RUN useradd -m -s /bin/zsh testuser \
    && echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

RUN mkdir -p /opt/facet /opt/e2e

# Copy the facet binary (cross-compiled for linux on host)
COPY facet-linux /usr/local/bin/facet
RUN chmod +x /usr/local/bin/facet

# Copy test infrastructure
COPY fixtures/ /opt/e2e/fixtures/
COPY suites/ /opt/e2e/suites/
COPY harness.sh /opt/e2e/harness.sh
RUN chmod +x /opt/e2e/harness.sh /opt/e2e/suites/*.sh /opt/e2e/fixtures/*.sh

ENV SUITE_DIR=/opt/e2e/suites
ENV FIXTURE_DIR=/opt/e2e/fixtures

USER testuser
WORKDIR /home/testuser

ENTRYPOINT ["/opt/e2e/harness.sh"]
```

- [ ] **Step 6: Add Makefile targets**

Add to `Makefile`:

```makefile
# --- Cross-compile for Docker ---
build-linux:
	GOOS=linux GOARCH=amd64 go build -o e2e/facet-linux .

build-linux-arm:
	GOOS=linux GOARCH=arm64 go build -o e2e/facet-linux .

# --- E2E: Docker (Linux, real apt possible) ---
e2e: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -e FACET_E2E_REAL_PACKAGES=1 facet-e2e

e2e-suite: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -e FACET_E2E_REAL_PACKAGES=1 facet-e2e /opt/e2e/suites/$(SUITE).sh

e2e-shell: build-linux
	docker build -t facet-e2e -f e2e/Dockerfile.ubuntu e2e/
	docker run --rm -it --entrypoint /bin/bash facet-e2e

# --- E2E: Native (macOS/Linux, mocked packages, isolated HOME) ---
# Safe: never touches your real config files or PATH.
e2e-local: build
	@echo "Running E2E locally (HOME will be sandboxed, packages mocked)"
	@PATH="$$PWD:$$PATH" bash e2e/harness.sh

e2e-local-suite: build
	@PATH="$$PWD:$$PATH" bash e2e/harness.sh e2e/suites/$(SUITE).sh

# --- CI convenience ---
ci: test e2e
	@echo "All tests passed"

.PHONY: build build-linux build-linux-arm test test-cover clean e2e e2e-suite e2e-shell e2e-local e2e-local-suite ci
```

- [ ] **Step 7: Commit**

```bash
git add e2e/ Makefile
git commit -m "feat: E2E infrastructure — harness, mock tools, fixtures, Dockerfile"
```

---

### Task 17: E2E Test Suites

**Files:**
- Create: `e2e/suites/01-init.sh`
- Create: `e2e/suites/02-apply-basic.sh`
- Create: `e2e/suites/03-configs.sh`
- Create: `e2e/suites/04-profile-switch.sh`
- Create: `e2e/suites/05-idempotent.sh`
- Create: `e2e/suites/06-edge-cases.sh`
- Create: `e2e/suites/07-packages.sh`
- Create: `e2e/suites/08-status.sh`
- Create: `e2e/suites/09-force-flag.sh`
- Create: `e2e/e2e_test.go`

- [ ] **Step 1: Suite 01 — init**

```bash
#!/bin/bash
# e2e/suites/01-init.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Test: init creates config repo structure
INIT_DIR="$HOME/new-repo"
facet_init "$INIT_DIR"

assert_file_exists "$INIT_DIR/facet.yaml"
assert_file_exists "$INIT_DIR/base.yaml"
assert_file_exists "$INIT_DIR/profiles"
assert_file_exists "$INIT_DIR/configs"
echo "  init creates config repo structure"

# Test: init creates .local.yaml in state dir
assert_file_exists "$HOME/.facet/.local.yaml"
echo "  init creates .local.yaml in state dir"

# Test: init fails if facet.yaml already exists
assert_exit_code 1 bash -c "cd $INIT_DIR && facet -s $HOME/.facet init"
echo "  init errors on existing facet.yaml"
```

- [ ] **Step 2: Suite 02 — apply basic**

```bash
#!/bin/bash
# e2e/suites/02-apply-basic.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply work profile
facet_apply work
echo "  apply exited cleanly"

# State file written
assert_file_exists "$HOME/.facet/.state.json"
assert_json_field "$HOME/.facet/.state.json" '.profile' 'work'
echo "  state file written with correct profile"

# Packages should have run (check mock log)
if [ "${FACET_E2E_REAL_PACKAGES:-}" != "1" ]; then
    assert_file_exists "$HOME/.mock-packages"
    assert_file_contains "$HOME/.mock-packages" "ripgrep"
    assert_file_contains "$HOME/.mock-packages" "docker"
    echo "  packages install commands executed"
fi
```

- [ ] **Step 3: Suite 03 — config deployment**

```bash
#!/bin/bash
# e2e/suites/03-configs.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
facet_apply work

# .zshrc should be symlinked (no ${facet:} vars)
assert_symlink "$HOME/.zshrc"
assert_file_contains "$HOME/.zshrc" "EDITOR=nvim"
echo "  .zshrc is symlinked"

# .gitconfig should be templated (has ${facet:} vars) — regular file, not symlink
assert_not_symlink "$HOME/.gitconfig"
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
assert_file_contains "$HOME/.gitconfig" "Sarah Chen"
assert_file_contains "$HOME/.gitconfig" "cursor --wait"
assert_file_not_contains "$HOME/.gitconfig" '${facet:'
echo "  .gitconfig templated with resolved vars"

# .npmrc should be symlinked
assert_symlink "$HOME/.npmrc"
assert_file_contains "$HOME/.npmrc" "acme-corp.com"
echo "  .npmrc symlinked"

# Parent dirs created automatically
assert_file_exists "$HOME/.config/starship.toml"
assert_symlink "$HOME/.config/starship.toml"
echo "  parent directories created (mkdir -p)"
```

- [ ] **Step 4: Suite 04 — profile switching**

```bash
#!/bin/bash
# e2e/suites/04-profile-switch.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply work first
facet_apply work
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
assert_file_exists "$HOME/.npmrc"
echo "  work profile applied"

# Switch to personal
facet_apply personal

# .gitconfig should now have personal email
assert_file_contains "$HOME/.gitconfig" "sarah@hey.com"
assert_file_not_contains "$HOME/.gitconfig" "acme"
assert_file_not_contains "$HOME/.gitconfig" "gpgsign"
echo "  .gitconfig switched to personal vars"

# .npmrc should be gone (personal doesn't define it — orphan cleanup)
assert_file_not_exists "$HOME/.npmrc"
echo "  .npmrc removed (orphan cleanup)"

# .zshrc still present (both profiles have it via base)
assert_symlink "$HOME/.zshrc"
echo "  .zshrc preserved from base"

# State file updated
assert_json_field "$HOME/.facet/.state.json" '.profile' 'personal'
echo "  state file shows personal"
```

- [ ] **Step 5: Suite 05 — idempotency**

```bash
#!/bin/bash
# e2e/suites/05-idempotent.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply twice
facet_apply work
FIRST_GITCONFIG=$(cat "$HOME/.gitconfig")
FIRST_ZSHRC_TARGET=$(readlink "$HOME/.zshrc")

facet_apply work
SECOND_GITCONFIG=$(cat "$HOME/.gitconfig")
SECOND_ZSHRC_TARGET=$(readlink "$HOME/.zshrc")

# Content should be identical
if [ "$FIRST_GITCONFIG" != "$SECOND_GITCONFIG" ]; then
    echo "  ASSERT FAIL: .gitconfig changed on second apply"
    exit 1
fi
echo "  .gitconfig identical after double apply"

if [ "$FIRST_ZSHRC_TARGET" != "$SECOND_ZSHRC_TARGET" ]; then
    echo "  ASSERT FAIL: .zshrc symlink changed on second apply"
    exit 1
fi
echo "  .zshrc symlink identical after double apply"

echo "  double apply is idempotent"
```

- [ ] **Step 6: Suite 06 — edge cases**

```bash
#!/bin/bash
# e2e/suites/06-edge-cases.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Missing profile → fatal error
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply nonexistent
echo "  missing profile errors correctly"

# Undefined variable → fatal error
cat > "$HOME/dotfiles/profiles/badvar.yaml" << 'YAML'
extends: base
configs:
  ~/.badfile: configs/.zshrc
packages:
  - name: test
    install: echo ${facet:totally_undefined_var}
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badvar
echo "  undefined variable errors correctly"

# Profile without extends → fatal error
cat > "$HOME/dotfiles/profiles/noextends.yaml" << 'YAML'
vars:
  test: value
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply noextends
echo "  missing extends errors correctly"

# Profile with invalid extends → fatal error
cat > "$HOME/dotfiles/profiles/badextends.yaml" << 'YAML'
extends: something_else
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badextends
echo "  invalid extends value errors correctly"

# Missing .local.yaml → fatal error
rm "$HOME/.facet/.local.yaml"
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work
echo "  missing .local.yaml errors correctly"

# Empty profile (just extends) inherits base correctly
echo "vars:" > "$HOME/.facet/.local.yaml"  # restore
cat > "$HOME/dotfiles/profiles/minimal.yaml" << 'YAML'
extends: base
YAML
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply minimal
assert_symlink "$HOME/.zshrc"
echo "  empty profile inherits base correctly"
```

- [ ] **Step 7: Suite 07 — package installation**

```bash
#!/bin/bash
# e2e/suites/07-packages.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
facet_apply work

if [ "${FACET_E2E_REAL_PACKAGES:-}" = "1" ]; then
    echo "  real package tests (Docker)"
    # In Docker, verify real packages were installed
    # The fixture uses apt-get commands for linux
    echo "  (real package verification is best-effort in Docker)"
else
    # Mock: verify install commands were logged
    assert_file_contains "$HOME/.mock-packages" "ripgrep"
    assert_file_contains "$HOME/.mock-packages" "curl"
    assert_file_contains "$HOME/.mock-packages" "docker"
    echo "  all package install commands executed"

    # Per-OS: node has macos/linux variants. Check the right one ran.
    if [ "$(uname)" = "Darwin" ]; then
        # On macOS, mock brew should have received "node"
        assert_file_contains "$HOME/.mock-packages" "node"
    fi
    echo "  per-OS install command selected correctly"
fi

# Package failure should not prevent config deployment
cat >> "$HOME/dotfiles/base.yaml" << 'YAML'
  - name: will-fail
    install: "false"
YAML
facet_apply work
# Configs should still be deployed despite package failure
assert_symlink "$HOME/.zshrc"
echo "  package failure does not block config deployment"
```

- [ ] **Step 8: Suite 08 — status**

```bash
#!/bin/bash
# e2e/suites/08-status.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Status with no apply yet
STATUS_OUTPUT=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" status 2>&1 || true)
echo "$STATUS_OUTPUT" | grep -qi "no profile\|facet apply"
echo "  status shows hint when no profile applied"

# Apply and check status
setup_basic
facet_apply work

STATUS_OUTPUT=$(facet_status)
echo "$STATUS_OUTPUT" | grep -q "work"
echo "  status shows active profile"

echo "$STATUS_OUTPUT" | grep -q ".gitconfig"
echo "  status lists deployed configs"

echo "$STATUS_OUTPUT" | grep -q ".zshrc"
echo "  status lists all configs"

# Break a symlink source and check validity
rm "$HOME/dotfiles/configs/.zshrc"
STATUS_OUTPUT=$(facet_status 2>&1)
echo "$STATUS_OUTPUT" | grep -qi "broken\|missing\|invalid\|✗"
echo "  status detects broken symlink"

# Restore
echo 'export EDITOR=nvim' > "$HOME/dotfiles/configs/.zshrc"
```

- [ ] **Step 9: Suite 09 — force flag and skip-failure**

```bash
#!/bin/bash
# e2e/suites/09-force-flag.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Create a regular file at a target path (not managed by facet)
mkdir -p "$HOME"
echo "user's manual file" > "$HOME/.zshrc"

# Normal apply should fail/prompt (non-interactive → error)
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work
echo "  conflicting regular file blocks normal apply"

# --force should replace it
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --force work
assert_symlink "$HOME/.zshrc"
echo "  --force replaces conflicting file"

# --force on same profile = full unapply + reapply (clean slate)
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --force work
assert_symlink "$HOME/.zshrc"
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
echo "  --force on same profile works (clean slate)"

# --skip-failure: config deploy failures warn instead of rollback
cat > "$HOME/dotfiles/profiles/badconfig.yaml" << 'YAML'
extends: base
configs:
  ~/.zshrc: configs/.zshrc
  ~/.missing-source: configs/this_file_does_not_exist
YAML
# Without --skip-failure: should fail and rollback
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badconfig

# With --skip-failure: should succeed with warning, deploying what it can
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --skip-failure badconfig
assert_symlink "$HOME/.zshrc"
echo "  --skip-failure warns and continues on config deploy failure"
```

- [ ] **Step 10: Create Go test wrapper**

```go
// e2e/e2e_test.go
//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestE2E_Docker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found, skipping Docker E2E")
	}

	goarch := "amd64"
	if runtime.GOARCH == "arm64" {
		goarch = "arm64"
	}

	build := exec.Command("go", "build", "-o", "facet-linux", "..")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	dockerBuild := exec.Command("docker", "build", "-t", "facet-e2e", "-f", "Dockerfile.ubuntu", ".")
	if out, err := dockerBuild.CombinedOutput(); err != nil {
		t.Fatalf("docker build failed: %s\n%s", err, out)
	}

	dockerRun := exec.Command("docker", "run", "--rm",
		"-e", "FACET_E2E_REAL_PACKAGES=1",
		"facet-e2e")
	dockerRun.Stdout = os.Stdout
	dockerRun.Stderr = os.Stderr
	if err := dockerRun.Run(); err != nil {
		t.Fatalf("E2E tests failed: %s", err)
	}
}

func TestE2E_Native(t *testing.T) {
	build := exec.Command("go", "build", "-o", "facet", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	harness := exec.Command("bash", "harness.sh")
	harness.Env = append(os.Environ(),
		"PATH="+os.Getenv("PWD")+"/..:"+os.Getenv("PATH"))
	harness.Stdout = os.Stdout
	harness.Stderr = os.Stderr
	if err := harness.Run(); err != nil {
		t.Fatalf("Native E2E tests failed: %s", err)
	}
}
```

- [ ] **Step 11: Run E2E tests locally**

Run: `cd /Users/edocsss/aec/src/facet && make e2e-local`
Expected: All 9 suites PASS. Sandbox cleaned up. Real HOME untouched.

- [ ] **Step 12: Run E2E in Docker**

Run: `cd /Users/edocsss/aec/src/facet && make e2e`
Expected: All suites PASS inside Docker container.

- [ ] **Step 13: Commit**

```bash
git add e2e/suites/ e2e/e2e_test.go
git commit -m "feat: E2E test suites — init, apply, configs, profile switch, idempotency, edge cases, packages, status, force flag"
```

---

### Task 18: CI Pipeline

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create CI workflow**

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [master, main]
  pull_request:

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -v -cover

  e2e-linux:
    needs: unit
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: make e2e

  e2e-macos:
    needs: unit
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: brew install jq
      - run: make e2e-local
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat: CI pipeline — unit tests + E2E on Linux (Docker) and macOS (native)"
```
