package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"facet/internal/ai"
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)

type mockLoader struct {
	meta    *profile.FacetMeta
	configs map[string]*profile.FacetConfig
	err     error
}

func (m *mockLoader) LoadMeta(configDir string) (*profile.FacetMeta, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.meta, nil
}

func (m *mockLoader) LoadConfig(path string) (*profile.FacetConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	cfg, ok := m.configs[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return cfg, nil
}

type mockInstaller struct {
	results []packages.PackageResult
}

func (m *mockInstaller) InstallAll(pkgs []profile.PackageEntry) []packages.PackageResult {
	return m.results
}

type mockDeployer struct {
	deployed []deploy.ConfigResult
	err      error
}

func (m *mockDeployer) DeployOne(targetPath, source string, force bool) (deploy.ConfigResult, error) {
	r := deploy.ConfigResult{Target: targetPath, Source: source, Strategy: deploy.StrategySymlink}
	if m.err != nil {
		return r, m.err
	}
	m.deployed = append(m.deployed, r)
	return r, nil
}

func (m *mockDeployer) Unapply(configs []deploy.ConfigResult) error { return nil }
func (m *mockDeployer) Rollback() error                             { return nil }
func (m *mockDeployer) Deployed() []deploy.ConfigResult             { return m.deployed }

func TestApply_DryRun_NoSideEffects(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	// Create config source files so DetectStrategy works
	os.MkdirAll(filepath.Join(cfgDir, "configs"), 0o755)
	os.WriteFile(filepath.Join(cfgDir, "configs", ".zshrc"), []byte("# zshrc"), 0o644)

	// Create .local.yaml
	os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644)

	r := &mockReporter{}
	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				Packages: []profile.PackageEntry{
					{Name: "ripgrep", Install: profile.InstallCmd{Command: "brew install ripgrep"}},
				},
				Configs: map[string]string{
					filepath.Join(stateDir, ".zshrc"): "configs/.zshrc",
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	installerCalled := false
	mockInst := &mockInstaller{}
	origInstall := mockInst.InstallAll
	_ = origInstall
	// Use a tracking installer to prove it's never called
	trackingInstaller := &trackingInstaller{called: &installerCalled}

	mockDep := &mockDeployer{}
	a := New(Deps{
		Reporter:   r,
		Loader:     loader,
		Installer:  trackingInstaller,
		StateStore: &mockStateStore{},
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		DryRun:    true,
	})
	assert.NoError(t, err)

	// Installer should never be called
	assert.False(t, installerCalled, "installer should not be called during dry run")

	// Deployer should never be called
	assert.Empty(t, mockDep.deployed, "deployer should not be called during dry run")

	// Reporter should show dry-run header
	foundDryRun := false
	for _, msg := range r.messages {
		if contains(msg, "Dry run") {
			foundDryRun = true
		}
	}
	assert.True(t, foundDryRun, "should print dry-run header")
}

type trackingInstaller struct {
	called *bool
}

func (t *trackingInstaller) InstallAll(pkgs []profile.PackageEntry) []packages.PackageResult {
	*t.called = true
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestApply_MissingFacetYaml(t *testing.T) {
	r := &mockReporter{}
	loader := &mockLoader{err: fmt.Errorf("file not found")}

	a := New(Deps{
		Reporter: r,
		Loader:   loader,
	})

	err := a.Apply("work", ApplyOpts{ConfigDir: "/fake", StateDir: t.TempDir()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "facet config directory")
}

type mockAIOrchestrator struct {
	applyCalled   bool
	applyConfig   ai.EffectiveAIConfig
	applyPrev     *ai.AIState
	applyResult   *ai.AIState
	applyErr      error
	unapplyCalled bool
	unapplyPrev   *ai.AIState
	unapplyErr    error
}

func (m *mockAIOrchestrator) Apply(config ai.EffectiveAIConfig, previousState *ai.AIState) (*ai.AIState, error) {
	m.applyCalled = true
	m.applyConfig = config
	m.applyPrev = previousState
	return m.applyResult, m.applyErr
}

func (m *mockAIOrchestrator) Unapply(previousState *ai.AIState) error {
	m.unapplyCalled = true
	m.unapplyPrev = previousState
	return m.unapplyErr
}

func TestApply_WithAI(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	// Create .local.yaml
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	r := &mockReporter{}
	aiOrch := &mockAIOrchestrator{
		applyResult: &ai.AIState{
			Permissions: map[string]ai.PermissionState{
				"claude-code": {Allow: []string{"Read"}, Deny: []string{"Execute"}},
			},
		},
	}
	mockDep := &mockDeployer{}
	stateStore := &mockStateStore{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				AI: &profile.AIConfig{
					Agents: []string{"claude-code"},
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read"},
							Deny:  []string{"Execute"},
						},
					},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:       r,
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	// AI orchestrator should have been called
	assert.True(t, aiOrch.applyCalled, "AI orchestrator Apply should be called")
	assert.Contains(t, aiOrch.applyConfig, "claude-code")

	// State should include AI data
	require.NotNil(t, stateStore.written)
	require.NotNil(t, stateStore.written.AI)
	assert.Equal(t, []string{"Read"}, stateStore.written.AI.Permissions["claude-code"].Allow)
}

func TestApply_WithInheritedAIAgentsFromBase(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	aiOrch := &mockAIOrchestrator{}
	stateStore := &mockStateStore{}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				AI: &profile.AIConfig{
					Agents: []string{"claude-code"},
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read"},
						},
					},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				AI: &profile.AIConfig{
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read", "Edit"},
						},
					},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:       &mockReporter{},
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)
	assert.True(t, aiOrch.applyCalled, "AI orchestrator should be called for inherited agents")
}

