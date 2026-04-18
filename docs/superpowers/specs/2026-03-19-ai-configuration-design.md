# AI Configuration — Design Spec

This document specifies how facet manages AI coding tool configuration across Claude Code, Cursor, and Codex. It covers permissions, skills/plugins, and MCP servers.

---

## 1. Scope

### In scope
- AI configuration section in profile YAML (base, profiles, .local)
- Permission management per agent (read/merge/write settings files)
- Skills management via `npx skills` CLI
- MCP server registration/removal per agent
- Per-agent overrides with shared defaults
- Canonical permission vocabulary with agent-native mapping
- Orphan tracking for skills and MCPs via `.state.json`
- Agent provider abstraction with proper SRP and interfaces
- Comprehensive unit tests, integration tests, and hermetic E2E tests

### Out of scope
- Agent tools beyond Claude Code, Cursor, and Codex
- Auto-detection of installed agents (agents are declared in the profile)
- Installing agent tools themselves (assumed to be present)

---

## 2. Profile Schema

The `ai` section lives in `base.yaml`, `profiles/*.yaml`, and `.local.yaml`. It follows the same merge rules as the rest of the profile.

```yaml
ai:
  agents: [claude-code, cursor, codex]

  permissions:
    allow: [read, edit, bash, web-search]
    deny: [computer-use]

  skills:
    - source: vercel-labs/agent-skills
      skills: [frontend-design, writing-plans]
    - source: anthropics/claude-plugins-official
      skills: [superpowers]
      agents: [claude-code]  # only for claude-code

  mcps:
    - name: playwright
      command: npx
      args: ["@anthropic/mcp-playwright"]
    - name: github
      command: npx
      args: ["@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${facet:github.token}"
      agents: [claude-code, cursor]

  # Per-agent overrides
  overrides:
    cursor:
      permissions:
        allow: [read, edit]
        deny: []
    codex:
      permissions:
        allow: [read, edit, bash]
```

### Struct integration

An `AI` field is added to `FacetConfig` in `profile/types.go`:

```go
type FacetConfig struct {
    Extends  string            `yaml:"extends"`
    Vars     map[string]any    `yaml:"vars"`
    Packages []Package         `yaml:"packages"`
    Configs  map[string]string `yaml:"configs"`
    AI       *AIConfig         `yaml:"ai"`
}

type AIConfig struct {
    Agents      []string              `yaml:"agents"`
    Permissions *PermissionsConfig    `yaml:"permissions"`
    Skills      []SkillEntry          `yaml:"skills"`
    MCPs        []MCPEntry            `yaml:"mcps"`
    Overrides   map[string]*AIOverride `yaml:"overrides"` // keyed by agent name
}

type AIOverride struct {
    Permissions *PermissionsConfig `yaml:"permissions"`
}

type PermissionsConfig struct {
    Allow []string `yaml:"allow"`
    Deny  []string `yaml:"deny"`
}

type SkillEntry struct {
    Source string   `yaml:"source"`
    Skills []string `yaml:"skills"`
    Agents []string `yaml:"agents"` // optional, defaults to top-level agents
}

type MCPEntry struct {
    Name    string            `yaml:"name"`
    Command string            `yaml:"command"`
    Args    []string          `yaml:"args"`
    Env     map[string]string `yaml:"env"`
    Agents  []string          `yaml:"agents"` // optional, defaults to top-level agents
}
```

The `profile.Merge()` function is extended to handle the `ai` section. The `profile.Resolve()` function is extended to resolve `${facet:...}` variables within MCP `env` values, `command`, and `args` fields. The `internal/ai/` package receives an already-merged, already-resolved `AIConfig` — it does not perform any YAML parsing or variable resolution.

### Merge rules for `ai`

| Field | Merge strategy |
|---|---|
| `agents` | Last writer wins (list replacement, not union) |
| `permissions.allow` | Last writer wins (list replacement) |
| `permissions.deny` | Last writer wins (list replacement) |
| `skills` | Normalized to `(source, skill_name)` tuples; union by tuple key, last writer wins on conflict |
| `mcps` | Union by `name`, last writer wins on conflict |
| `overrides` | Deep merge by agent name, same rules as above. Only `permissions` is supported in per-agent overrides. |

