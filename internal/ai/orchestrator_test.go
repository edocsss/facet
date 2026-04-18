package ai

import (
	"errors"
	"fmt"
	"sort"
	"testing"
)

type mockProvider struct {
	name               string
	appliedPermissions *ResolvedPermissions
	removedPermissions *ResolvedPermissions
	registeredMCPs     []ResolvedMCP
	removedMCPs        []string
	applyPermErr       error
	registerMCPErr     error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) ApplyPermissions(perms ResolvedPermissions) error {
	if m.applyPermErr != nil {
		return m.applyPermErr
	}
	m.appliedPermissions = &perms
	return nil
}

func (m *mockProvider) RemovePermissions(perms ResolvedPermissions) error {
	m.removedPermissions = &perms
	return nil
}

func (m *mockProvider) RegisterMCP(mcp ResolvedMCP) error {
	if m.registerMCPErr != nil {
		return m.registerMCPErr
	}
	m.registeredMCPs = append(m.registeredMCPs, mcp)
	return nil
}

func (m *mockProvider) RemoveMCP(name string) error {
	m.removedMCPs = append(m.removedMCPs, name)
	return nil
}

func (m *mockProvider) SettingsFilePath() string {
	return fmt.Sprintf("/mock/%s/settings.json", m.name)
}

type mockSkillsMgr struct {
	installed []struct {
		source string
		skills []string
		agents []string
	}
	sourceSkills map[string][]string
	removed      []struct {
		skills []string
		agents []string
	}
	installErr error
	removeErr  error
}

func (m *mockSkillsMgr) Install(source string, skills []string, agents []string) error {
	if m.installErr != nil {
		return m.installErr
	}
	m.installed = append(m.installed, struct {
		source string
		skills []string
		agents []string
	}{source: source, skills: skills, agents: agents})
	return nil
}

func (m *mockSkillsMgr) Remove(skills []string, agents []string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, struct {
		skills []string
		agents []string
	}{skills: skills, agents: agents})
	return nil
}

func (m *mockSkillsMgr) InstalledForSource(source string) ([]string, error) {
	return append([]string{}, m.sourceSkills[source]...), nil
}

func (m *mockSkillsMgr) Check() error  { return nil }
func (m *mockSkillsMgr) Update() error { return nil }

type mockReporter struct{}

func (m *mockReporter) Success(_ string)       {}
func (m *mockReporter) Warning(_ string)       {}
func (m *mockReporter) Error(_ string)         {}
func (m *mockReporter) Header(_ string)        {}
func (m *mockReporter) PrintLine(_ string)     {}
func (m *mockReporter) Dim(text string) string { return text }

type orderTracker struct {
	calls []string
}

type trackingProvider struct {
	name    string
	tracker *orderTracker
}

func (t *trackingProvider) Name() string { return t.name }

func (t *trackingProvider) ApplyPermissions(_ ResolvedPermissions) error {
	t.tracker.calls = append(t.tracker.calls, "apply-permissions")
	return nil
}

func (t *trackingProvider) RemovePermissions(_ ResolvedPermissions) error {
	t.tracker.calls = append(t.tracker.calls, "remove-permissions")
	return nil
}

func (t *trackingProvider) RegisterMCP(_ ResolvedMCP) error {
	t.tracker.calls = append(t.tracker.calls, "register-mcp")
	return nil
}

func (t *trackingProvider) RemoveMCP(_ string) error {
	t.tracker.calls = append(t.tracker.calls, "remove-mcp")
	return nil
}

func (t *trackingProvider) SettingsFilePath() string {
	return fmt.Sprintf("/mock/%s/settings.json", t.name)
}

type trackingSkillsMgr struct {
	tracker *orderTracker
}

func (t *trackingSkillsMgr) Install(_ string, _ []string, _ []string) error {
	t.tracker.calls = append(t.tracker.calls, "install-skill")
	return nil
}

func (t *trackingSkillsMgr) Remove(_ []string, _ []string) error {
	t.tracker.calls = append(t.tracker.calls, "remove-skill")
	return nil
}

func (t *trackingSkillsMgr) InstalledForSource(_ string) ([]string, error) {
	return nil, nil
}

func (t *trackingSkillsMgr) Check() error  { return nil }
func (t *trackingSkillsMgr) Update() error { return nil }

