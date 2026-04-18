#!/bin/bash
#
# Hermetic E2E test runner. Creates an isolated HOME for each suite.
# The parent shell's HOME and PATH are NEVER modified.
set -euo pipefail

# ── Create top-level sandbox ──
REAL_HOME="$HOME"
export E2E_SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/facet-e2e.XXXXXXXX")

# Cleanup on any exit — sandbox is always deleted
cleanup() {
    local exit_code=$?
    if [ -n "${E2E_SANDBOX:-}" ] && [ -d "$E2E_SANDBOX" ]; then
        chmod -R u+w "$E2E_SANDBOX" 2>/dev/null || true
        rm -rf "$E2E_SANDBOX"
    fi
    exit $exit_code
}
trap cleanup EXIT

# Resolve suite/fixture locations
if [ -d "/opt/e2e" ]; then
    # Docker: files were COPYed to /opt/e2e
    SUITE_DIR="/opt/e2e/suites"
    FIXTURE_DIR="/opt/e2e/fixtures"
    FACET_BIN="/usr/local/bin/facet"
else
    # Native: relative to this script
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    SUITE_DIR="$SCRIPT_DIR/suites"
    FIXTURE_DIR="$SCRIPT_DIR/fixtures"
    REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
    BIN_DIR="$E2E_SANDBOX/bin"
    FACET_BIN="$BIN_DIR/facet"
    BUILD_HOME="$E2E_SANDBOX/build-home"
    GO_CACHE_DIR="$E2E_SANDBOX/go-cache"
    GO_MOD_CACHE_DIR="$E2E_SANDBOX/go-mod-cache"
    mkdir -p "$BIN_DIR"
    mkdir -p "$BUILD_HOME"
    mkdir -p "$GO_CACHE_DIR"
    mkdir -p "$GO_MOD_CACHE_DIR"

    echo "  Building facet CLI into sandbox..."
    (
        cd "$REPO_ROOT"
        export HOME="$BUILD_HOME"
        export GOCACHE="$GO_CACHE_DIR"
        export GOMODCACHE="$GO_MOD_CACHE_DIR"
        go build -modcacherw -o "$FACET_BIN" .
    )
    chmod +x "$FACET_BIN"
fi

# Filter suites if specific ones requested
if [ $# -gt 0 ]; then
    SUITES=("$@")
else
    SUITES=("$SUITE_DIR"/[0-9]*.sh)
fi

echo "========================================"
echo "  facet E2E test run"
echo "  $(date -Iseconds 2>/dev/null || date)"
echo "  Sandbox: $E2E_SANDBOX"
echo "  OS: $(uname -s) $(uname -m)"
echo "  facet: $("$FACET_BIN" --version 2>/dev/null || echo 'not found')"
echo "========================================"
echo ""

PASSED=0
FAILED=0
ERRORS=()
RUN_START=$(date +%s)

for suite in "${SUITES[@]}"; do
    name=$(basename "$suite" .sh)
    [[ "$name" == "helpers" ]] && continue

    echo "--- [$name] ---"
    SUITE_START=$(date +%s)

    # Each suite gets its own clean HOME subdirectory.
    # HOME and PATH are only set for the child process — parent is untouched.
    SUITE_HOME=$(mktemp -d "$E2E_SANDBOX/suite.XXXXXXXX")
    SUITE_PATH="$SUITE_HOME/mock-bin:$(dirname "$FACET_BIN"):$PATH"

    # Set up mock tools in the suite's HOME (scoped to child process)
    if HOME="$SUITE_HOME" PATH="$SUITE_PATH" \
       FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$FIXTURE_DIR/mock-tools.sh" >/dev/null 2>&1 \
       && HOME="$SUITE_HOME" PATH="$SUITE_PATH" \
       FIXTURE_DIR="$FIXTURE_DIR" SUITE_DIR="$SUITE_DIR" \
       bash "$suite" 2>&1; then
        SUITE_END=$(date +%s)
        echo "  ✓ PASS ($(( SUITE_END - SUITE_START ))s)"
        PASSED=$((PASSED + 1))
    else
        SUITE_END=$(date +%s)
        echo "  ✗ FAIL (exit $?) ($(( SUITE_END - SUITE_START ))s)"
        FAILED=$((FAILED + 1))
        ERRORS+=("$name")
    fi
    echo ""
done

RUN_END=$(date +%s)
TOTAL_SECS=$(( RUN_END - RUN_START ))

echo "========================================"
echo "  Results: $PASSED passed, $FAILED failed (${TOTAL_SECS}s)"
if [ $FAILED -gt 0 ]; then
    echo "  Failed: ${ERRORS[*]}"
    echo "========================================"
    exit 1
fi
echo "========================================"
exit 0