func TestApply_WithoutAI(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	// Create .local.yaml
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	r := &mockReporter{}
	aiOrch := &mockAIOrchestrator{}
	mockDep := &mockDeployer{}
	stateStore := &mockStateStore{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"):             {},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:       r,
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	// AI orchestrator should NOT have been called (no AI config)
	assert.False(t, aiOrch.applyCalled, "AI orchestrator Apply should not be called when no AI config")

	// State should not include AI data
	require.NotNil(t, stateStore.written)
	assert.Nil(t, stateStore.written.AI)
}

func TestApply_SameProfileRemovalOfAISectionReconcilesPreviousState(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	aiOrch := &mockAIOrchestrator{}
	stateStore := &mockStateStore{
		state: &ApplyState{
			Profile: "work",
			AI: &ai.AIState{
				Permissions: map[string]ai.PermissionState{
					"claude-code": {Allow: []string{"Read"}},
				},
				Skills: []ai.SkillState{
					{Source: "@org/skills", Name: "code-review", Agents: []string{"claude-code"}},
				},
				MCPs: []ai.MCPState{
					{Name: "playwright", Agents: []string{"claude-code"}},
				},
			},
		},
	}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"):             {},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:       &mockReporter{},
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)
	assert.True(t, aiOrch.applyCalled, "AI orchestrator should reconcile previous AI state on same-profile removal")
	assert.Nil(t, aiOrch.applyConfig, "current AI config should be nil when AI section is removed")
	require.NotNil(t, stateStore.written)
	assert.Nil(t, stateStore.written.AI, "written state should omit AI after full removal")
}

func TestApply_ProfileSwitchTriggersAIUnapply(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	prevAIState := &ai.AIState{
		Permissions: map[string]ai.PermissionState{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{"Execute"}},
		},
		Skills: []ai.SkillState{
			{Source: "@org/skills", Name: "code-review", Agents: []string{"claude-code"}},
		},
		MCPs: []ai.MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
		},
	}

	aiOrch := &mockAIOrchestrator{}
	stateStore := &mockStateStore{
		state: &ApplyState{
			Profile: "work",
			AI:      prevAIState,
		},
	}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {},
			filepath.Join(cfgDir, "profiles", "personal.yaml"): {
				Extends: "base",
				AI: &profile.AIConfig{
					Agents: []string{"claude-code"},
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read"},
						},
					},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:       &mockReporter{},
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("personal", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	assert.True(t, aiOrch.unapplyCalled, "AI orchestrator Unapply should be called on profile switch")
	assert.Equal(t, prevAIState, aiOrch.unapplyPrev, "Unapply should receive the previous AI state")
	assert.True(t, aiOrch.applyCalled, "AI orchestrator Apply should still be called for the new profile")
	assert.Nil(t, aiOrch.applyPrev, "Apply should not diff against previous AI state after full unapply")
}

func TestApply_ForceTriggersAIUnapply(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	prevAIState := &ai.AIState{
		Permissions: map[string]ai.PermissionState{
			"claude-code": {Allow: []string{"Read"}, Deny: []string{"Execute"}},
		},
		Skills: []ai.SkillState{
			{Source: "@org/skills", Name: "code-review", Agents: []string{"claude-code"}},
		},
		MCPs: []ai.MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
		},
	}

	aiOrch := &mockAIOrchestrator{}
	stateStore := &mockStateStore{
		state: &ApplyState{
			Profile: "work",
			AI:      prevAIState,
		},
	}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				AI: &profile.AIConfig{
					Agents: []string{"claude-code"},
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read"},
						},
					},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:       &mockReporter{},
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     stateStore,
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		Force:     true,
	})
	require.NoError(t, err)

	assert.True(t, aiOrch.unapplyCalled, "AI orchestrator Unapply should be called on force apply")
	assert.True(t, aiOrch.applyCalled, "AI orchestrator Apply should still be called after force unapply")
	assert.Nil(t, aiOrch.applyPrev, "Apply should not diff against previous AI state after force unapply")
}

type mockScriptRunner struct {
	commands []string
	dirs     []string
	failOn   map[string]error
}

