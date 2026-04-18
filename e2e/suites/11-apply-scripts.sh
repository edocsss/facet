#!/bin/bash
# e2e/suites/11-apply-scripts.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Test: pre_apply and post_apply scripts run during apply
facet_apply work

assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-work-pre-ran"
assert_file_exists "$HOME/.facet-base-post-ran"
assert_file_exists "$HOME/.facet-work-post-ran"
echo "  pre_apply and post_apply scripts run during apply"

# Test: variable resolution in post_apply scripts
assert_file_exists "$HOME/.facet-post-email"
assert_file_contains "$HOME/.facet-post-email" "sarah@acme.com"
echo "  scripts resolve variables in run strings"

# Test: scripts are re-runnable (apply again)
facet_apply work --force
assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-base-post-ran"
echo "  scripts are re-runnable on re-apply"

# Test: pre_apply script failure halts apply
cat > "$HOME/dotfiles/profiles/failing.yaml" << 'YAML'
extends: base

pre_apply:
  - name: will-succeed
    run: touch "$HOME/.facet-fail-first"
  - name: will-fail
    run: exit 1
  - name: should-be-skipped
    run: touch "$HOME/.facet-fail-skipped"
YAML

assert_exit_code 1 bash -c "facet -c $HOME/dotfiles -s $HOME/.facet apply failing"
assert_file_exists "$HOME/.facet-fail-first"
assert_file_not_exists "$HOME/.facet-fail-skipped"
echo "  pre_apply fails fast on non-zero exit, skips remaining"

# Test: --stages filters which stages run
setup_basic

# Remove marker files from previous test
rm -f "$HOME/.facet-base-pre-ran" "$HOME/.facet-work-pre-ran"
rm -f "$HOME/.facet-base-post-ran" "$HOME/.facet-work-post-ran"

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work --stages packages
assert_file_not_exists "$HOME/.facet-base-pre-ran"
assert_file_not_exists "$HOME/.facet-base-post-ran"
echo "  --stages packages skips scripts"

# Test: --stages runs only selected stages
rm -f "$HOME/.facet-base-pre-ran" "$HOME/.facet-work-pre-ran"

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work --force --stages pre_apply
assert_file_exists "$HOME/.facet-base-pre-ran"
assert_file_exists "$HOME/.facet-work-pre-ran"
assert_file_not_exists "$HOME/.facet-base-post-ran"
echo "  --stages pre_apply runs only pre_apply scripts"

# Test: apply with no scripts succeeds
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen
YAML

cat > "$HOME/dotfiles/profiles/noscripts.yaml" << 'YAML'
extends: base
YAML

facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply noscripts
echo "  apply succeeds when no scripts are defined"

# Test: scripts can read from stdin (interactive support)
setup_basic

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen

post_apply:
  - name: read-stdin
    run: read -r line && echo "$line" > "$HOME/.facet-stdin-result"
YAML

cat > "$HOME/dotfiles/profiles/interactive.yaml" << 'YAML'
extends: base
YAML

echo "hello-from-stdin" | facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply interactive
assert_file_exists "$HOME/.facet-stdin-result"
assert_file_contains "$HOME/.facet-stdin-result" "hello-from-stdin"
echo "  scripts can read from stdin (interactive support)"

# Test: script output streams to stdout
cat > "$HOME/dotfiles/base.yaml" << 'YAML'
vars:
  git_name: Sarah Chen

pre_apply:
  - name: echo-stdout
    run: echo "visible-output"
YAML

output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply interactive 2>&1)
echo "$output" | grep -q "visible-output"
echo "  script output streams to terminal"
