#!/bin/bash
# e2e/suites/13-ai-skills-commands.sh — AI skills check/update commands
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Test 1: facet ai skills check — runs npx skills check
facet -c "$HOME/dotfiles" -s "$HOME/.facet" ai skills check
assert_file_contains "$HOME/.mock-ai" "npx skills check"
echo "  facet ai skills check invoked npx skills check"

# Test 2: facet ai skills update — runs npx skills update
: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" ai skills update
assert_file_contains "$HOME/.mock-ai" "npx skills update"
echo "  facet ai skills update invoked npx skills update"

# Test 3: help output for parent commands
facet ai --help > "$HOME/.help-output" 2>&1
assert_file_contains "$HOME/.help-output" "skills"
echo "  facet ai --help shows skills subcommand"

facet ai skills --help > "$HOME/.help-output" 2>&1
assert_file_contains "$HOME/.help-output" "check"
assert_file_contains "$HOME/.help-output" "update"
echo "  facet ai skills --help shows check and update subcommands"

# Test 4: error when npx is not available
# Replace mock npx with one that always fails (simulates npx not installed).
cat > "$HOME/mock-bin/npx" << 'FAILEOF'
#!/bin/bash
echo "command not found: npx" >&2
exit 127
FAILEOF
chmod +x "$HOME/mock-bin/npx"
set +e
facet -c "$HOME/dotfiles" -s "$HOME/.facet" ai skills check > "$HOME/.error-output" 2>&1
exit_code=$?
set -e
if [ "$exit_code" -eq 0 ]; then
    echo "  ASSERT FAIL: expected non-zero exit when npx missing"
    exit 1
fi
echo "  facet ai skills check fails cleanly when npx is missing"
