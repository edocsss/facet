package ai

import (
	"path/filepath"
	"testing"
)

func TestCursorProvider_ApplyPermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "cli-config.json")
	mcpPath := filepath.Join(dir, "mcp.json")

	// Pre-existing settings with unrelated key.
	writeTestJSON(t, settingsPath, map[string]any{
		"version": 1,
		"editor":  map[string]any{"vimMode": true},
		"permissions": map[string]any{
			"allow": []any{"Shell(git)"},
			"deny":  []any{"Read(.env*)"},
		},
	})

	p := NewCursorProvider(settingsPath, mcpPath)

	perms := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.ApplyPermissions(perms); err != nil {
		t.Fatalf("ApplyPermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	if result["version"] != float64(1) {
		t.Errorf("expected version preserved, got %v", result["version"])
	}

	editor, ok := result["editor"].(map[string]any)
	if !ok || editor["vimMode"] != true {
		t.Errorf("expected editor config preserved, got %v", result["editor"])
	}

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 2 || allowed[0] != "Bash" || allowed[1] != "Read" {
		t.Errorf("unexpected permissions.allow: %v", allowed)
	}

	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 1 || denied[0] != "Edit" {
		t.Errorf("unexpected permissions.deny: %v", denied)
	}
}

func TestCursorProvider_RemovePermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "cli-config.json")
	mcpPath := filepath.Join(dir, "mcp.json")

	writeTestJSON(t, settingsPath, map[string]any{
		"version": 1,
		"permissions": map[string]any{
			"allow": []any{"Bash", "Read"},
			"deny":  []any{"Edit"},
		},
	})

	p := NewCursorProvider(settingsPath, mcpPath)

	prev := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.RemovePermissions(prev); err != nil {
		t.Fatalf("RemovePermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	if result["version"] != float64(1) {
		t.Errorf("expected version preserved, got %v", result["version"])
	}

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 0 {
		t.Errorf("expected empty permissions.allow after remove, got: %v", allowed)
	}

	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 0 {
		t.Errorf("expected empty permissions.deny after remove, got: %v", denied)
	}
}

func TestCursorProvider_RegisterMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	mcpPath := filepath.Join(dir, "mcp.json")

	p := NewCursorProvider(settingsPath, mcpPath)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"ROOT": "/tmp"},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	result := readTestJSON(t, mcpPath)

	mcpServers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers is not a map, got %T: %v", result["mcpServers"], result["mcpServers"])
	}

	entry, ok := mcpServers["filesystem"].(map[string]any)
	if !ok {
		t.Fatalf("filesystem entry is not a map, got %T: %v", mcpServers["filesystem"], mcpServers["filesystem"])
	}

	if entry["command"] != "npx" {
		t.Errorf("expected command 'npx', got %v", entry["command"])
	}

	args, ok := entry["args"].([]any)
	if !ok {
		t.Fatalf("args is not a slice, got %T: %v", entry["args"], entry["args"])
	}
	if len(args) != 2 || args[0] != "-y" || args[1] != "@modelcontextprotocol/server-filesystem" {
		t.Errorf("unexpected args: %v", args)
	}

	env, ok := entry["env"].(map[string]any)
	if !ok {
		t.Fatalf("env is not a map, got %T: %v", entry["env"], entry["env"])
	}
	if env["ROOT"] != "/tmp" {
		t.Errorf("expected env ROOT=/tmp, got %v", env["ROOT"])
	}
}

func TestCursorProvider_RemoveMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	mcpPath := filepath.Join(dir, "mcp.json")

	// Pre-existing MCPs.
	writeTestJSON(t, mcpPath, map[string]any{
		"mcpServers": map[string]any{
			"filesystem": map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@modelcontextprotocol/server-filesystem"},
				"env":     map[string]any{},
			},
			"other-server": map[string]any{
				"command": "other-cmd",
				"args":    []any{},
				"env":     map[string]any{},
			},
		},
	})

	p := NewCursorProvider(settingsPath, mcpPath)

	if err := p.RemoveMCP("filesystem"); err != nil {
		t.Fatalf("RemoveMCP returned error: %v", err)
	}

	result := readTestJSON(t, mcpPath)

	mcpServers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers is not a map, got %T: %v", result["mcpServers"], result["mcpServers"])
	}

	// "filesystem" must be removed.
	if _, exists := mcpServers["filesystem"]; exists {
		t.Errorf("expected filesystem to be removed from mcpServers")
	}

	// "other-server" must remain.
	if _, exists := mcpServers["other-server"]; !exists {
		t.Errorf("expected other-server to remain in mcpServers")
	}
}

