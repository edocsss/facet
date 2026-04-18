#!/bin/bash
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

mkdir -p "$HOME/dotfiles/profiles" "$HOME/dotfiles/configs/local" "$HOME/.facet"
cat > "$HOME/dotfiles/facet.yaml" <<'YAML'
min_version: "0.1.0"
YAML
cat > "$HOME/.facet/.local.yaml" <<'YAML'
vars:
  git_email: user@example.com
YAML

REMOTE_REPO="$HOME/remote-base"
mkdir -p "$REMOTE_REPO/configs/remote"
git init -b main "$REMOTE_REPO" >/dev/null 2>&1
git -C "$REMOTE_REPO" config user.name "Facet E2E"
git -C "$REMOTE_REPO" config user.email "facet-e2e@example.com"

cat > "$REMOTE_REPO/base.yaml" <<'YAML'
vars:
  remote_name: tagged
configs:
  ~/.remote-base-config: configs/remote/base.conf
pre_apply:
  - name: remote-pre
    run: printf '%s' "$PWD" > "$HOME/remote-pre-dir.txt"
YAML
cat > "$REMOTE_REPO/configs/remote/base.conf" <<'EOF'
tagged-file
EOF
git -C "$REMOTE_REPO" add .
git -C "$REMOTE_REPO" commit -m "tagged" >/dev/null 2>&1
git -C "$REMOTE_REPO" tag v1.0.0

cat > "$REMOTE_REPO/base.yaml" <<'YAML'
vars:
  remote_name: commit
configs:
  ~/.remote-base-config: configs/remote/base.conf
pre_apply:
  - name: remote-pre
    run: printf '%s' "$PWD" > "$HOME/remote-pre-dir.txt"
YAML
cat > "$REMOTE_REPO/configs/remote/base.conf" <<'EOF'
commit-file
EOF
git -C "$REMOTE_REPO" add .
git -C "$REMOTE_REPO" commit -m "commit" >/dev/null 2>&1
REMOTE_COMMIT=$(git -C "$REMOTE_REPO" rev-parse HEAD)

git -C "$REMOTE_REPO" checkout -b feature >/dev/null 2>&1
cat > "$REMOTE_REPO/base.yaml" <<'YAML'
vars:
  remote_name: feature
configs:
  ~/.remote-base-config: configs/remote/base.conf
pre_apply:
  - name: remote-pre
    run: printf '%s' "$PWD" > "$HOME/remote-pre-dir.txt"
YAML
cat > "$REMOTE_REPO/configs/remote/base.conf" <<'EOF'
feature-file
EOF
git -C "$REMOTE_REPO" add .
git -C "$REMOTE_REPO" commit -m "feature" >/dev/null 2>&1

git -C "$REMOTE_REPO" checkout main >/dev/null 2>&1
cat > "$REMOTE_REPO/base.yaml" <<'YAML'
vars:
  remote_name: default
configs:
  ~/.remote-base-config: configs/remote/base.conf
pre_apply:
  - name: remote-pre
    run: printf '%s' "$PWD" > "$HOME/remote-pre-dir.txt"
YAML
cat > "$REMOTE_REPO/configs/remote/base.conf" <<'EOF'
default-file
EOF
git -C "$REMOTE_REPO" add .
git -C "$REMOTE_REPO" commit -m "default" >/dev/null 2>&1

cat > "$HOME/dotfiles/profiles/work.yaml" <<YAML
extends: file://$REMOTE_REPO@main
configs:
  ~/.local-override: configs/local/override.conf
post_apply:
  - name: local-post
    run: printf '%s' "\$PWD" > "\$HOME/local-post-dir.txt"
YAML
cat > "$HOME/dotfiles/configs/local/override.conf" <<'EOF'
local-file
EOF

facet_apply work
DOTFILES_DIR="$(cd "$HOME/dotfiles" && pwd)"
assert_file_exists "$HOME/.remote-base-config"
assert_not_symlink "$HOME/.remote-base-config"
assert_file_contains "$HOME/.remote-base-config" "default-file"
assert_symlink "$HOME/.local-override"
assert_file_contains "$HOME/remote-pre-dir.txt" "facet-extends-"
assert_file_contains "$HOME/local-post-dir.txt" "$DOTFILES_DIR"
assert_json_field "$HOME/.facet/.state.json" '.configs[] | select(.source == "configs/remote/base.conf") | .strategy' "copy"
assert_json_field "$HOME/.facet/.state.json" '.configs[] | select(.source == "configs/local/override.conf") | .strategy' "symlink"
status_output=$(facet_status 2>&1)
echo "$status_output" | grep -q ".remote-base-config"
if echo "$status_output" | grep -q "broken symlink"; then
    echo "  ASSERT FAIL: remote copied config should not report a broken source"
    exit 1
