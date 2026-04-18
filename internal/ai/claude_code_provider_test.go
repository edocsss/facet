package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"facet/internal/common/execrunner"
)

// helper: write a JSON file into dir with the given data.
func writeTestJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test JSON: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write test JSON: %v", err)
	}
}

// helper: read back a JSON file from disk.
func readTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return result
}

func TestClaudeCodeProvider_ApplyPermissions_MergeWithExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-existing settings with unrelated key and old perms.
	writeTestJSON(t, settingsPath, map[string]any{
		"model": "claude-opus-4-5",
		"permissions": map[string]any{
			"allow": []any{"OldTool"},
			"deny":  []any{"OldDenied"},
			"ask":   []any{"Bash(git push *)"},
		},
	})

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	perms := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.ApplyPermissions(perms); err != nil {
		t.Fatalf("ApplyPermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	// "model" key must be preserved.
	if result["model"] != "claude-opus-4-5" {
		t.Errorf("expected model preserved, got %v", result["model"])
	}

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	// permissions.allow updated.
	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 2 || allowed[0] != "Bash" || allowed[1] != "Read" {
		t.Errorf("unexpected permissions.allow: %v", allowed)
	}

	// permissions.deny updated.
	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 1 || denied[0] != "Edit" {
		t.Errorf("unexpected permissions.deny: %v", denied)
	}

	ask, ok := permissions["ask"].([]any)
	if !ok || len(ask) != 1 || ask[0] != "Bash(git push *)" {
		t.Errorf("expected permissions.ask to be preserved, got %v", permissions["ask"])
	}
}

func TestClaudeCodeProvider_ApplyPermissions_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	// File does not exist.

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	perms := ResolvedPermissions{
		Allow: []string{"Bash"},
		Deny:  []string{},
	}
	if err := p.ApplyPermissions(perms); err != nil {
		t.Fatalf("ApplyPermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 1 || allowed[0] != "Bash" {
		t.Errorf("unexpected permissions.allow: %v", allowed)
	}

	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 0 {
		t.Errorf("expected empty permissions.deny, got: %v", denied)
	}
}

func TestClaudeCodeProvider_RemovePermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	writeTestJSON(t, settingsPath, map[string]any{
		"model": "claude-opus-4-5",
		"permissions": map[string]any{
			"allow": []any{"Bash", "Read"},
			"deny":  []any{"Edit"},
			"ask":   []any{"Bash(git push *)"},
		},
	})

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	prev := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.RemovePermissions(prev); err != nil {
		t.Fatalf("RemovePermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	// "model" must be preserved.
	if result["model"] != "claude-opus-4-5" {
		t.Errorf("expected model preserved, got %v", result["model"])
	}

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	// permissions.allow must be empty slice.
	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 0 {
		t.Errorf("expected empty permissions.allow after remove, got: %v", allowed)
	}

	// permissions.deny must be empty slice.
	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 0 {
		t.Errorf("expected empty permissions.deny after remove, got: %v", denied)
	}

	ask, ok := permissions["ask"].([]any)
	if !ok || len(ask) != 1 || ask[0] != "Bash(git push *)" {
		t.Errorf("expected permissions.ask to be preserved, got %v", permissions["ask"])
	}
}

func TestClaudeCodeProvider_RegisterMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"ROOT": "/tmp"},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(runner.commands), runner.commands)
	}

	cmd := runner.commands[0]
	// Must contain the base invocation with user scope.
	if !contains(cmd, "claude mcp add filesystem --scope user") {
		t.Errorf("command missing 'claude mcp add filesystem --scope user': %q", cmd)
	}
	// Must contain the env var flag.
	if !contains(cmd, "-e ROOT=/tmp") {
		t.Errorf("command missing env flag '-e ROOT=/tmp': %q", cmd)
	}
	// Must contain the separator and command.
	if !contains(cmd, "-- npx") {
		t.Errorf("command missing '-- npx': %q", cmd)
	}
	// Must contain args.
	if !contains(cmd, "-y") || !contains(cmd, "@modelcontextprotocol/server-filesystem") {
		t.Errorf("command missing args: %q", cmd)
	}
}