type selectiveSkillsMgr struct {
	failSource string
	installed  []struct {
		source string
		skills []string
		agents []string
	}
	removed []struct {
		skills []string
		agents []string
	}
}

func (s *selectiveSkillsMgr) Install(source string, skills []string, agents []string) error {
	if source == s.failSource {
		return fmt.Errorf("install failed for source %q", source)
	}
	s.installed = append(s.installed, struct {
		source string
		skills []string
		agents []string
	}{source: source, skills: skills, agents: agents})
	return nil
}

func (s *selectiveSkillsMgr) Remove(skills []string, agents []string) error {
	s.removed = append(s.removed, struct {
		skills []string
		agents []string
	}{skills: skills, agents: agents})
	return nil
}

func (s *selectiveSkillsMgr) InstalledForSource(_ string) ([]string, error) {
	return nil, nil
}

func (s *selectiveSkillsMgr) Check() error  { return nil }
func (s *selectiveSkillsMgr) Update() error { return nil }

func TestOrchestrator_Apply_Permissions(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Permissions: ResolvedPermissions{
				Allow: []string{"Read", "Edit"},
				Deny:  []string{"Bash"},
			},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if provider.appliedPermissions == nil {
		t.Fatal("expected permissions to be applied")
	}
	if len(provider.appliedPermissions.Allow) != 2 || provider.appliedPermissions.Allow[0] != "Read" || provider.appliedPermissions.Allow[1] != "Edit" {
		t.Fatalf("unexpected applied allow: %v", provider.appliedPermissions.Allow)
	}
	if len(provider.appliedPermissions.Deny) != 1 || provider.appliedPermissions.Deny[0] != "Bash" {
		t.Fatalf("unexpected applied deny: %v", provider.appliedPermissions.Deny)
	}
	if ps, ok := state.Permissions["claude-code"]; !ok || len(ps.Allow) != 2 || ps.Allow[0] != "Read" || ps.Allow[1] != "Edit" {
		t.Fatalf("unexpected permission state: %+v", state.Permissions)
	}
}

func TestOrchestrator_Apply_SkillOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "skill-1", Agents: []string{"claude-code"}},
			{Source: "@org/skills", Name: "skill-2", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: "skill-1"}},
		},
	}

	state, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 1 || skillsMgr.removed[0].skills[0] != "skill-2" {
		t.Fatalf("expected orphan skill-2 removal, got %+v", skillsMgr.removed)
	}
	if len(skillsMgr.installed) != 1 || skillsMgr.installed[0].skills[0] != "skill-1" {
		t.Fatalf("expected skill-1 install, got %+v", skillsMgr.installed)
	}
	if len(state.Skills) != 1 || state.Skills[0].Name != "skill-1" {
		t.Fatalf("unexpected state skills: %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_MCPOrphanRemoval(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		MCPs: []MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
			{Name: "github", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			MCPs: []ResolvedMCP{{Name: "playwright", Command: "npx", Args: []string{"playwright-mcp"}}},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(provider.removedMCPs) != 1 || provider.removedMCPs[0] != "github" {
		t.Fatalf("expected github orphan removal, got %v", provider.removedMCPs)
	}
	if len(provider.registeredMCPs) != 1 || provider.registeredMCPs[0].Name != "playwright" {
		t.Fatalf("expected playwright registration, got %+v", provider.registeredMCPs)
	}
}

func TestOrchestrator_Apply_NonFatalPermissionError(t *testing.T) {
	failProvider := &mockProvider{name: "cursor", applyPermErr: errors.New("cursor write failed")}
	okProvider := &mockProvider{name: "claude-code"}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": okProvider,
			"cursor":      failProvider,
		},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {Permissions: ResolvedPermissions{Allow: []string{"Read"}}},
		"cursor":      {Permissions: ResolvedPermissions{Allow: []string{"Read(**)"}}},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply should not return error on partial failure: %v", err)
	}
	if _, ok := state.Permissions["claude-code"]; !ok {
		t.Fatal("expected claude-code in state")
	}
	if _, ok := state.Permissions["cursor"]; ok {
		t.Fatal("did not expect cursor in state after failure")
	}
}

