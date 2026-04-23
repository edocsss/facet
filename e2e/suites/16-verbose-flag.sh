#!/bin/bash
# e2e/suites/16-verbose-flag.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose work 2>&1)
echo "$output" | grep -q "Loading profile" || { echo "  FAIL: missing 'Loading profile' in verbose output"; exit 1; }
echo "$output" | grep -q "Resolving extends" || { echo "  FAIL: missing 'Resolving extends' in verbose output"; exit 1; }
echo "$output" | grep -q "Merging layers" || { echo "  FAIL: missing 'Merging layers' in verbose output"; exit 1; }
echo "$output" | grep -q "Deploying configs" || { echo "  FAIL: missing 'Deploying configs' in verbose output"; exit 1; }
echo "$output" | grep -q "Installing packages" || { echo "  FAIL: missing 'Installing packages' in verbose output"; exit 1; }
echo "$output" | grep -q "Running pre_apply scripts" || { echo "  FAIL: missing 'Running pre_apply scripts' in verbose output"; exit 1; }
echo "$output" | grep -q "Running post_apply scripts" || { echo "  FAIL: missing 'Running post_apply scripts' in verbose output"; exit 1; }
echo "  --verbose shows stage progress lines"

echo "$output" | grep -q "→ ripgrep" || { echo "  FAIL: missing package item progress for 'ripgrep'"; exit 1; }
echo "$output" | grep -q "→ create-pre-marker" || { echo "  FAIL: missing pre_apply item progress for 'create-pre-marker'"; exit 1; }
echo "  --verbose shows item-level detail"

output_short=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply -v work 2>&1)
echo "$output_short" | grep -q "Loading profile" || { echo "  FAIL: -v short form missing 'Loading profile'"; exit 1; }
echo "  -v short form works"

output_quiet=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work 2>&1)
echo "$output_quiet" | grep -q "Applied profile: work" || { echo "  FAIL: non-verbose apply did not complete normally"; exit 1; }
if echo "$output_quiet" | grep -q "Loading profile"; then
	echo "  FAIL: 'Loading profile' appeared without --verbose flag"
	exit 1
fi
if echo "$output_quiet" | grep -q "Deploying configs"; then
	echo "  FAIL: 'Deploying configs' appeared without --verbose flag"
	exit 1
fi
echo "  without --verbose, no stage progress lines shown"

output_force=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose --force work 2>&1)
echo "$output_force" | grep -q "Unapplying previous state" || { echo "  FAIL: missing 'Unapplying previous state' in verbose --force output"; exit 1; }
echo "  --verbose --force shows unapply progress"
