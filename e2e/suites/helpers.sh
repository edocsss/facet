#!/bin/bash
#
# Assertion functions sourced by each test suite.
# All paths use $HOME which the harness points to the sandbox.

assert_file_exists() {
    if [ ! -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to exist"
        exit 1
    fi
}

assert_file_not_exists() {
    if [ -e "$1" ]; then
        echo "  ASSERT FAIL: expected file $1 to NOT exist"
        exit 1
    fi
}

assert_symlink() {
    if [ ! -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to be a symlink"
        exit 1
    fi
}

assert_not_symlink() {
    if [ -L "$1" ]; then
        echo "  ASSERT FAIL: expected $1 to NOT be a symlink (got symlink)"
        exit 1
    fi
}

assert_file_contains() {
    if ! grep -F -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to contain '$2'"
        echo "  Actual content:"
        head -20 "$1" 2>/dev/null || echo "  (file not readable)"
        exit 1
    fi
}

assert_file_not_contains() {
    if grep -F -q "$2" "$1" 2>/dev/null; then
        echo "  ASSERT FAIL: expected $1 to NOT contain '$2'"
        exit 1
    fi
}

assert_json_field() {
    local file="$1" path="$2" expected="$3"
    local actual
    actual=$(jq -r "$path" "$file" 2>/dev/null)
    if [ "$actual" != "$expected" ]; then
        echo "  ASSERT FAIL: $file $path"
        echo "  Expected: $expected"
        echo "  Actual:   $actual"
        exit 1
    fi
}

assert_exit_code() {
    local expected="$1"
    shift
    local actual
    set +e
    "$@" >/dev/null 2>&1
    actual=$?
    set -e
    if [ "$actual" -ne "$expected" ]; then
        echo "  ASSERT FAIL: expected exit code $expected, got $actual"
        echo "  Command: $*"
        exit 1
    fi
}

# Shorthand for running facet with test dirs
facet_apply() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply "$@"
}

facet_status() {
    facet -c "$HOME/dotfiles" -s "$HOME/.facet" status "$@"
}

facet_scaffold() {
    # scaffold operates on cwd, so cd into the target
    local target="${1:-$HOME/dotfiles}"
    mkdir -p "$target"
    (cd "$target" && facet -s "$HOME/.facet" scaffold)
}

# Helper to source the fixture setup
setup_basic() {
    bash "$FIXTURE_DIR/setup-basic.sh"
}