func TestOrchestrator_Unapply(t *testing.T) {
	provider := &mockProvider{name: "claude-code"}
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": provider},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{
			"claude-code": {Allow: []string{"Read", "Edit"}, Deny: []string{"Bash"}},
		},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "skill-1", Agents: []string{"claude-code"}},
		},
		MCPs: []MCPState{
			{Name: "playwright", Agents: []string{"claude-code"}},
		},
	}

	if err := orch.Unapply(previousState); err != nil {
		t.Fatalf("Unapply returned unexpected error: %v", err)
	}
	if len(provider.removedMCPs) != 1 || provider.removedMCPs[0] != "playwright" {
		t.Fatalf("unexpected MCP removals: %v", provider.removedMCPs)
	}
	if len(skillsMgr.removed) != 1 || skillsMgr.removed[0].skills[0] != "skill-1" {
		t.Fatalf("unexpected skill removals: %+v", skillsMgr.removed)
	}
	if provider.removedPermissions == nil || len(provider.removedPermissions.Allow) != 2 {
		t.Fatalf("unexpected permission removals: %+v", provider.removedPermissions)
	}
}

func TestOrchestrator_Unapply_NilState(t *testing.T) {
	orch := NewOrchestrator(map[string]AgentProvider{}, &mockSkillsMgr{}, &mockReporter{})
	if err := orch.Unapply(nil); err != nil {
		t.Fatalf("Unapply(nil) should return nil, got: %v", err)
	}
}

func TestOrchestrator_Unapply_Order(t *testing.T) {
	tracker := &orderTracker{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &trackingProvider{name: "claude-code", tracker: tracker}},
		&trackingSkillsMgr{tracker: tracker},
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{"claude-code": {Allow: []string{"Read"}}},
		Skills:      []SkillState{{Source: "@org/skills", Name: "skill-1", Agents: []string{"claude-code"}}},
		MCPs:        []MCPState{{Name: "playwright", Agents: []string{"claude-code"}}},
	}

	if err := orch.Unapply(previousState); err != nil {
		t.Fatalf("Unapply returned unexpected error: %v", err)
	}
	expected := []string{"remove-mcp", "remove-skill", "remove-permissions"}
	if len(tracker.calls) != len(expected) {
		t.Fatalf("unexpected call count: %v", tracker.calls)
	}
	for i, want := range expected {
		if tracker.calls[i] != want {
			t.Fatalf("call[%d]: expected %q, got %q (%v)", i, want, tracker.calls[i], tracker.calls)
		}
	}
}

func TestOrchestrator_Apply_SkillGrouping(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": &mockProvider{name: "claude-code"},
			"cursor":      &mockProvider{name: "cursor"},
		},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-a"},
				{Source: "@org/skills", Name: "skill-b"},
				{Source: "@org/skills", Name: "skill-c"},
			},
		},
		"cursor": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: "skill-b"}},
		},
	}

	if _, err := orch.Apply(config, nil); err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 2 {
		t.Fatalf("expected 2 install calls, got %+v", skillsMgr.installed)
	}
	sort.Slice(skillsMgr.installed, func(i, j int) bool {
		return len(skillsMgr.installed[i].agents) < len(skillsMgr.installed[j].agents)
	})
	first := skillsMgr.installed[0]
	sort.Strings(first.skills)
	if len(first.skills) != 2 || first.skills[0] != "skill-a" || first.skills[1] != "skill-c" {
		t.Fatalf("unexpected first grouped skills: %+v", first)
	}
	second := skillsMgr.installed[1]
	if len(second.skills) != 1 || second.skills[0] != "skill-b" {
		t.Fatalf("unexpected second grouped skills: %+v", second)
	}
}

func TestOrchestrator_Apply_UnknownProviderWarning(t *testing.T) {
	orch := NewOrchestrator(map[string]AgentProvider{}, &mockSkillsMgr{}, &mockReporter{})
	config := EffectiveAIConfig{
		"unknown-agent": {Permissions: ResolvedPermissions{Allow: []string{"Read"}}},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply should not return error for unknown provider: %v", err)
	}
	if _, ok := state.Permissions["unknown-agent"]; ok {
		t.Fatalf("did not expect unknown-agent in state: %+v", state.Permissions)
	}
}

