#!/bin/bash
# e2e/suites/02-apply-basic.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Apply work profile
facet_apply work
echo "  apply exited cleanly"

# State file written
assert_file_exists "$HOME/.facet/.state.json"
assert_json_field "$HOME/.facet/.state.json" '.profile' 'work'
echo "  state file written with correct profile"

# Packages should have run (check mock log)
if [ "${FACET_E2E_REAL_PACKAGES:-}" != "1" ]; then
    assert_file_exists "$HOME/.mock-packages"
    assert_file_contains "$HOME/.mock-packages" "ripgrep"
    assert_file_contains "$HOME/.mock-packages" "docker"
    echo "  packages install commands executed"
fi
