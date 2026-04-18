#!/bin/bash
# e2e/suites/06-edge-cases.sh
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

# Missing profile → fatal error
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply nonexistent
echo "  missing profile errors correctly"

# Undefined variable → fatal error
cat > "$HOME/dotfiles/profiles/badvar.yaml" << 'YAML'
extends: base
configs:
  ~/.badfile: configs/.zshrc
packages:
  - name: test
    install: echo ${facet:totally_undefined_var}
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badvar
echo "  undefined variable errors correctly"

# Profile without extends → fatal error
cat > "$HOME/dotfiles/profiles/noextends.yaml" << 'YAML'
vars:
  test: value
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply noextends
echo "  missing extends errors correctly"

# Profile with invalid extends → fatal error
cat > "$HOME/dotfiles/profiles/badextends.yaml" << 'YAML'
extends: something_else
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply badextends
echo "  invalid extends value errors correctly"

# Missing .local.yaml → fatal error
rm "$HOME/.facet/.local.yaml"
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work
echo "  missing .local.yaml errors correctly"

# Empty profile (just extends) inherits base correctly
echo "vars:" > "$HOME/.facet/.local.yaml"  # restore
cat > "$HOME/dotfiles/profiles/minimal.yaml" << 'YAML'
extends: base
YAML
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply minimal
assert_symlink "$HOME/.zshrc"
echo "  empty profile inherits base correctly"
