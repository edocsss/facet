# AI Configuration Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Fix the AI configuration bugs found in branch review around skill identity, per-agent orphan cleanup, permission cleanup, and shell-safe command execution.

**Architecture:** Keep the existing `internal/ai` package boundaries, but tighten identity and reconciliation logic to match the v1 AI design spec. Replace shell-string command construction with argv-based execution so provider and skills commands are deterministic and safe.

**Tech Stack:** Go, testify, existing E2E shell harness

**Design Spec:** `docs/superpowers/specs/2026-03-19-ai-configuration-design.md`

---

## Scope

### Behavior fixes

1. Preserve skill identity by `(source, skill_name)` instead of `skill_name` alone.
2. Diff skills and MCPs at the per-agent level so same-profile edits remove stale registrations.
3. Remove permissions for agents that were previously managed but are no longer in the effective config.
4. Execute external commands without `sh -c` string joining.

### Test additions

1. Unit tests for skill identity collisions across sources.
2. Unit tests for per-agent orphan reconciliation on same-profile edits.
3. Unit tests for permission cleanup when an agent is removed.
4. Unit tests for argv-based command execution with spaces and shell metacharacters.
5. E2E coverage for same-profile AI edits that shrink agent scope.

---

## Tasks

### Task 1: Add failing orchestrator tests

**Files:**
- Modify: `internal/ai/orchestrator_test.go`

- [ ] Add a failing test where the same skill name appears from two different sources.
- [ ] Add a failing test where a skill remains for one agent but must be removed from another.
- [ ] Add a failing test where an MCP remains for one agent but must be removed from another.
- [ ] Add a failing test where previous permissions exist for an agent missing from the current config.
- [ ] Run targeted orchestrator tests and confirm they fail for the expected reason.

### Task 2: Add failing command execution tests

**Files:**
- Modify: `internal/ai/claude_code_provider_test.go`
- Modify: `internal/ai/skills_manager_test.go`
- Modify: `internal/ai/test_helpers_test.go`

- [ ] Add failing tests that assert providers and skills manager pass argv slices instead of shell-joined strings.
- [ ] Include arguments containing spaces or shell metacharacters.
- [ ] Run targeted tests and confirm they fail for the expected reason.

### Task 3: Implement minimal AI fixes

**Files:**
- Modify: `internal/ai/interfaces.go`
- Create: `internal/ai/exec_runner.go`
- Modify: `internal/ai/orchestrator.go`
- Modify: `internal/ai/claude_code_provider.go`
- Modify: `internal/ai/skills_manager.go`
- Modify: `main.go`

- [ ] Introduce argv-based command execution.
- [ ] Rework skill diffing/grouping to preserve `(source, skill_name)` identity.
- [ ] Rework skill and MCP orphan removal to operate on per-agent deltas.
- [ ] Reconcile permission removals for agents dropped from the effective config.
- [ ] Run targeted unit tests until green.

### Task 4: Extend E2E coverage

**Files:**
- Modify: `e2e/suites/10-ai-config.sh`

- [ ] Add same-profile AI edit coverage that shrinks agent scope.
- [ ] Verify stale skill, MCP, and permission state is removed.
- [ ] Run the AI E2E suite locally and confirm it passes.

### Task 5: Full verification

**Files:**
- None

- [ ] Run `go test ./...`
- [ ] Run `go test -cover ./internal/ai ./internal/app ./internal/profile`
- [ ] Run `make e2e-local-suite SUITE=10-ai-config`
- [ ] Summarize any remaining risk or follow-up work.
