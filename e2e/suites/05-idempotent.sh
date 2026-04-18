#!/bin/bash
# e2e/suites/05-idempotent.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply twice
facet_apply work
FIRST_GITCONFIG=$(cat "$HOME/.gitconfig")
FIRST_ZSHRC_TARGET=$(readlink "$HOME/.zshrc")

facet_apply work
SECOND_GITCONFIG=$(cat "$HOME/.gitconfig")
SECOND_ZSHRC_TARGET=$(readlink "$HOME/.zshrc")

# Content should be identical
if [ "$FIRST_GITCONFIG" != "$SECOND_GITCONFIG" ]; then
    echo "  ASSERT FAIL: .gitconfig changed on second apply"
    exit 1
fi
echo "  .gitconfig identical after double apply"

if [ "$FIRST_ZSHRC_TARGET" != "$SECOND_ZSHRC_TARGET" ]; then
    echo "  ASSERT FAIL: .zshrc symlink changed on second apply"
    exit 1
fi
echo "  .zshrc symlink identical after double apply"

echo "  double apply is idempotent"
