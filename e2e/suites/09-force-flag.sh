#!/bin/bash
# e2e/suites/09-force-flag.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Create a regular file at a target path (not managed by facet)
mkdir -p "$HOME"
echo "user's manual file" > "$HOME/.zshrc"

# Normal apply should fail/prompt (non-interactive → error)
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work
echo "  conflicting regular file blocks normal apply"

# --force should replace it
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --force work
assert_symlink "$HOME/.zshrc"
echo "  --force replaces conflicting file"

# --force on same profile = full unapply + reapply (clean slate)
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --force work
assert_symlink "$HOME/.zshrc"
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
echo "  --force on same profile works (clean slate)"

# --skip-failure: config deploy failures warn instead of rollback
cat > "$HOME/dotfiles/profiles/badconfig.yaml" << 'YAML'
extends: base
configs:
  ~/.zshrc: configs/.zshrc
  ~/.missing-source: configs/this_file_does_not_exist
YAML
# Without --skip-failure: should fail and rollback
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badconfig

# With --skip-failure: should succeed with warning, deploying what it can
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --skip-failure badconfig
assert_symlink "$HOME/.zshrc"
echo "  --skip-failure warns and continues on config deploy failure"
