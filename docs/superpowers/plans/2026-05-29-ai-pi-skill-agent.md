# AI Pi Skill Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `pi` as a supported `npx skills` agent name for facet AI skill installation.

**Architecture:** Keep Pi skill support in the AI skill resolution path by adding `pi` to the default skill-agent allowlist. Avoid adding Pi permissions or MCP behavior; orchestration should not warn for Pi when it has no permissions/MCP work to perform. Documentation and E2E tests must describe and protect the new behavior.

**Tech Stack:** Go, Cobra app wiring, shell-based E2E suites, `npx skills` CLI.

---

### Task 1: Protect Pi default skill resolution

**Files:**
- Modify: `internal/ai/resolve_test.go`
- Modify: `internal/ai/resolve.go`

- [ ] **Step 1: Write failing unit coverage**

Add `pi` to `TestResolve_SkillDefaultAgents_ExcludesNonDefault` input and assert that Pi receives the default skill while non-default agents remain excluded.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai -run TestResolve_SkillDefaultAgents_ExcludesNonDefault -count=1`
Expected: FAIL because `result["pi"].Skills` is empty.

- [ ] **Step 3: Implement minimal change**

Add `"pi": true` to `DefaultSkillAgents` in `internal/ai/resolve.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai -run TestResolve_SkillDefaultAgents_ExcludesNonDefault -count=1`
Expected: PASS.

### Task 2: Prevent unsupported-provider noise for skill-only Pi

**Files:**
- Modify: `internal/ai/orchestrator_test.go`
- Modify: `internal/ai/orchestrator.go`

- [ ] **Step 1: Write failing unit coverage**

Add a test that applies a config with `pi` skills but no Pi permissions or MCPs and asserts no warning containing `no provider for agent "pi"` is emitted.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai -run TestOrchestrator_Apply_SkillOnlyAgentWithoutProviderDoesNotWarnForPermissions -count=1`
Expected: FAIL because current permission orchestration warns for missing Pi provider.

- [ ] **Step 3: Implement minimal change**

In `applyPermissions`, skip agents without providers when both allow and deny permissions are empty. This keeps unsupported permission warnings for agents with requested permissions, but avoids noise for skill-only agents.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai -run TestOrchestrator_Apply_SkillOnlyAgentWithoutProviderDoesNotWarnForPermissions -count=1`
Expected: PASS.

### Task 3: Add E2E protection for npx agent name

**Files:**
- Modify: `e2e/suites/17-named-skill-verification.sh`

- [ ] **Step 1: Add failing E2E coverage**

Add a scenario with `agents: [pi]` and a named skill in the lock. Assert `.state.json` records `pi` and `.mock-ai` includes `npx skills add ... -a pi`.

- [ ] **Step 2: Run suite to verify it fails before implementation**

Run: `go test ./e2e -run TestE2E/17-named-skill-verification -count=1`
Expected before code changes: FAIL because Pi default/explicit behavior is not fully supported without warnings/state behavior.

- [ ] **Step 3: Verify after implementation**

Run: `go test ./e2e -run TestE2E/17-named-skill-verification -count=1`
Expected: PASS.

### Task 4: Update user-facing docs

**Files:**
- Modify: `internal/docs/topics/ai.md`
- Modify: `README.md`
- Modify: `docs/architecture/v1-design-spec.md`

- [ ] **Step 1: Update docs**

Document that unscoped skills default to `claude-code`, `cursor`, `codex`, and `pi`, and that the `npx skills` agent name is `pi`.

- [ ] **Step 2: Run docs-related tests**

Run: `go test ./internal/docs ./cmd -count=1`
Expected: PASS.

### Task 5: Final verification

- [ ] Run: `go test ./internal/ai ./internal/app ./internal/profile -count=1`
- [ ] Run: `go test ./e2e -run TestE2E/17-named-skill-verification -count=1`
- [ ] Run: `git diff --check`
