#!/bin/bash
# e2e/suites/08-status.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Status with no apply yet
STATUS_OUTPUT=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" status 2>&1 || true)
echo "$STATUS_OUTPUT" | grep -qi "no profile\|facet apply"
echo "  status shows hint when no profile applied"

# Apply and check status
setup_basic
facet_apply work

STATUS_OUTPUT=$(facet_status)
echo "$STATUS_OUTPUT" | grep -q "work"
echo "  status shows active profile"

echo "$STATUS_OUTPUT" | grep -q ".gitconfig"
echo "  status lists deployed configs"

echo "$STATUS_OUTPUT" | grep -q ".zshrc"
echo "  status lists all configs"

# Break a symlink source and check validity
rm "$HOME/dotfiles/configs/.zshrc"
STATUS_OUTPUT=$(facet_status 2>&1)
echo "$STATUS_OUTPUT" | grep -qi "broken\|missing\|invalid\|✗"
echo "  status detects broken symlink"

# Restore
echo 'export EDITOR=nvim' > "$HOME/dotfiles/configs/.zshrc"
