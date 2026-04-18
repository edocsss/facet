package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexProvider_ApplyPermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "config.toml")
	mcpPath := filepath.Join(dir, "config.toml")
	requireNoError(t, os.WriteFile(settingsPath, []byte("model = \"gpt-5.4\"\n"), 0o644))

	p := NewCodexProvider(settingsPath, mcpPath)

	perms := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.ApplyPermissions(perms); err != nil {
		t.Fatalf("ApplyPermissions returned error: %v", err)
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(content) != "model = \"gpt-5.4\"\n" {
		t.Fatalf("expected Codex permissions apply to leave config unchanged, got:\n%s", string(content))
	}
}

func TestCodexProvider_RemovePermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "config.toml")
	mcpPath := filepath.Join(dir, "config.toml")
	requireNoError(t, os.WriteFile(settingsPath, []byte("model = \"gpt-5.4\"\n"), 0o644))

	p := NewCodexProvider(settingsPath, mcpPath)

	prev := ResolvedPermissions{
		Allow: []string{"Bash", "Read"},
		Deny:  []string{"Edit"},
	}
	if err := p.RemovePermissions(prev); err != nil {
		t.Fatalf("RemovePermissions returned error: %v", err)
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(content) != "model = \"gpt-5.4\"\n" {
		t.Fatalf("expected Codex permissions remove to leave config unchanged, got:\n%s", string(content))
	}
}

func TestCodexProvider_RegisterMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "config.toml")
	mcpPath := filepath.Join(dir, "config.toml")

	p := NewCodexProvider(settingsPath, mcpPath)

	mcp := ResolvedMCP{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"ROOT": "/tmp"},
	}

	if err := p.RegisterMCP(mcp); err != nil {
		t.Fatalf("RegisterMCP returned error: %v", err)
	}

	content, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[mcp_servers.filesystem]") {
		t.Fatalf("expected MCP server table in config, got:\n%s", text)
	}
	if !strings.Contains(text, "command = \"npx\"") {
		t.Fatalf("expected command in config, got:\n%s", text)
	}
	if !strings.Contains(text, "args = [\"-y\", \"@modelcontextprotocol/server-filesystem\"]") {
		t.Fatalf("expected args in config, got:\n%s", text)
	}
	if !strings.Contains(text, "[mcp_servers.filesystem.env]") || !strings.Contains(text, "ROOT = \"/tmp\"") {
		t.Fatalf("expected env table in config, got:\n%s", text)
	}
}

func TestCodexProvider_RemoveMCP(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "config.toml")
	mcpPath := filepath.Join(dir, "config.toml")
	requireNoError(t, os.WriteFile(mcpPath, []byte(
		"[mcp_servers.filesystem]\ncommand = \"npx\"\nargs = [\"-y\", \"@modelcontextprotocol/server-filesystem\"]\n\n"+
			"[mcp_servers.other-server]\ncommand = \"other-cmd\"\nargs = []\n",
	), 0o644))

	p := NewCodexProvider(settingsPath, mcpPath)

	if err := p.RemoveMCP("filesystem"); err != nil {
		t.Fatalf("RemoveMCP returned error: %v", err)
	}

	content, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "[mcp_servers.filesystem]") {
		t.Fatalf("expected filesystem MCP to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "[mcp_servers.other-server]") {
		t.Fatalf("expected other-server MCP to remain, got:\n%s", text)
	}
}

func TestCodexProvider_Name(t *testing.T) {
	p := NewCodexProvider("/some/settings.json", "/some/mcp.json")
	if p.Name() != "codex" {
		t.Errorf("expected 'codex', got %q", p.Name())
	}
}

func TestCodexProvider_SettingsFilePath(t *testing.T) {
	p := NewCodexProvider("/some/settings.json", "/some/mcp.json")
	if p.SettingsFilePath() != "/some/settings.json" {
		t.Errorf("unexpected settings path: %q", p.SettingsFilePath())
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