func TestOrchestrator_Apply_MCPRegistrationPartialFailure(t *testing.T) {
	failProvider := &mockProvider{name: "cursor", registerMCPErr: errors.New("cursor MCP failed")}
	okProvider := &mockProvider{name: "claude-code"}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": okProvider,
			"cursor":      failProvider,
		},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {MCPs: []ResolvedMCP{{Name: "playwright", Command: "npx", Args: []string{"playwright-mcp"}}}},
		"cursor":      {MCPs: []ResolvedMCP{{Name: "playwright", Command: "npx", Args: []string{"playwright-mcp"}}}},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply should not return error: %v", err)
	}
	if len(state.MCPs) != 1 || state.MCPs[0].Name != "playwright" || len(state.MCPs[0].Agents) != 1 || state.MCPs[0].Agents[0] != "claude-code" {
		t.Fatalf("unexpected MCP state: %+v", state.MCPs)
	}
}

func TestOrchestrator_Apply_SameSkillNameDifferentSources(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills-a", Name: "shared-name"},
				{Source: "@org/skills-b", Name: "shared-name"},
			},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 2 || len(state.Skills) != 2 {
		t.Fatalf("expected two source-distinct installs, got installs=%+v state=%+v", skillsMgr.installed, state.Skills)
	}
}

func TestOrchestrator_Apply_SkillOrphanRemoval_PerAgentDelta(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": &mockProvider{name: "claude-code"},
			"cursor":      &mockProvider{name: "cursor"},
		},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "frontend-design", Agents: []string{"claude-code", "cursor"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {Skills: []ResolvedSkill{{Source: "@org/skills", Name: "frontend-design"}}},
	}

	state, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 1 || len(skillsMgr.removed[0].agents) != 1 || skillsMgr.removed[0].agents[0] != "cursor" {
		t.Fatalf("expected cursor-only orphan removal, got %+v", skillsMgr.removed)
	}
	if len(state.Skills) != 1 || len(state.Skills[0].Agents) != 1 || state.Skills[0].Agents[0] != "claude-code" {
		t.Fatalf("unexpected state skills: %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_MCPOrphanRemoval_PerAgentDelta(t *testing.T) {
	claudeProvider := &mockProvider{name: "claude-code"}
	cursorProvider := &mockProvider{name: "cursor"}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": claudeProvider,
			"cursor":      cursorProvider,
		},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		MCPs: []MCPState{
			{Name: "playwright", Agents: []string{"claude-code", "cursor"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {MCPs: []ResolvedMCP{{Name: "playwright", Command: "npx", Args: []string{"playwright"}}}},
	}

	state, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(cursorProvider.removedMCPs) != 1 || cursorProvider.removedMCPs[0] != "playwright" {
		t.Fatalf("expected cursor MCP removal, got %v", cursorProvider.removedMCPs)
	}
	if len(state.MCPs) != 1 || len(state.MCPs[0].Agents) != 1 || state.MCPs[0].Agents[0] != "claude-code" {
		t.Fatalf("unexpected MCP state: %+v", state.MCPs)
	}
}

func TestOrchestrator_Apply_Order(t *testing.T) {
	tracker := &orderTracker{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &trackingProvider{name: "claude-code", tracker: tracker}},
		&trackingSkillsMgr{tracker: tracker},
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Permissions: ResolvedPermissions{Allow: []string{"Read"}},
			Skills:      []ResolvedSkill{{Source: "@org/skills", Name: "skill-1"}},
			MCPs:        []ResolvedMCP{{Name: "playwright", Command: "npx", Args: []string{"playwright-mcp"}}},
		},
	}

	if _, err := orch.Apply(config, nil); err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	expected := []string{"apply-permissions", "install-skill", "register-mcp"}
	if len(tracker.calls) != len(expected) {
		t.Fatalf("unexpected call count: %v", tracker.calls)
	}
	for i, want := range expected {
		if tracker.calls[i] != want {
			t.Fatalf("call[%d]: expected %q, got %q (%v)", i, want, tracker.calls[i], tracker.calls)
		}
	}
}

