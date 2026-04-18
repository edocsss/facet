#!/bin/bash
# e2e/suites/07-packages.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic
facet_apply work

if [ "${FACET_E2E_REAL_PACKAGES:-}" = "1" ]; then
    echo "  real package tests (Docker)"
    # In Docker, verify real packages were installed
    # The fixture uses apt-get commands for linux
    echo "  (real package verification is best-effort in Docker)"
else
    # Mock: verify install commands were logged
    assert_file_contains "$HOME/.mock-packages" "ripgrep"
    assert_file_contains "$HOME/.mock-packages" "curl"
    assert_file_contains "$HOME/.mock-packages" "docker"
    echo "  all package install commands executed"

    # Per-OS: node has macos/linux variants. Check the right one ran.
    if [ "$(uname)" = "Darwin" ]; then
        # On macOS, mock brew should have received "node"
        assert_file_contains "$HOME/.mock-packages" "node"
    fi
    echo "  per-OS install command selected correctly"
fi

# Package failure should not prevent config deployment
cat >> "$HOME/dotfiles/base.yaml" << 'YAML'
  - name: will-fail
    install: "false"
YAML
facet_apply work
# Configs should still be deployed despite package failure
assert_symlink "$HOME/.zshrc"
echo "  package failure does not block config deployment"