func (m *mockScriptRunner) Run(command string, dir string) error {
	m.commands = append(m.commands, command)
	m.dirs = append(m.dirs, dir)
	if err, ok := m.failOn[command]; ok {
		return err
	}
	return nil
}

func TestApply_RunsPreAndPostApplyScripts(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	mockDep := &mockDeployer{}
	stateStore := &mockStateStore{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "base-pre", Run: "echo base-pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "base-post", Run: "echo base-post"},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				PreApply: []profile.ScriptEntry{
					{Name: "profile-pre", Run: "echo profile-pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "profile-post", Run: "echo profile-post"},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &mockInstaller{},
		StateStore:   stateStore,
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	require.Len(t, runner.commands, 4)
	assert.Equal(t, "echo base-pre", runner.commands[0])
	assert.Equal(t, "echo profile-pre", runner.commands[1])
	assert.Equal(t, "echo base-post", runner.commands[2])
	assert.Equal(t, "echo profile-post", runner.commands[3])
}

func TestApply_PreApplyScriptFailureHaltsApply(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{
		failOn: map[string]error{
			"echo fail": fmt.Errorf("exit status 1"),
		},
	}
	installerCalled := false
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "will-fail", Run: "echo fail"},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &trackingInstaller{called: &installerCalled},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "will-fail")
	assert.False(t, installerCalled, "installer should not run after pre_apply failure")
}

func TestApply_StagesFiltering(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	installerCalled := false
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				PreApply: []profile.ScriptEntry{
					{Name: "pre", Run: "echo pre"},
				},
				PostApply: []profile.ScriptEntry{
					{Name: "post", Run: "echo post"},
				},
				Packages: []profile.PackageEntry{
					{Name: "git", Install: profile.InstallCmd{Command: "brew install git"}},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &trackingInstaller{called: &installerCalled},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		Stages:    "packages",
	})
	require.NoError(t, err)

	assert.True(t, installerCalled, "installer should run when packages stage is selected")
	assert.Empty(t, runner.commands, "scripts should not run when not in stages")
	assert.Empty(t, mockDep.deployed, "configs should not deploy when not in stages")
}

func TestApply_InvalidStageReturnsError(t *testing.T) {
	a := New(Deps{
		Reporter: &mockReporter{},
		Loader: &mockLoader{
			meta: &profile.FacetMeta{},
		},
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: "/fake",
		StateDir:  "/fake",
		Stages:    "invalid_stage",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stage")
}

func TestApply_ScriptsResolveVariables(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	runner := &mockScriptRunner{}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {
				Vars: map[string]any{
					"git": map[string]any{"email": "sarah@acme.com"},
				},
				PreApply: []profile.ScriptEntry{
					{Name: "setup", Run: `echo "${facet:git.email}"`},
				},
			},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {Extends: "base"},
			filepath.Join(stateDir, ".local.yaml"):         {},
		},
	}

	a := New(Deps{
		Reporter:     &mockReporter{},
		Loader:       loader,
		Installer:    &mockInstaller{},
		StateStore:   &mockStateStore{},
		ScriptRunner: runner,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
	})
	require.NoError(t, err)

	require.Len(t, runner.commands, 1)
	assert.Equal(t, `echo "sarah@acme.com"`, runner.commands[0])
}

func TestApply_DryRunSkipsAIOrchestrator(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(stateDir, ".local.yaml"), []byte(""), 0o644))

	aiOrch := &mockAIOrchestrator{}
	mockDep := &mockDeployer{}

	loader := &mockLoader{
		meta: &profile.FacetMeta{},
		configs: map[string]*profile.FacetConfig{
			filepath.Join(cfgDir, "base.yaml"): {},
			filepath.Join(cfgDir, "profiles", "work.yaml"): {
				Extends: "base",
				AI: &profile.AIConfig{
					Agents: []string{"claude-code"},
					Permissions: map[string]*profile.PermissionsConfig{
						"claude-code": {
							Allow: []string{"Read", "Edit"},
							Deny:  []string{"Execute"},
						},
					},
				},
			},
			filepath.Join(stateDir, ".local.yaml"): {},
		},
	}

	a := New(Deps{
		Reporter:       &mockReporter{},
		Loader:         loader,
		Installer:      &mockInstaller{},
		StateStore:     &mockStateStore{},
		AIOrchestrator: aiOrch,
		DeployerFactory: func(configDir, homeDir string, vars map[string]any, ownedConfigs []deploy.ConfigResult) deploy.Service {
			return mockDep
		},
		OSName: "macos",
	})

	err := a.Apply("work", ApplyOpts{
		ConfigDir: cfgDir,
		StateDir:  stateDir,
		DryRun:    true,
	})
	require.NoError(t, err)

	assert.False(t, aiOrch.applyCalled, "AI orchestrator Apply should not be called during dry run")
	assert.False(t, aiOrch.unapplyCalled, "AI orchestrator Unapply should not be called during dry run")
}