func TestOrchestrator_Apply_SkillInstallPartialFailure(t *testing.T) {
	ssm := &selectiveSkillsMgr{failSource: "@org/failing-skills"}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		ssm,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/good-skills", Name: "skill-ok"},
				{Source: "@org/failing-skills", Name: "skill-fail"},
			},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply should not return error on partial skill failure: %v", err)
	}
	if len(state.Skills) != 1 || state.Skills[0].Name != "skill-ok" {
		t.Fatalf("unexpected state skills: %+v", state.Skills)
	}
	if len(ssm.installed) != 1 || ssm.installed[0].source != "@org/good-skills" {
		t.Fatalf("unexpected installs: %+v", ssm.installed)
	}
}

func TestOrchestrator_Apply_AllSkillsFromSource(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: ""}},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 1 {
		t.Fatalf("expected 1 install call, got %d: %+v", len(skillsMgr.installed), skillsMgr.installed)
	}
	inst := skillsMgr.installed[0]
	if inst.source != "@org/skills" {
		t.Errorf("expected source @org/skills, got %q", inst.source)
	}
	if inst.skills != nil {
		t.Errorf("expected nil skills (all), got %v", inst.skills)
	}
	if len(inst.agents) != 1 || inst.agents[0] != "claude-code" {
		t.Errorf("unexpected agents: %v", inst.agents)
	}
	if len(state.Skills) != 2 || state.Skills[0].Name != "skill-a" || state.Skills[1].Name != "skill-b" {
		t.Fatalf("unexpected state: %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_AllSkillsFromSource_TracksResolvedStateForLaterRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"git@github.com:org/skills.git": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "git@github.com:org/skills.git", Name: ""}},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 2 {
		t.Fatalf("expected resolved state entries, got %+v", state.Skills)
	}

	skillsMgr.sourceSkills["git@github.com:org/skills.git"] = []string{"skill-a", "skill-b", "skill-c"}

	_, err = orch.Apply(EffectiveAIConfig{"claude-code": {}}, state)
	if err != nil {
		t.Fatalf("second Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 2 {
		t.Fatalf("expected 2 named removals, got %+v", skillsMgr.removed)
	}
	sort.Slice(skillsMgr.removed, func(i, j int) bool {
		return skillsMgr.removed[i].skills[0] < skillsMgr.removed[j].skills[0]
	})
	if skillsMgr.removed[0].skills[0] != "skill-a" || skillsMgr.removed[1].skills[0] != "skill-b" {
		t.Fatalf("expected tracked skills only to be removed, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_MixedAllAndSpecificSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/all-skills": {"skill-x", "skill-y"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/all-skills", Name: ""},
				{Source: "@org/specific", Name: "skill-a"},
				{Source: "@org/specific", Name: "skill-b"},
			},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.installed) != 2 {
		t.Fatalf("expected 2 install calls, got %d: %+v", len(skillsMgr.installed), skillsMgr.installed)
	}

	sort.Slice(skillsMgr.installed, func(i, j int) bool {
		return skillsMgr.installed[i].source < skillsMgr.installed[j].source
	})

	allInst := skillsMgr.installed[0]
	if allInst.source != "@org/all-skills" || allInst.skills != nil {
		t.Errorf("expected all-skills with nil skills, got %+v", allInst)
	}

	specInst := skillsMgr.installed[1]
	sort.Strings(specInst.skills)
	if specInst.source != "@org/specific" || len(specInst.skills) != 2 || specInst.skills[0] != "skill-a" || specInst.skills[1] != "skill-b" {
		t.Errorf("expected specific skills, got %+v", specInst)
	}

	if len(state.Skills) != 4 {
		t.Fatalf("expected 3 state entries, got %+v", state.Skills)
	}
}

func TestOrchestrator_Apply_AllSkillsFromSource_UnresolvedLockSkipsStateTracking(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: ""}},
		},
	}

	state, err := orch.Apply(config, nil)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(state.Skills) != 0 {
		t.Fatalf("expected unresolved all-source install to skip state tracking, got %+v", state.Skills)
	}

	skillsMgr.sourceSkills = map[string][]string{
		"@org/skills": {"skill-a", "skill-b"},
	}
	_, err = orch.Apply(EffectiveAIConfig{"claude-code": {}}, state)
	if err != nil {
		t.Fatalf("second Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals for untracked all-source install, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_SpecificToAllTransition_SkipsOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "skill-1", Agents: []string{"claude-code"}},
			{Source: "@org/skills", Name: "skill-2", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: ""}},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals (all covers previous), got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_AllToSpecificTransition_SkipsOrphanRemoval(t *testing.T) {
	skillsMgr := &mockSkillsMgr{}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{
				{Source: "@org/skills", Name: "skill-1"},
			},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 0 {
		t.Fatalf("expected no removals for all-to-specific transition, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_AllToNothing_RemovesResolvedSourceSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 1 {
		t.Fatalf("expected 1 removal for all-to-nothing transition, got %+v", skillsMgr.removed)
	}
	sort.Strings(skillsMgr.removed[0].skills)
	if len(skillsMgr.removed[0].skills) != 2 || skillsMgr.removed[0].skills[0] != "skill-a" || skillsMgr.removed[0].skills[1] != "skill-b" {
		t.Fatalf("expected skill-a and skill-b removal, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_AllToSpecific_RemovesOnlyDroppedSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {
			Skills: []ResolvedSkill{{Source: "@org/skills", Name: "skill-a"}},
		},
	}

	_, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 1 {
		t.Fatalf("expected 1 removal, got %+v", skillsMgr.removed)
	}
	if len(skillsMgr.removed[0].skills) != 1 || skillsMgr.removed[0].skills[0] != "skill-b" {
		t.Errorf("expected skill-b removal, got %+v", skillsMgr.removed[0])
	}
}

func TestOrchestrator_Unapply_AllSource_RemovesResolvedSourceSkills(t *testing.T) {
	skillsMgr := &mockSkillsMgr{
		sourceSkills: map[string][]string{
			"@org/skills": {"skill-a", "skill-b"},
			"@org/other":  {"skill-1"},
		},
	}
	orch := NewOrchestrator(
		map[string]AgentProvider{"claude-code": &mockProvider{name: "claude-code"}},
		skillsMgr,
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{},
		Skills: []SkillState{
			{Source: "@org/skills", Name: "", Agents: []string{"claude-code"}},
			{Source: "@org/other", Name: "skill-1", Agents: []string{"claude-code"}},
		},
	}

	if err := orch.Unapply(previousState); err != nil {
		t.Fatalf("Unapply returned unexpected error: %v", err)
	}
	if len(skillsMgr.removed) != 2 {
		t.Fatalf("expected 2 removals, got %+v", skillsMgr.removed)
	}
	byJoinedSkills := make(map[string][]string, len(skillsMgr.removed))
	for _, removal := range skillsMgr.removed {
		skills := append([]string{}, removal.skills...)
		sort.Strings(skills)
		byJoinedSkills[fmt.Sprintf("%v", skills)] = removal.agents
	}
	if _, exists := byJoinedSkills["[skill-a skill-b]"]; !exists {
		t.Errorf("expected all-source skills removal, got %+v", skillsMgr.removed)
	}
	if _, exists := byJoinedSkills["[skill-1]"]; !exists {
		t.Errorf("expected skill-1 removal, got %+v", skillsMgr.removed)
	}
}

func TestOrchestrator_Apply_RemovesPermissionsForDroppedAgents(t *testing.T) {
	claudeProvider := &mockProvider{name: "claude-code"}
	cursorProvider := &mockProvider{name: "cursor"}
	orch := NewOrchestrator(
		map[string]AgentProvider{
			"claude-code": claudeProvider,
			"cursor":      cursorProvider,
		},
		&mockSkillsMgr{},
		&mockReporter{},
	)

	previousState := &AIState{
		Permissions: map[string]PermissionState{
			"claude-code": {Allow: []string{"Read"}},
			"cursor":      {Allow: []string{"Read(**)", "Write(**)"}},
		},
	}
	config := EffectiveAIConfig{
		"claude-code": {Permissions: ResolvedPermissions{Allow: []string{"Read"}}},
	}

	state, err := orch.Apply(config, previousState)
	if err != nil {
		t.Fatalf("Apply returned unexpected error: %v", err)
	}
	if cursorProvider.removedPermissions == nil {
		t.Fatal("expected dropped cursor permissions to be removed")
	}
	if _, ok := state.Permissions["cursor"]; ok {
		t.Fatalf("did not expect cursor in current permission state: %+v", state.Permissions)
	}
}
