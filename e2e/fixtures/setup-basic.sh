#!/bin/bash
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

pre_apply:
  - name: create-pre-marker
    run: touch "$HOME/.facet-base-pre-ran"

post_apply:
  - name: create-post-marker
    run: touch "$HOME/.facet-base-post-ran"
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

pre_apply:
  - name: create-work-pre-marker
    run: touch "$HOME/.facet-work-pre-ran"

post_apply:
  - name: create-work-post-marker
    run: touch "$HOME/.facet-work-post-ran"
  - name: write-resolved-var
    run: echo "${facet:git.email}" > "$HOME/.facet-post-email"
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
