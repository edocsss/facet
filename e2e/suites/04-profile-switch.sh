#!/bin/bash
# e2e/suites/04-profile-switch.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply work first
facet_apply work
assert_file_contains "$HOME/.gitconfig" "sarah@acme.com"
assert_file_exists "$HOME/.npmrc"
echo "  work profile applied"

# Switch to personal
facet_apply personal

# .gitconfig should now have personal email
assert_file_contains "$HOME/.gitconfig" "sarah@hey.com"
assert_file_not_contains "$HOME/.gitconfig" "acme"
assert_file_not_contains "$HOME/.gitconfig" "gpgsign"
echo "  .gitconfig switched to personal vars"

# .npmrc should be gone (personal doesn't define it — orphan cleanup)
assert_file_not_exists "$HOME/.npmrc"
echo "  .npmrc removed (orphan cleanup)"

# .zshrc still present (both profiles have it via base)
assert_symlink "$HOME/.zshrc"
echo "  .zshrc preserved from base"

# State file updated
assert_json_field "$HOME/.facet/.state.json" '.profile' 'personal'
echo "  state file shows personal"
