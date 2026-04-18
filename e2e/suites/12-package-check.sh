#!/bin/bash
# e2e/suites/12-package-check.sh — tests the optional check command on packages
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

CONFIG_DIR="$HOME/dotfiles"
STATE_DIR="$HOME/.facet"

mkdir -p "$CONFIG_DIR"/{profiles,configs}
mkdir -p "$STATE_DIR"

cat > "$CONFIG_DIR/facet.yaml" << 'YAML'
min_version: "0.1.0"
YAML

# Use "echo" as the install command — it works everywhere (mock and real).
# The check field uses "true" (always passes) and "false" (always fails).
cat > "$CONFIG_DIR/base.yaml" << 'YAML'
packages:
  - name: already-here
    check: "true"
    install: "echo installing already-here"

  - name: not-here
    check: "false"
    install: "echo installing not-here"

  - name: no-check
    install: "echo installing no-check"
YAML

cat > "$CONFIG_DIR/profiles/test.yaml" << 'YAML'
extends: base
YAML

cat > "$STATE_DIR/.local.yaml" << 'YAML'
vars: {}
YAML

# Run apply
facet_apply test

# Verify state.json has correct statuses
assert_json_field "$STATE_DIR/.state.json" '.packages[0].name' "already-here"
assert_json_field "$STATE_DIR/.state.json" '.packages[0].status' "already_installed"
assert_json_field "$STATE_DIR/.state.json" '.packages[1].name' "not-here"
assert_json_field "$STATE_DIR/.state.json" '.packages[1].status' "ok"
assert_json_field "$STATE_DIR/.state.json" '.packages[2].name' "no-check"
assert_json_field "$STATE_DIR/.state.json" '.packages[2].status' "ok"
echo "  state.json records correct statuses"

echo "  check passing → install skipped"
echo "  check failing → install ran"
echo "  no check → install ran"