### Skills merge normalization

During merge, skill entries are internally normalized to individual `(source, skill_name)` tuples. This resolves ambiguity when two layers define the same source with different skill lists:

```yaml
# base.yaml
ai:
  skills:
    - source: vercel-labs/agent-skills
      skills: [frontend-design, writing-plans]

# profile — adds code-review, keeps writing-plans, drops frontend-design
ai:
  skills:
    - source: vercel-labs/agent-skills
      skills: [writing-plans, code-review]
```

Normalized tuples from base: `(vercel-labs/agent-skills, frontend-design)`, `(vercel-labs/agent-skills, writing-plans)`.
Normalized tuples from profile: `(vercel-labs/agent-skills, writing-plans)`, `(vercel-labs/agent-skills, code-review)`.
After union with last-writer-wins: `(vercel-labs/agent-skills, frontend-design)`, `(vercel-labs/agent-skills, writing-plans)`, `(vercel-labs/agent-skills, code-review)`.

To remove a skill from a parent layer, the profile must explicitly exclude it (this will be addressed in a future version if needed). In v1, skills are additive across layers.

### Per-item agent targeting

Individual skills and MCPs can declare an `agents` list to narrow their scope. If omitted, they apply to all agents listed in the top-level `agents` field. The `agents` field serves dual purpose: it declares which agents are in scope AND provides the default target for skills/MCPs without an explicit `agents` list.

### Per-agent overrides

Per-agent overrides live under the `overrides:` key, keyed by agent name (e.g. `overrides.cursor`, `overrides.codex`). Only `permissions` is supported in per-agent overrides — skills and MCPs use per-item `agents` lists for targeting instead. The override replaces the corresponding key entirely (e.g. `overrides.cursor.permissions.allow` replaces the shared `permissions.allow` for Cursor).

### Effective resolution

Effective resolution is owned by a `Resolver` function in `internal/ai/resolve.go`. It takes the merged `AIConfig` and produces a `map[string]EffectiveAgentConfig` — one entry per agent.

```go
type EffectiveAgentConfig struct {
    Permissions ResolvedPermissions
    Skills      []ResolvedSkill
    MCPs        []ResolvedMCP
}
```

The algorithm per agent:
1. Start with shared defaults (`permissions`, `skills`, `mcps`)
2. Filter skills and MCPs: include only those whose `agents` list contains this agent (or whose `agents` list is empty, meaning all agents)
3. Apply per-agent `permissions` override if present (full replacement of `allow`/`deny`)

Resolved permissions are in **canonical terms** (Claude Code vocabulary). The orchestrator maps them to agent-native terms via `PermissionMapper` before passing to `AgentProvider.ApplyPermissions()`. The `Resolve()` function is a pure function on `AIConfig` — it has no external dependencies.

The call chain is: `App.Apply()` calls `ai.Resolve(mergedConfig.AI)` to produce the `EffectiveAIConfig`, then passes it to `AIOrchestrator.Apply()`.

---

## 3. Permission Mapping

### Canonical vocabulary

Facet uses Claude Code's permission terms as the canonical vocabulary. A mapping table translates canonical terms to each agent's native terms.

Example canonical permissions: `read`, `edit`, `bash`, `web-search`, `computer-use`, `mcp`, `notebook-edit`.

### Mapping table

| Canonical (Claude Code) | Cursor | Codex |
|---|---|---|
| `read` | `read` | `read` |
| `edit` | `edit` | `edit` |
| `bash` | `terminal` | `shell` |
| `web-search` | `web` | `web-search` |
| ... | ... | ... |

The full mapping table will be populated by referencing each tool's documentation. Unknown or unmappable permissions for a given agent are skipped with a warning logged to the console.

### Apply behavior

1. Resolve the effective permissions for each agent (shared defaults + per-agent overrides)
2. Map canonical terms to agent-native terms via the mapping table
3. Read the agent's settings file (e.g. `.claude/settings.json`)
4. Nuke only the permission keys (`allow`/`deny` or equivalent) — leave all other keys untouched
5. Write the mapped permissions back to the settings file

