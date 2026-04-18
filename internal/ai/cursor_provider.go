package ai

import (
	"fmt"

	"facet/internal/common/jsonutil"
)

// CursorProvider implements AgentProvider for Cursor.
type CursorProvider struct {
	settingsPath string // .cursor/cli-config.json for permissions
	mcpPath      string // .cursor/mcp.json for MCP servers
}

// NewCursorProvider constructs a CursorProvider.
func NewCursorProvider(settingsPath, mcpPath string) *CursorProvider {
	return &CursorProvider{settingsPath: settingsPath, mcpPath: mcpPath}
}

// Name returns the agent identifier.
func (p *CursorProvider) Name() string { return "cursor" }

// SettingsFilePath returns the path to the Cursor settings file.
func (p *CursorProvider) SettingsFilePath() string { return p.settingsPath }

// ApplyPermissions writes permissions.allow and permissions.deny into the
// settings file, preserving all other existing keys.
func (p *CursorProvider) ApplyPermissions(perms ResolvedPermissions) error {
	settings, err := jsonutil.ReadFile(p.settingsPath)
	if err != nil {
		return fmt.Errorf("cursor ApplyPermissions: %w", err)
	}
	if _, ok := settings["version"]; !ok {
		settings["version"] = 1
	}
	permissions := jsonutil.GetOrCreateObject(settings, "permissions")

	allowedTools := make([]any, len(perms.Allow))
	for i, v := range perms.Allow {
		allowedTools[i] = v
	}
	deniedTools := make([]any, len(perms.Deny))
	for i, v := range perms.Deny {
		deniedTools[i] = v
	}

	permissions["allow"] = allowedTools
	permissions["deny"] = deniedTools
	settings["permissions"] = permissions

	if err := jsonutil.WriteFile(p.settingsPath, settings); err != nil {
		return fmt.Errorf("cursor ApplyPermissions: %w", err)
	}
	return nil
}

// RemovePermissions clears permissions.allow and permissions.deny in the
// settings file, preserving all other existing keys.
func (p *CursorProvider) RemovePermissions(_ ResolvedPermissions) error {
	settings, err := jsonutil.ReadFile(p.settingsPath)
	if err != nil {
		return fmt.Errorf("cursor RemovePermissions: %w", err)
	}
	if _, ok := settings["version"]; !ok {
		settings["version"] = 1
	}
	permissions := jsonutil.GetOrCreateObject(settings, "permissions")

	permissions["allow"] = []any{}
	permissions["deny"] = []any{}
	settings["permissions"] = permissions

	if err := jsonutil.WriteFile(p.settingsPath, settings); err != nil {
		return fmt.Errorf("cursor RemovePermissions: %w", err)
	}
	return nil
}

// RegisterMCP registers an MCP server by writing into .cursor/mcp.json.
func (p *CursorProvider) RegisterMCP(mcp ResolvedMCP) error {
	data, err := jsonutil.ReadFile(p.mcpPath)
	if err != nil {
		return fmt.Errorf("cursor RegisterMCP: %w", err)
	}

	mcpServers, ok := data["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}

	args := make([]any, len(mcp.Args))
	for i, v := range mcp.Args {
		args[i] = v
	}

	env := make(map[string]any, len(mcp.Env))
	for k, v := range mcp.Env {
		env[k] = v
	}

	mcpServers[mcp.Name] = map[string]any{
		"command": mcp.Command,
		"args":    args,
		"env":     env,
	}
	data["mcpServers"] = mcpServers

	if err := jsonutil.WriteFile(p.mcpPath, data); err != nil {
		return fmt.Errorf("cursor RegisterMCP: %w", err)
	}
	return nil
}

// RemoveMCP removes an MCP server entry from .cursor/mcp.json.
func (p *CursorProvider) RemoveMCP(name string) error {
	data, err := jsonutil.ReadFile(p.mcpPath)
	if err != nil {
		return fmt.Errorf("cursor RemoveMCP: %w", err)
	}

	mcpServers, ok := data["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}

	delete(mcpServers, name)
	data["mcpServers"] = mcpServers

	if err := jsonutil.WriteFile(p.mcpPath, data); err != nil {
		return fmt.Errorf("cursor RemoveMCP: %w", err)
	}
	return nil
}
