#!/bin/bash

set -euo pipefail

# remove-worktrees.sh - Remove git worktrees that are merged or stale
#
# Usage: remove-worktrees.sh [--real]
# Default: dry-run mode (shows what would be removed)
# With --real: actually removes worktrees
#
# A worktree is removed if:
# - Its branch is merged into main, OR
# - Its branch no longer exists on remote

REPO_ROOT="$(git rev-parse --show-toplevel)"
WORKTREES_DIR="$REPO_ROOT/.worktrees"
REAL_RUN=false

# Parse arguments
if [[ $# -gt 0 && "$1" == "--real" ]]; then
    REAL_RUN=true
fi

# Check if worktrees directory exists
if [[ ! -d "$WORKTREES_DIR" ]]; then
    echo "No .worktrees directory found at $WORKTREES_DIR"
    exit 0
fi

# Fetch latest from remote
echo "Fetching latest from remote..."
if ! git fetch origin; then
    echo "Warning: fetch failed, using cached remote refs"
fi

echo ""
echo "Scanning worktrees..."
if [[ "$REAL_RUN" == false ]]; then
    echo "(DRY RUN - no worktrees will be deleted)"
fi
echo ""

removed_count=0
skipped_count=0
failed_count=0

# Process each worktree
for worktree_path in "$WORKTREES_DIR"/*; do
    # Skip if not a directory
    [[ -d "$worktree_path" ]] || continue

    worktree_name=$(basename "$worktree_path")

    # Get the branch name - try to read from git
    branch_name=$(git -C "$worktree_path" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

    if [[ -z "$branch_name" ]]; then
        echo "Skipped: $worktree_name (could not determine branch)"
        ((skipped_count++))
        continue
    fi

    # Check if worktree is currently checked out (current working directory)
    if [[ "$PWD" == "$worktree_path" || "$PWD" == "$worktree_path"/* ]]; then
        echo "Skipped: $worktree_name (currently checked out)"
        ((skipped_count++))
        continue
    fi

    # Check for uncommitted changes
    if ! git -C "$worktree_path" diff-index --quiet HEAD -- 2>/dev/null; then
        echo "Failed: $worktree_name (has uncommitted changes)"
        ((failed_count++))
        continue
    fi

    # Check if branch is merged into main
    if git branch -r --merged origin/main | grep -qE "^\s+origin/$branch_name$"; then
        is_merged=1
    else
        is_merged=0
    fi

    # Check if branch exists on remote
    branch_on_remote=$(git show-ref --verify --quiet "refs/remotes/origin/$branch_name" && echo "true" || echo "false")

    # Check if branch has unmerged commits (commits not reachable from main)
    has_unmerged=$(git log main.."$branch_name" --oneline 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$is_merged" -gt 0 || ("$branch_on_remote" == "false" && "$has_unmerged" -eq 0) ]]; then
        if [[ "$REAL_RUN" == true ]]; then
            git worktree remove "$worktree_path"
            git branch -D "$branch_name" 2>/dev/null || true
            echo "Removed: $worktree_name (branch: $branch_name)"
        else
            echo "Would remove: $worktree_name (branch: $branch_name)"
            if [[ "$is_merged" -gt 0 ]]; then
                echo "  Reason: branch merged into main"
            else
                echo "  Reason: branch no longer exists on remote (no unmerged commits)"
            fi
        fi
        ((removed_count++))
    else
        echo "Skipped: $worktree_name (branch still active)"
        ((skipped_count++))
    fi
done

echo ""
echo "Summary:"
echo "  Removable: $removed_count"
echo "  Skipped: $skipped_count"
echo "  Failed: $failed_count"
echo ""

if [[ "$REAL_RUN" == false && "$removed_count" -gt 0 ]]; then
    echo "To actually remove these worktrees, run:"
    echo "  $0 --real"
fi

# Exit with error if any failed
if [[ "$failed_count" -gt 0 ]]; then
    exit 1
fi