fi
echo "  remote git base materializes configs, keeps local overrides symlinked, and status stays valid"

cat > "$HOME/dotfiles/profiles/default-branch.yaml" <<YAML
extends: file://$REMOTE_REPO
YAML
facet_apply default-branch
assert_file_contains "$HOME/.remote-base-config" "default-file"
echo "  git extends without @ref uses the default branch"

cat > "$HOME/dotfiles/profiles/feature-branch.yaml" <<YAML
extends: file://$REMOTE_REPO@feature
YAML
facet_apply feature-branch
assert_file_contains "$HOME/.remote-base-config" "feature-file"
echo "  git extends supports branch refs"

cat > "$HOME/dotfiles/profiles/tagged.yaml" <<YAML
extends: file://$REMOTE_REPO@v1.0.0
YAML
facet_apply tagged
assert_file_contains "$HOME/.remote-base-config" "tagged-file"
echo "  git extends supports tag refs"

cat > "$HOME/dotfiles/profiles/commit.yaml" <<YAML
extends: file://$REMOTE_REPO@$REMOTE_COMMIT
YAML
facet_apply commit
assert_file_contains "$HOME/.remote-base-config" "commit-file"
echo "  git extends supports commit refs"

LOCAL_DIR_BASE="$HOME/local-dir-base"
mkdir -p "$LOCAL_DIR_BASE/configs"
cat > "$LOCAL_DIR_BASE/base.yaml" <<'YAML'
configs:
  ~/.local-dir-config: configs/dir.conf
YAML
cat > "$LOCAL_DIR_BASE/configs/dir.conf" <<'EOF'
local-dir-file
EOF
cat > "$HOME/dotfiles/profiles/local-dir.yaml" <<YAML
extends: $LOCAL_DIR_BASE
YAML
facet_apply local-dir
assert_symlink "$HOME/.local-dir-config"
assert_file_contains "$HOME/.local-dir-config" "local-dir-file"
echo "  local directory extends keep local configs symlinked"

mkdir -p "$HOME/shared-file-base/configs"
cat > "$HOME/shared-file-base/base.yaml" <<'YAML'
configs:
  ~/.local-file-config: configs/file.conf
YAML
cat > "$HOME/shared-file-base/configs/file.conf" <<'EOF'
local-file-base
EOF
cat > "$HOME/dotfiles/profiles/local-file.yaml" <<YAML
extends: $HOME/shared-file-base/base.yaml
YAML
facet_apply local-file
assert_symlink "$HOME/.local-file-config"
assert_file_contains "$HOME/.local-file-config" "local-file-base"
echo "  local file extends resolve relative sources from the base file directory"

cat > "$HOME/dotfiles/profiles/bad-ref.yaml" <<YAML
extends: file://$REMOTE_REPO@does-not-exist
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply bad-ref
echo "  invalid git refs fail cleanly"

BROKEN_REPO="$HOME/broken-base"
git init -b main "$BROKEN_REPO" >/dev/null 2>&1
git -C "$BROKEN_REPO" config user.name "Facet E2E"
git -C "$BROKEN_REPO" config user.email "facet-e2e@example.com"
git -C "$BROKEN_REPO" commit --allow-empty -m "empty" >/dev/null 2>&1
cat > "$HOME/dotfiles/profiles/no-base.yaml" <<YAML
extends: file://$BROKEN_REPO
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply no-base
echo "  missing remote base.yaml fails cleanly"

ABSOLUTE_REPO="$HOME/absolute-base"
mkdir -p "$ABSOLUTE_REPO"
git init -b main "$ABSOLUTE_REPO" >/dev/null 2>&1
git -C "$ABSOLUTE_REPO" config user.name "Facet E2E"
git -C "$ABSOLUTE_REPO" config user.email "facet-e2e@example.com"
cat > "$ABSOLUTE_REPO/base.yaml" <<'YAML'
configs:
  ~/.absolute-remote-config: /etc/hosts
YAML
git -C "$ABSOLUTE_REPO" add .
git -C "$ABSOLUTE_REPO" commit -m "absolute" >/dev/null 2>&1
cat > "$HOME/dotfiles/profiles/absolute-source.yaml" <<YAML
extends: file://$ABSOLUTE_REPO
YAML
assert_exit_code 1 facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply absolute-source
echo "  git-based bases reject absolute source paths"
