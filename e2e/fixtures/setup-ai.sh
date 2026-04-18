#!/bin/bash
#
# Extends basic setup with AI configuration in profiles.
# Must be called AFTER setup-basic.sh.
set -euo pipefail

CONFIG_DIR="$HOME/dotfiles"

# ── Overwrite base.yaml with AI section added ──
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

ai:
  agents: [claude-code, cursor]
  permissions:
    claude-code:
      allow: [Read, Edit, Bash]
      deny: []
    cursor:
      allow: ["Read(**)", "Write(**)"]
      deny: []
  skills:
    - source: "@vercel-labs/agent-skills"
      skills: [frontend-design]
  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
    - name: github
      command: npx
      args: ["@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${facet:secret_key}"
      agents: [claude-code]
YAML

# ── Create minimal profile for orphan testing ──
cat > "$CONFIG_DIR/profiles/minimal.yaml" << 'YAML'
extends: base

ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
YAML

mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.cursor"
mkdir -p "$HOME/.codex"

echo "[setup-ai] AI configuration added to base.yaml"
