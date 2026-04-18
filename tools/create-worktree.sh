#!/bin/bash

set -euo pipefail

# create-worktree.sh - Create a new git worktree for a feature branch
#
# Usage: create-worktree.sh <branch-name>
# Example: create-worktree.sh feature/auth-system
#          Creates worktree at .worktrees/feature-auth-system

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <branch-name>"
    echo "Example: $0 feature/auth-system"
    exit 1
fi

BRANCH_NAME="$1"
REPO_ROOT="$(git rev-parse --show-toplevel)"
WORKTREES_DIR="$REPO_ROOT/.worktrees"

# Normalize branch name: convert slashes to dashes
NORMALIZED_NAME="${BRANCH_NAME//\//-}"
WORKTREE_PATH="$WORKTREES_DIR/$NORMALIZED_NAME"

# Validate branch name format (basic check)
if [[ ! "$BRANCH_NAME" =~ ^[a-zA-Z0-9/_-]+$ ]]; then
    echo "Error: Invalid branch name format. Use alphanumeric characters, slashes, underscores, or dashes."
    exit 1
fi

# Ensure worktrees directory exists
mkdir -p "$WORKTREES_DIR"

# Check if branch already exists locally
if git show-ref --verify --quiet "refs/heads/$BRANCH_NAME"; then
    echo "Error: Branch '$BRANCH_NAME' already exists locally."
    exit 1
fi

# Fetch latest from remote
echo "Fetching latest from remote..."
git fetch origin

# Check if branch exists on remote
if git show-ref --verify --quiet "refs/remotes/origin/$BRANCH_NAME"; then
    echo "Error: Branch '$BRANCH_NAME' already exists on remote."
    exit 1
fi

# Check if worktree directory already exists
if [[ -d "$WORKTREE_PATH" ]]; then
    echo "Error: Worktree directory already exists at $WORKTREE_PATH"
    exit 1
fi

# Get latest main
echo "Checking out latest main..."
git fetch origin main:main 2>/dev/null || git fetch origin main

# Create branch from main
echo "Creating branch '$BRANCH_NAME' from main..."
git branch "$BRANCH_NAME" main

# Create worktree
echo "Creating worktree at $WORKTREE_PATH..."
git worktree add "$WORKTREE_PATH" "$BRANCH_NAME"

echo ""
echo "Worktree created successfully!"
echo "  Branch: $BRANCH_NAME"
echo "  Path: $WORKTREE_PATH"
echo ""
echo "To start working:"
echo "  cd $WORKTREE_PATH"
