#!/bin/bash
# e2e/suites/16-verbose-flag.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

output=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose work 2>&1)
echo "$output" | grep -Eq "facet apply work \.\.\. start" || { echo "  FAIL: missing total apply start timing"; exit 1; }
echo "$output" | grep -Eq "Loading profile \.\.\. ok [0-9]+ms|Loading profile \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed 'Loading profile'"; exit 1; }
echo "$output" | grep -Eq "Resolving extends \.\.\. ok [0-9]+ms|Resolving extends \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed 'Resolving extends'"; exit 1; }
echo "$output" | grep -Eq "Merging base and profile \.\.\. ok [0-9]+ms|Merging base and profile \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing timed merge"; exit 1; }
echo "$output" | grep -Eq "Deploying configs \.\.\. start" || { echo "  FAIL: missing config timing start"; exit 1; }
echo "$output" | grep -Eq "Installing packages \.\.\. start" || { echo "  FAIL: missing package timing start"; exit 1; }
echo "$output" | grep -Eq "Running pre_apply scripts \.\.\. start" || { echo "  FAIL: missing pre_apply timing start"; exit 1; }
echo "$output" | grep -Eq "Running post_apply scripts \.\.\. start" || { echo "  FAIL: missing post_apply timing start"; exit 1; }
echo "$output" | grep -Eq "Writing state \.\.\. ok [0-9]+ms|Writing state \.\.\. ok [0-9]+\.[0-9]s" || { echo "  FAIL: missing state write timing"; exit 1; }
echo "$output" | grep -Eq "facet apply work \.\.\. done [0-9]+ms|facet apply work \.\.\. done [0-9]+\.[0-9]s" || { echo "  FAIL: missing total apply done timing"; exit 1; }
echo "  --verbose shows timed stage progress lines"

echo "$output" | grep -Eq -- "-> ripgrep (check|install|skip) \.\.\. (ok|failed|skipped)" || { echo "  FAIL: missing package item timing for 'ripgrep'"; exit 1; }
echo "$output" | grep -Eq -- "-> create-pre-marker \.\.\. ok" || { echo "  FAIL: missing pre_apply item timing for 'create-pre-marker'"; exit 1; }
echo "  --verbose shows timed item-level detail"

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
if echo "$output_quiet" | grep -q "\.\.\. ok [0-9]"; then
	echo "  FAIL: timing appeared without --verbose flag"
	exit 1
fi
echo "  without --verbose, no stage progress lines shown"

output_force=$(facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply --verbose --force work 2>&1)
echo "$output_force" | grep -q "Unapplying previous state" || { echo "  FAIL: missing 'Unapplying previous state' in verbose --force output"; exit 1; }
echo "  --verbose --force shows unapply progress"