func TestClaudeCodeProvider_RegisterMCP_NoEnvNoArgs(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	mcp := ResolvedMCP{
		Name:    "simple",
		Command: "myserver",
		Args:    []string{},
		Env:     map[string]string{},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	cmd := runner.commands[0]
	expected := "claude mcp add simple --scope user -- myserver"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestClaudeCodeProvider_RegisterMCP_AlreadyExists_RemovesAndRetries(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &sequentialMockRunner{
		errors: []error{
			fmt.Errorf("exit status 1: MCP server filesystem already exists in local config"),
			nil, // remove succeeds
			nil, // retry add succeeds
		},
	}
	p := NewClaudeCodeProvider(settingsPath, runner)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	if len(runner.commands) != 3 {
		t.Fatalf("expected 3 commands (add, remove, add), got %d: %v", len(runner.commands), runner.commands)
	}

	if !contains(runner.commands[0], "claude mcp add filesystem --scope user") {
		t.Errorf("first command should be add with user scope, got %q", runner.commands[0])
	}
	if runner.commands[1] != "claude mcp remove filesystem --scope user" {
		t.Errorf("second command should be user-scope remove, got %q", runner.commands[1])
	}
	if !contains(runner.commands[2], "claude mcp add filesystem --scope user") {
		t.Errorf("third command should be add retry with user scope, got %q", runner.commands[2])
	}
}

func TestClaudeCodeProvider_RegisterMCP_AlreadyExists_RemoveFails(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &sequentialMockRunner{
		errors: []error{
			fmt.Errorf("exit status 1: MCP server filesystem already exists in local config"),
			fmt.Errorf("exit status 1: remove failed"), // remove fails
		},
	}
	p := NewClaudeCodeProvider(settingsPath, runner)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{},
	}

	err := p.RegisterMCP(mcp)
	if err == nil {
		t.Fatal("expected error when remove fails, got nil")
	}
	if !contains(err.Error(), "remove existing MCP") {
		t.Errorf("expected error to mention remove, got %q", err.Error())
	}
	if len(runner.commands) != 2 {
		t.Fatalf("expected 2 commands (add, remove), got %d: %v", len(runner.commands), runner.commands)
	}
}

func TestClaudeCodeProvider_RegisterMCP_NonAlreadyExistsError(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &mockRunner{
		err: fmt.Errorf("exit status 1: network timeout"),
	}
	p := NewClaudeCodeProvider(settingsPath, runner)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{},
	}

	err := p.RegisterMCP(mcp)
	if err == nil {
		t.Fatal("expected error for non-already-exists failure, got nil")
	}
	if !contains(err.Error(), "network timeout") {
		t.Errorf("expected original error, got %q", err.Error())
	}
	// Should not attempt remove — only 1 command issued.
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command (add only), got %d: %v", len(runner.commands), runner.commands)
	}
}

func TestClaudeCodeProvider_RemoveMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	if err := p.RemoveMCP("filesystem"); err != nil {
		t.Fatalf("RemoveMCP returned error: %v", err)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(runner.commands), runner.commands)
	}

	expected := "claude mcp remove filesystem --scope user"
	if runner.commands[0] != expected {
		t.Errorf("expected %q, got %q", expected, runner.commands[0])
	}
}

func TestClaudeCodeProvider_RegisterMCP_PreservesArgumentBoundaries(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	logPath := filepath.Join(dir, "claude-args.log")
	binDir := filepath.Join(dir, "bin")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	script := "#!/bin/sh\n" +
		"for arg in \"$@\"; do\n" +
		"  printf '%s\\n' \"$arg\" >> \"$LOGFILE\"\n" +
		"done\n"
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write claude stub: %v", err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("LOGFILE", logPath)

	p := NewClaudeCodeProvider(settingsPath, execrunner.New())

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "python3",
		Args:    []string{"server.py", "--root", "/tmp/my dir"},
		Env:     map[string]string{"ROOT": "/tmp/path with spaces"},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read arg log: %v", err)
	}

	got := splitNonEmptyLines(string(data))
	want := []string{
		"mcp",
		"add",
		"filesystem",
		"--scope",
		"user",
		"-e",
		"ROOT=/tmp/path with spaces",
		"--",
		"python3",
		"server.py",
		"--root",
		"/tmp/my dir",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected arg count:\n  got:  %#v\n  want: %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d]: got %q, want %q\nfull got: %#v", i, got[i], want[i], got)
		}
	}
}

func TestClaudeCodeProvider_Name(t *testing.T) {
	p := NewClaudeCodeProvider("/some/path", &mockRunner{})
	if p.Name() != "claude-code" {
		t.Errorf("expected 'claude-code', got %q", p.Name())
	}
}

func TestClaudeCodeProvider_SettingsFilePath(t *testing.T) {
	p := NewClaudeCodeProvider("/some/path/settings.json", &mockRunner{})
	if p.SettingsFilePath() != "/some/path/settings.json" {
		t.Errorf("unexpected settings path: %q", p.SettingsFilePath())
	}
}

func TestClaudeCodeProvider_ApplyPermissions_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write corrupted JSON to settings path.
	if err := os.WriteFile(settingsPath, []byte(`{invalid json`), 0o644); err != nil {
		t.Fatalf("failed to write corrupted JSON: %v", err)
	}

	runner := &mockRunner{}
	p := NewClaudeCodeProvider(settingsPath, runner)

	perms := ResolvedPermissions{
		Allow: []string{"Bash"},
		Deny:  []string{},
	}
	err := p.ApplyPermissions(perms)
	if err == nil {
		t.Fatal("expected error for corrupted JSON, got nil")
	}
	if !contains(err.Error(), "parse") {
		t.Errorf("expected error to contain 'parse', got %q", err.Error())
	}
}

// contains is a simple substring check for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		indexStr(s, substr) >= 0)
}

func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func splitNonEmptyLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i != len(s) && s[i] != '\n' {
			continue
		}
		if i > start {
			out = append(out, s[start:i])
		}
		start = i + 1
	}
	return out
}
