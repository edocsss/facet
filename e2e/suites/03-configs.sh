#!/bin/bash
# e2e/suites/03-configs.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
facet_apply work

# .zshrc should be symlinked (no ${facet:} vars)
assert_symlink "$HOME/.zshrc"
assert_file_contains "$HOME/.zshrc" "EDITOR=nvim"
echo "  .zshrc is symlinked"

# .gitconfig should be templated (has ${facet:} vars) — regular file, not symlink
assert_not_symlink "$HOME/.gitconfig"
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
assert_file_contains "$HOME/.gitconfig" "Sarah Chen"
assert_file_contains "$HOME/.gitconfig" "cursor --wait"
assert_file_not_contains "$HOME/.gitconfig" '${facet:'
echo "  .gitconfig templated with resolved vars"

# .npmrc should be symlinked
assert_symlink "$HOME/.npmrc"
assert_file_contains "$HOME/.npmrc" "acme-corp.com"
echo "  .npmrc symlinked"

# Parent dirs created automatically
assert_file_exists "$HOME/.config/starship.toml"
assert_symlink "$HOME/.config/starship.toml"
echo "  parent directories created (mkdir -p)"