### Unapply behavior

Read `.state.json` to get the permission keys that were written, and remove them from each agent's settings file (set to empty arrays or remove the keys).

---

## 4. Skills Management

### Dependencies

Requires Node.js and `npx` to be available on `PATH`. Facet checks for `npx` before executing any skills commands and fails with a clear error if not found.

Skills are managed via the [skills CLI](https://github.com/vercel-labs/skills) (`npx skills`).

### Apply behavior

1. Resolve the effective skills list per agent (shared defaults + per-item `agents` filtering)
2. Read `.state.json` to get previously managed skills
3. Diff: find orphaned skills (in state but not in current profile)
4. Remove orphans: `npx skills remove <orphan1> <orphan2> -a <agent> -y` for each agent
5. Install declared skills: `npx skills add <source> --skill <name> -a <agent> -y` for each agent
6. Record managed skills in `.state.json`

### Unapply behavior

Remove all facet-managed skills from `.state.json`:
`npx skills remove <skill1> <skill2> ... -a <agent> -y` for each agent.

### Dry-run

Prints what skills would be installed/removed without executing any commands.

---

## 5. MCP Management

### Apply behavior

1. Resolve the effective MCPs per agent (shared defaults + per-item `agents` filtering)
2. Read `.state.json` to get previously managed MCPs
3. Diff: find orphaned MCPs (in state but not in current profile)
4. Remove orphans per agent:
   - Claude Code: `claude mcp remove <name>`
   - Cursor: remove entry from `.cursor/mcp.json`
   - Codex: remove from its config file
5. Register declared MCPs per agent:
   - Claude Code: `claude mcp add <name> -- <command> <args...>` (with `-e KEY=VAL` for env vars)
   - Cursor: write entry into `.cursor/mcp.json`
   - Codex: write to its config file
6. Record managed MCPs in `.state.json`

### Env vars

MCP environment variables use the standard `${facet:...}` variable system. Resolution happens in `profile.Resolve()`, which is extended to walk the `ai` section (specifically MCP `env` values, `command`, and `args` fields). By the time the `internal/ai/` package receives the config, all variables are already resolved.

### Unapply behavior

Remove all facet-managed MCPs from each agent's config, using the same per-agent mechanisms as removal.

### Dry-run

Prints what MCPs would be registered/removed without executing any commands.

---

## 6. State Tracking

The existing `~/.facet/.state.json` is extended with an `ai` section. The `ApplyState` struct in `internal/app/state.go` gains an `AI` field:

```go
type AIState struct {
    Skills      []SkillState                `json:"skills"`
    MCPs        []MCPState                  `json:"mcps"`
    Permissions map[string]PermissionState  `json:"permissions"` // keyed by agent name
}

type SkillState struct {
    Source string   `json:"source"`
    Name   string   `json:"name"`
    Agents []string `json:"agents"` // agents where install succeeded
}

type MCPState struct {
    Name   string   `json:"name"`
    Agents []string `json:"agents"` // agents where registration succeeded
}

type PermissionState struct {
    Allow []string `json:"allow"` // stored in agent-native terms
    Deny  []string `json:"deny"`
}
```

Example `.state.json`:

```json
{
  "profile": "work",
  "applied_at": "2026-03-19T10:30:00Z",
  "facet_version": "0.2.0",
  "packages": [],
  "configs": [],
  "ai": {
    "skills": [
      {
        "source": "vercel-labs/agent-skills",
        "name": "frontend-design",
        "agents": ["claude-code", "cursor", "codex"]
      },
      {
        "source": "anthropics/claude-plugins-official",
        "name": "superpowers",
        "agents": ["claude-code"]
      }
    ],
    "mcps": [
      {
        "name": "playwright",
        "agents": ["claude-code", "cursor", "codex"]
      },
      {
        "name": "github",
        "agents": ["claude-code", "cursor"]
      }
    ],
    "permissions": {
      "claude-code": { "allow": ["Read", "Edit", "Bash"], "deny": [] },
      "cursor": { "allow": ["read", "edit"], "deny": [] },
      "codex": { "allow": ["read", "edit", "shell"], "deny": [] }
    }
  }
}
```

Permissions are stored in agent-native terms (post-mapping) so unapply knows exactly what to clean up without re-running the mapping.

### Partial failure recording

When an AI step partially succeeds (e.g. skills install succeeds for 2 of 3 agents), state records only the agents where the operation succeeded. This ensures orphan diffing on the next apply correctly targets only agents that actually have the skill/MCP installed. The `agents` list in state reflects actual installation state, not intent.

---

## 7. Apply Pipeline Integration

AI configuration steps are added to the existing apply pipeline after config deployment:

| Step | Description | Failure behavior |
|---|---|---|
| 1. Find config dir | Locate `facet.yaml` | Fatal |
| 2. Load `facet.yaml` | Parse metadata | Fatal |
| 3. Load `base.yaml` | Parse base config | Fatal |
| 4. Load `profiles/<name>.yaml` | Parse profile | Fatal |
| 5. Load `.local.yaml` | Parse local overrides | Fatal if missing |
| 6. Merge layers | Combine base + profile + local | Fatal on type conflicts |
| 7. Resolve `${facet:...}` | Variable substitution (including `ai` section) | Fatal on undefined var |
| 8. Write canary to `.state.json` | Early permission/disk check | Fatal |
| 9. Install packages | Run install commands | Non-fatal, continue |
| 10. Deploy configs | Symlink/template files | Rollback or skip-failure |
| **11. Apply AI permissions** | **Write permission keys to agent settings files** | **Non-fatal, continue** |
| **12. Apply AI skills** | **Remove orphans + install via `npx skills`** | **Non-fatal, continue** |
| **13. Apply AI MCPs** | **Remove orphans + register via agent CLIs/config files** | **Non-fatal, continue** |
| 14. Write final `.state.json` | Persist state (including AI section) | Fatal |

AI steps are non-fatal, consistent with package installation. Errors are logged and recorded in state.

### AI orphan reconciliation

AI steps 11-13 always run orphan diffing against `.state.json`, regardless of whether this is a same-profile reapply or a profile switch. This differs from config unapply (which only runs on `--force` or profile switch) because skills and MCPs need to detect items removed from the profile.

### Unapply order (profile switch or --force)

When switching profiles or using `--force`, the full unapply runs before apply. The order is: MCPs -> skills -> permissions -> configs. AI unapply removes all facet-managed AI state from the previous profile before applying the new one.

### facet status

Extended to display AI state: installed skills, registered MCPs, and permissions per agent.

---

## 8. Abstraction and Interfaces

### App-level interface

Following CLAUDE.md rules ("define interfaces where the consumer lives"), the interface consumed by the `app` layer is defined in `internal/app/interfaces.go`:

```go
// internal/app/interfaces.go

type AIOrchestrator interface {
    Apply(config EffectiveAIConfig, previousState *AIState) (*AIState, error)
    Unapply(previousState *AIState) error
}
```

The `App` struct depends on this interface. The concrete `ai.Orchestrator` implements it, wired in `main.go`.

### Internal AI interfaces

Interfaces consumed within the `internal/ai/` package live in `internal/ai/interfaces.go`:

```go
// internal/ai/interfaces.go

type AgentProvider interface {
    Name() string
    ApplyPermissions(permissions ResolvedPermissions) error
    RemovePermissions(previousPermissions ResolvedPermissions) error
    RegisterMCP(mcp ResolvedMCP) error
    RemoveMCP(name string) error
    SettingsFilePath() string
}

type PermissionMapper interface {
    MapToNative(canonical string, agent string) (string, error)
    MapAllToNative(canonical []string, agent string) ([]string, []string)
    // Returns (mapped, warnings) — warnings for unmappable terms
}

type SkillsManager interface {
    Install(source string, skills []string, agents []string) error
    Remove(skills []string, agents []string) error
}
```

### Type definitions

```go
// internal/ai/types.go

type ResolvedPermissions struct {
    Allow []string // canonical terms (mapped to native by orchestrator before apply)
    Deny  []string // canonical terms (mapped to native by orchestrator before apply)
}

type ResolvedSkill struct {
    Source string
    Name   string
}

type ResolvedMCP struct {
    Name    string
    Command string
    Args    []string
    Env     map[string]string // already resolved via ${facet:...}
}

type EffectiveAgentConfig struct {
    Permissions ResolvedPermissions
    Skills      []ResolvedSkill
    MCPs        []ResolvedMCP
}

// EffectiveAIConfig is a map of agent name to its resolved config.
type EffectiveAIConfig map[string]EffectiveAgentConfig
```

### Concrete implementations

- `internal/ai/orchestrator.go` — `Orchestrator` (implements `app.AIOrchestrator`)
- `internal/ai/resolve.go` — `Resolve(config *profile.AIConfig) EffectiveAIConfig`
- `internal/ai/claude_code_provider.go` — `ClaudeCodeProvider`
- `internal/ai/cursor_provider.go` — `CursorProvider`
- `internal/ai/codex_provider.go` — `CodexProvider`
- `internal/ai/permission_mapper.go` — `DefaultPermissionMapper`
- `internal/ai/skills_manager.go` — `NPXSkillsManager` (takes `ShellRunner` via constructor injection)

### AI orchestrator

```go
// internal/ai/orchestrator.go

type Orchestrator struct {
    providers       map[string]AgentProvider
    permMapper      PermissionMapper
    skillsManager   SkillsManager
    reporter        Reporter
}

func NewOrchestrator(
    providers map[string]AgentProvider,
    permMapper PermissionMapper,
    skillsManager SkillsManager,
    reporter Reporter,
) *Orchestrator
```

All concrete implementations wired in `main.go`.

---

## 9. Testing Strategy

### Unit tests

Fully hermetic — no real filesystem, no real CLI calls.

- **AgentProvider implementations**: Use `t.TempDir()` for settings files. Verify JSON read/merge/write. Test that only permission keys are modified while other settings are preserved.
- **PermissionMapper**: Test all canonical-to-native mappings. Test unknown permission warning behavior.
- **SkillsManager**: Mock `ShellRunner` interface. Verify correct CLI commands are constructed (arguments, flags, ordering) without executing them.
- **Orchestrator**: Mock all providers, permission mapper, and skills manager. Verify:
  - Orphan diffing logic (skills/MCPs removed from profile are detected)
  - Correct apply ordering (permissions -> skills -> MCPs)
  - Correct unapply ordering (MCPs -> skills -> permissions)
  - Per-agent filtering and override resolution
  - Non-fatal error handling (one agent failing doesn't block others)

### Integration tests

Use real filesystem (`t.TempDir()`) with mock shell executor. Verify the full orchestrator flow:
- Settings files are read/patched/written correctly across multiple agents
- State JSON is updated with correct AI section
- Profile merge rules work correctly for the `ai` section

### E2E tests

Hermetic E2E using stub CLI scripts. Test setup:

1. Create a temp dir with a full facet config repo (including `ai` section in profiles)
2. Create stub CLI scripts (`skills`, `claude`) in a temp bin directory:
   - Stubs verify they received the correct arguments (fail the test if not)
   - Stubs create expected side-effects (write a skill symlink, write an MCP config entry)
   - Stubs return expected exit codes
3. Prepend the temp bin directory to `PATH`
4. Run `facet apply <profile>`
5. Assert:
   - Settings files are patched correctly
   - State JSON is updated with AI section
   - Stub scripts received the correct arguments
   - Orphan removal works when reapplying with a reduced skill/MCP list

---

## 10. Non-Requirements

| Feature | Why excluded |
|---|---|
| Agent auto-detection | Agents are declared explicitly in the profile |
| Installing agent tools | Assumed to be already installed |
| Agents beyond Claude Code, Cursor, Codex | Out of scope for this version |
| Skills diffing (install only changed) | Nuke-and-pave per managed skill is simpler |
| Permission key merging (vs replacement) | Nuke-and-pave permission keys, leave other settings alone |
| Interactive skills installation | Always non-interactive (`-y` flag) |
| Custom skills CLI path | Always uses `npx skills` |