func TestCursorProvider_Name(t *testing.T) {
	p := NewCursorProvider("/some/settings.json", "/some/mcp.json")
	if p.Name() != "cursor" {
		t.Errorf("expected 'cursor', got %q", p.Name())
	}
}

func TestCursorProvider_SettingsFilePath(t *testing.T) {
	p := NewCursorProvider("/some/settings.json", "/some/mcp.json")
	if p.SettingsFilePath() != "/some/settings.json" {
		t.Errorf("unexpected settings path: %q", p.SettingsFilePath())
	}
}

func TestCursorProvider_RegisterMCP_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	mcpPath := filepath.Join(dir, "mcp.json")

	// Pre-existing mcp.json with an existing server.
	writeTestJSON(t, mcpPath, map[string]any{
		"mcpServers": map[string]any{
			"existing-server": map[string]any{
				"command": "existing-cmd",
				"args":    []any{},
				"env":     map[string]any{},
			},
		},
	})

	p := NewCursorProvider(settingsPath, mcpPath)

	mcp := ResolvedMCP{
		Name:    "new-server",
		Command: "new-cmd",
		Args:    []string{"--flag"},
		Env:     map[string]string{"KEY": "val"},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	result := readTestJSON(t, mcpPath)

	mcpServers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers is not a map, got %T: %v", result["mcpServers"], result["mcpServers"])
	}

	// existing-server must still be present.
	if _, exists := mcpServers["existing-server"]; !exists {
		t.Error("expected existing-server to be preserved in mcpServers")
	}

	// new-server must be present with correct data.
	newEntry, ok := mcpServers["new-server"].(map[string]any)
	if !ok {
		t.Fatalf("new-server entry is not a map, got %T: %v", mcpServers["new-server"], mcpServers["new-server"])
	}
	if newEntry["command"] != "new-cmd" {
		t.Errorf("expected command 'new-cmd', got %v", newEntry["command"])
	}

	args, ok := newEntry["args"].([]any)
	if !ok {
		t.Fatalf("args is not a slice, got %T: %v", newEntry["args"], newEntry["args"])
	}
	if len(args) != 1 || args[0] != "--flag" {
		t.Errorf("unexpected args: %v", args)
	}

	env, ok := newEntry["env"].(map[string]any)
	if !ok {
		t.Fatalf("env is not a map, got %T: %v", newEntry["env"], newEntry["env"])
	}
	if env["KEY"] != "val" {
		t.Errorf("expected env KEY=val, got %v", env["KEY"])
	}
}

func TestCursorProvider_ApplyPermissions_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "cli-config.json")
	mcpPath := filepath.Join(dir, "mcp.json")
	// File does not exist.

	p := NewCursorProvider(settingsPath, mcpPath)

	perms := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.ApplyPermissions(perms); err != nil {
		t.Fatalf("ApplyPermissions returned error: %v", err)
	}

	result := readTestJSON(t, settingsPath)

	// version must be set to 1 for new files.
	if result["version"] != float64(1) {
		t.Errorf("expected version=1, got %v", result["version"])
	}

	permissions, ok := result["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions is not a map, got %T: %v", result["permissions"], result["permissions"])
	}

	allowed, ok := permissions["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow is not a slice, got %T: %v", permissions["allow"], permissions["allow"])
	}
	if len(allowed) != 2 || allowed[0] != "Bash" || allowed[1] != "Read" {
		t.Errorf("unexpected permissions.allow: %v", allowed)
	}

	denied, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions.deny is not a slice, got %T: %v", permissions["deny"], permissions["deny"])
	}
	if len(denied) != 1 || denied[0] != "Edit" {
		t.Errorf("unexpected permissions.deny: %v", denied)
	}
}
