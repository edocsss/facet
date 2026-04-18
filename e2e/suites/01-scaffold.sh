#!/bin/bash
# e2e/suites/01-scaffold.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

# Test: scaffold creates config repo structure
SCAFFOLD_DIR="$HOME/new-repo"
facet_scaffold "$SCAFFOLD_DIR"

assert_file_exists "$SCAFFOLD_DIR/facet.yaml"
assert_file_exists "$SCAFFOLD_DIR/base.yaml"
assert_file_exists "$SCAFFOLD_DIR/profiles"
assert_file_exists "$SCAFFOLD_DIR/configs"
echo "  scaffold creates config repo structure"

# Test: scaffold creates .local.yaml in state dir
assert_file_exists "$HOME/.facet/.local.yaml"
echo "  scaffold creates .local.yaml in state dir"

# Test: scaffold fails if facet.yaml already exists
assert_exit_code 1 bash -c "cd $SCAFFOLD_DIR && facet -s $HOME/.facet scaffold"
echo "  scaffold errors on existing facet.yaml"
