# Design: All Skills From Source

**Date:** 2026-03-30
**Status:** Approved

## Problem

Currently, each skill must be listed individually in the YAML config:

```yaml
ai:
  skills:
    - source: "@anthropic/claude-code-skills"
      skills: [code-review, testing, debugging, refactoring]
```

This is tedious when you want every skill from a source. The `npx skills add` CLI already supports an `--all` flag, but facet has no way to express "all skills" in config.

## Solution

When a `SkillEntry` has a `source` but an empty or omitted `skills` list, it means "install all skills from this source."

### YAML Config

```yaml
ai:
  skills:
    # All skills from this source, all agents
    - source: "@anthropic/claude-code-skills"

    # All skills from this source, scoped to specific agents
    - source: "owner/repo"
      agents: [claude-code]

    # Specific skills (existing behavior, unchanged)
    - source: "other/repo"
      skills: [just-this-one]
```

Both `skills: []` (explicit empty array) and omitted `skills` (nil) are treated as "all." A `SkillEntry` with no `source` remains invalid.

### CLI Invocation

When `skills` is empty, `NPXSkillsManager.Install()` passes `--all` to `npx skills add` instead of individual `-s` flags:

```
npx skills add @anthropic/claude-code-skills --all -a claude-code -a cursor
```

vs. the current specific-skill form:

```
npx skills add @anthropic/claude-code-skills -s code-review -s testing -a claude-code
```

### Merge Behavior

Last-writer-wins by `(source, skill_name)` tuple, consistent with existing merge semantics.

- **"All" in later layer replaces specifics from earlier layer:** An "all" entry has no individual skill names. It replaces the entire source's skill tuples from prior layers as a single atomic unit.
- **Specifics in later layer replace "all" from earlier layer:** Individual skill entries from a profile/local override a base "all" entry. The result is only the specific skills listed.
- **"All" does not affect other sources:** An "all" entry for source A has no effect on source B's entries.

#### Merge Implementation

The `mergeSkills` function currently flattens entries into `(source, skillName, agents)` tuples. For "all" entries:

- Stored as a single tuple with an empty skill name sentinel.
- When an "all" tuple exists for a source in a later layer, it removes all tuples for that source from earlier layers and replaces them with the "all" sentinel.
- When individual skill tuples exist in a later layer for a source that was "all" in an earlier layer, they replace the "all" sentinel.

### State Tracking

Currently, `.state.json` records each installed skill by name. For "all" entries, individual skill names are not known at config time.

- An "all" entry is recorded in state with an empty skill name (`""`) to represent "all from this source."
- Orphan detection handles transitions:
  - Specific -> "all": old individual state entries removed, new "all" entry added.
  - "All" -> specific: old "all" entry removed, new individual entries added.

### Resolve

`resolveAgentSkills()` passes entries with empty `Skills` slices through to the orchestrator without expansion. The orchestrator groups them as usual by `(source, agents)`.

## Testing

### Unit Tests

**Merge (`mergeSkills`):**
- "All" entry with no prior entries -> produces "all" entry
- "All" in base, specific in profile -> profile wins, only specific skills
- Specific in base, "all" in profile -> profile wins, "all" replaces individuals
- "All" in base, "all" in profile with different agents -> profile wins
- "All" for source A does not affect source B
- Three-layer: base specifics, profile "all", local specifics -> local wins

**Resolve (`resolveAgentSkills`):**
- "All" entry with no agents -> appears for every agent
- "All" entry with agent scoping -> only those agents
- Mix of "all" and specific entries from different sources

**Orchestrator (`applySkills`):**
- "All" entry calls `Install(source, nil, agents)` (empty skills slice)
- Mixed: "all" for one source, specific for another -> correct calls
- Orphan removal: "all" -> specific transition
- Orphan removal: specific -> "all" transition

**`NPXSkillsManager.Install()`:**
- Empty skills slice -> `--all` flag, no `-s` flags
- Non-empty skills slice -> unchanged behavior

**State tracking:**
- "All" entry produces correct state representation
- State diff detects "all" <-> specific transitions correctly

### E2E Tests

New suite covering:
- Apply with "all skills" entry -> verify `--all` invocation
- Apply with mixed "all" and specific entries -> correct commands
- "All" scoped to specific agents -> `--all` with `-a` flags
- Transition from specific to "all" -> orphan cleanup + `--all` install
- Transition from "all" to specific -> orphan cleanup + specific install

## Documentation Updates

1. **`internal/docs/topics/`** — update skills topic with "omit skills = all" behavior and examples.
2. **`README.md`** — update skills config examples to show both forms.
3. **`docs/architecture/v1-design-spec.md`** — update `SkillEntry` definition and merge semantics.
