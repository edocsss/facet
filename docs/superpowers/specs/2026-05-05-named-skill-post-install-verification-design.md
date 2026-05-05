# Named Skill Post-Install Verification

**Date:** 2026-05-05
**Status:** Approved

## Problem

When installing named skills (e.g. `--skill wagent-task-dev`), facet calls
`npx skills add <source> --skill <name> ... -y`, trusts the exit code 0, and
records the requested names in state — even when those skills don't exist in the
source repo. The `npx skills` CLI exits 0 silently in this case. As a result,
the apply report shows `✓ Skill: wagent-task-dev` for skills that were never
actually installed, and the state records them as if they were.

The "all skills" path (`Name: ""`) already calls `InstalledForSource()` after
install to verify what landed and records only what the lock file confirms. Named
skill installs have no equivalent check.

## Approach

Symmetric post-install verification: after any successful named-skill
`Install()` call, read the skill lock via `InstalledForSource()` and cross-check
the requested names against what is actually present. Warn for any that are
missing; record only the confirmed ones in state.

This mirrors the existing "all" path exactly and requires no interface changes.

## Design

### Logic change in `applySkills` (`orchestrator.go`)

In the post-install recording block, the named-skill path currently does:

```go
recordedSkills := skills   // trusts requested names unconditionally
```

Replace with:

```go
recordedSkills := skills   // fallback if lock read fails
verified, err := o.skillsManager.InstalledForSource(gk.source)
if err != nil {
    o.reporter.Warning(fmt.Sprintf(
        "installed skills from %q but could not verify via skill lock: %v — recording requested names",
        gk.source, err))
} else {
    verifiedSet := make(map[string]struct{}, len(verified))
    for _, s := range verified {
        verifiedSet[s] = struct{}{}
    }
    var confirmed, missing []string
    for _, s := range skills {
        if _, ok := verifiedSet[s]; ok {
            confirmed = append(confirmed, s)
        } else {
            missing = append(missing, s)
        }
    }
    if len(missing) > 0 {
        o.reporter.Warning(fmt.Sprintf(
            "skills %v from %q were not found in the skill lock after install — they may not exist in the source",
            missing, gk.source))
    }
    recordedSkills = confirmed   // nil if nothing landed
}
```

### Success message

Change the success line to use `recordedSkills` instead of `skills`:

```go
if len(recordedSkills) > 0 {
    sort.Strings(recordedSkills)
    o.reporter.Success(fmt.Sprintf("installed skills %v from %s", recordedSkills, gk.source))
}
```

If `recordedSkills` is empty (total miss), no success line is emitted — only the
warning above.

### Error / fallback behaviour summary

| Situation | Warning | State recorded |
|---|---|---|
| All requested skills land in lock | none | all requested names |
| Some skills missing from lock | `⚠ skills [x] not found in lock` | confirmed names only |
| All skills missing from lock | `⚠ skills [x y] not found in lock` | nothing |
| `InstalledForSource` returns error | `⚠ could not verify via skill lock` | all requested names (fallback) |

### No interface changes

`SkillsManager.InstalledForSource()` already exists and is already called on
the "all" path. No changes to `interfaces.go`, `skills_manager.go`,
`state.go`, or `types.go`.

## Tests

Four new tests in `orchestrator_test.go`:

1. **`TestOrchestrator_Apply_NamedSkillVerification_AllConfirmed`** — install
   `[a, b]`, lock has `[a, b]` → no warning, both recorded in state.

2. **`TestOrchestrator_Apply_NamedSkillVerification_PartialMiss`** — install
   `[a, b, c]`, lock has `[a, b]` only → warning fires for `[c]`, only `[a, b]`
   recorded in state.

3. **`TestOrchestrator_Apply_NamedSkillVerification_TotalMiss`** — install
   `[a, b]`, lock returns nothing for source → warning fires for `[a, b]`,
   nothing recorded in state.

4. **`TestOrchestrator_Apply_NamedSkillVerification_LockReadError`** — install
   `[a, b]`, `InstalledForSource` returns an error → fallback warning fires,
   both `[a, b]` recorded in state.

Test 4 needs a new mock variant that returns an error from `InstalledForSource`.
It lives inline in `orchestrator_test.go`.

The existing `mockSkillsMgr.sourceSkills` map is sufficient for tests 1–3.

## Documentation updates

All three locations must be updated:

### `internal/docs/topics/ai.md`

Add a paragraph under the skill reconciliation section explaining that after
installing named skills, facet verifies them against the skill lock and warns if
any requested names are not found.

### `docs/architecture/v1-design-spec.md`

Extend section 11 (AI Configuration) to note that named skill installs are
post-verified via the skill lock, and that missing skills produce a warning
rather than a fatal error.

### `README.md`

Verify no skill-install detail needs updating (expected: no change needed).

### `e2e/suites/`

Add a test case to the existing AI skills E2E suite (`10-ai-config.sh` or a new
`17-named-skill-verification.sh`) that:
- Installs a named skill from a source where the skill exists → assert `✓`
- Installs a named skill that does not exist in the source → assert `⚠` warning
  in output and that the skill is absent from `.state.json`
