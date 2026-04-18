package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CodexProvider implements AgentProvider for Codex.
type CodexProvider struct {
	settingsPath string
	mcpPath      string
}

// NewCodexProvider constructs a CodexProvider.
func NewCodexProvider(settingsPath, mcpPath string) *CodexProvider {
	return &CodexProvider{settingsPath: settingsPath, mcpPath: mcpPath}
}

// Name returns the agent identifier.
func (p *CodexProvider) Name() string { return "codex" }

// SettingsFilePath returns the path to the Codex settings file.
func (p *CodexProvider) SettingsFilePath() string { return p.settingsPath }

// ApplyPermissions is currently a no-op because facet does not have a supported
// Codex permission-file contract to write yet.
func (p *CodexProvider) ApplyPermissions(perms ResolvedPermissions) error {
	_ = perms
	return nil
}

// RemovePermissions is currently a no-op because facet does not have a
// supported Codex permission-file contract to remove from yet.
func (p *CodexProvider) RemovePermissions(_ ResolvedPermissions) error {
	return nil
}

// RegisterMCP registers an MCP server in ~/.codex/config.toml.
func (p *CodexProvider) RegisterMCP(mcp ResolvedMCP) error {
	content, err := readCodexConfig(p.mcpPath)
	if err != nil {
		return fmt.Errorf("codex RegisterMCP: %w", err)
	}
	content = removeCodexMCPBlock(content, mcp.Name)
	content = appendCodexMCPBlock(content, mcp)
	if err := writeCodexConfig(p.mcpPath, content); err != nil {
		return fmt.Errorf("codex RegisterMCP: %w", err)
	}
	return nil
}

// RemoveMCP removes an MCP server entry from ~/.codex/config.toml.
func (p *CodexProvider) RemoveMCP(name string) error {
	content, err := readCodexConfig(p.mcpPath)
	if err != nil {
		return fmt.Errorf("codex RemoveMCP: %w", err)
	}
	content = removeCodexMCPBlock(content, name)
	if err := writeCodexConfig(p.mcpPath, content); err != nil {
		return fmt.Errorf("codex RemoveMCP: %w", err)
	}
	return nil
}

func readCodexConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func writeCodexConfig(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func appendCodexMCPBlock(content string, mcp ResolvedMCP) string {
	var b strings.Builder
	if trimmed := strings.TrimRight(content, "\n"); trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n\n")
	}

	header := "mcp_servers." + mcp.Name
	b.WriteString("[" + header + "]\n")
	b.WriteString("command = " + strconv.Quote(mcp.Command) + "\n")
	b.WriteString("args = " + tomlStringArray(mcp.Args) + "\n")

	if len(mcp.Env) > 0 {
		keys := make([]string, 0, len(mcp.Env))
		for key := range mcp.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("\n[" + header + ".env]\n")
		for _, key := range keys {
			b.WriteString(key + " = " + strconv.Quote(mcp.Env[key]) + "\n")
		}
	}

	return b.String()
}

func removeCodexMCPBlock(content, name string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	prefix := "[mcp_servers." + name
	keep := make([]string, 0, len(lines))
	skipping := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if trimmed == prefix+"]" || strings.HasPrefix(trimmed, prefix+".") {
				skipping = true
				continue
			}
			if skipping {
				skipping = false
			}
		}
		if skipping {
			continue
		}
		keep = append(keep, line)
	}

	return strings.TrimRight(collapseBlankLines(strings.Join(keep, "\n")), "\n")
}

func collapseBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
		} else {
			blankCount = 0
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func tomlStringArray(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
