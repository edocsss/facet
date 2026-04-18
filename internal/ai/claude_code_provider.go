package ai

import (
	"fmt"
	"sort"
	"strings"

	"facet/internal/common/jsonutil"
)

// ClaudeCodeProvider implements AgentProvider for Claude Code.
type ClaudeCodeProvider struct {
	settingsPath string
	runner       CommandRunner
}

// NewClaudeCodeProvider constructs a ClaudeCodeProvider.
func NewClaudeCodeProvider(settingsPath string, runner CommandRunner) *ClaudeCodeProvider {
	return &ClaudeCodeProvider{settingsPath: settingsPath, runner: runner}
}

// Name returns the agent identifier.
func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

// SettingsFilePath returns the path to the Claude Code settings file.
func (p *ClaudeCodeProvider) SettingsFilePath() string { return p.settingsPath }

// ApplyPermissions writes permissions.allow and permissions.deny into the
// settings file, preserving all other existing keys.
func (p *ClaudeCodeProvider) ApplyPermissions(perms ResolvedPermissions) error {
	settings, err := jsonutil.ReadFile(p.settingsPath)
	if err != nil {
		return fmt.Errorf("claude-code ApplyPermissions: %w", err)
	}
	permissions := jsonutil.GetOrCreateObject(settings, "permissions")

	// Convert []string to []any for JSON round-trip compatibility.
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
		return fmt.Errorf("claude-code ApplyPermissions: %w", err)
	}
	return nil
}

// RemovePermissions clears permissions.allow and permissions.deny in the
// settings file, preserving all other existing keys.
func (p *ClaudeCodeProvider) RemovePermissions(_ ResolvedPermissions) error {
	settings, err := jsonutil.ReadFile(p.settingsPath)
	if err != nil {
		return fmt.Errorf("claude-code RemovePermissions: %w", err)
	}
	permissions := jsonutil.GetOrCreateObject(settings, "permissions")

	permissions["allow"] = []any{}
	permissions["deny"] = []any{}
	settings["permissions"] = permissions

	if err := jsonutil.WriteFile(p.settingsPath, settings); err != nil {
		return fmt.Errorf("claude-code RemovePermissions: %w", err)
	}
	return nil
}

// RegisterMCP registers an MCP server with Claude Code via the CLI.
// If the server already exists, it is removed first and re-added so
// the operation is idempotent (the CLI rejects duplicate names).
// MCPs are always registered at user scope so they are available across
// every project on this machine. The command format is:
//
//	claude mcp add <name> --scope user [-e K=V ...] -- <command> <args...>
func (p *ClaudeCodeProvider) RegisterMCP(mcp ResolvedMCP) error {
	parts := p.buildMCPAddArgs(mcp)

	err := p.runner.Run(parts[0], parts[1:]...)
	if err == nil {
		return nil
	}

	// If the MCP already exists, remove it and retry the add.
	if !strings.Contains(err.Error(), "already exists") {
		return err
	}

	if removeErr := p.runner.Run("claude", "mcp", "remove", mcp.Name, "--scope", "user"); removeErr != nil {
		return fmt.Errorf("remove existing MCP %q before re-add: %w", mcp.Name, removeErr)
	}

	return p.runner.Run(parts[0], parts[1:]...)
}

// buildMCPAddArgs constructs the argument vector for `claude mcp add`.
func (p *ClaudeCodeProvider) buildMCPAddArgs(mcp ResolvedMCP) []string {
	var parts []string
	parts = append(parts, "claude", "mcp", "add", mcp.Name, "--scope", "user")

	// Env vars — sorted for deterministic output.
	if len(mcp.Env) > 0 {
		keys := make([]string, 0, len(mcp.Env))
		for k := range mcp.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, "-e", fmt.Sprintf("%s=%s", k, mcp.Env[k]))
		}
	}

	parts = append(parts, "--", mcp.Command)
	parts = append(parts, mcp.Args...)

	return parts
}

// RemoveMCP removes an MCP server registration from Claude Code via the CLI.
// Removal targets the user scope to match RegisterMCP.
func (p *ClaudeCodeProvider) RemoveMCP(name string) error {
	return p.runner.Run("claude", "mcp", "remove", name, "--scope", "user")
}
