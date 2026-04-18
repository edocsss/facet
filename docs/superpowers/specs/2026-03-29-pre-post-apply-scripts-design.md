# Pre/Post Apply Scripts & Stages Design

## Summary

Remove the standalone `facet init` command. Replace the `init` YAML field with `pre_apply` and `post_apply` script fields that run automatically within `facet apply`. Add a `--stages` flag to let users selectively run specific stages of the apply workflow.

## YAML Config Changes

Rename `init` → `post_apply`, add new `pre_apply` field. Both use the same type:

```go
type ScriptEntry struct {
    Name string `yaml:"name"`
    Run  string `yaml:"run"`
}
```

```yaml
# base.yaml
pre_apply:
  - name: setup ssh keys
    run: ./scripts/setup-ssh.sh

packages:
  - name: docker
    install: brew install docker

post_apply:
  - name: configure git
    run: git config --global user.email "${facet:git.email}"
```

Breaking change: existing configs with `init:` must be updated to `post_apply:`. No migration — pre-1.0.

## Apply Workflow

```
1.  Load / merge / resolve config              (always)
2.  Dry-run check → exit if --dry-run          (always)
3.  Canary write                                (always)
4.  Read previous state                         (always)
5.  Unapply previous state if needed            (always)
6.  [configs]    deploy symlinks/templates      (stage-gated)
7.  [pre_apply]  run pre_apply scripts          (stage-gated)
8.  [packages]   install packages               (stage-gated)
9.  [post_apply] run post_apply scripts         (stage-gated)
10. [ai]         apply AI configuration         (stage-gated)
11. Write final state                           (always)
12. Print report                                (always)
```

## Stages Flag

- Flag: `--stages` (comma-separated string)
- Valid values: `configs`, `pre_apply`, `packages`, `post_apply`, `ai`
- Default (no flag): all five stages
- Invalid stage name → error listing valid stages
- Stage names listed in `--help` and `facet docs` output

## Stage Gating

Each stage block checks membership in the stages set before executing. For skipped stages, carry forward results from previous state so we don't lose track of what was deployed.

## Unapply Behavior

Unapply always runs regardless of `--stages`. It is cleanup of previous state, not a user-facing stage.

## Script Execution Model

Applies to both `pre_apply` and `post_apply`:

- Sequential execution, fail-fast on first error
- Inherits stdin/stdout/stderr (interactive support)
- Working directory is the config directory
- Variable resolution: `${facet:var.name}` in `run` field only; `name` left literal
- Undefined variable → error at resolution time

## Merge Rules

Same concatenation strategy for both fields:

- `mergePreApply(base, overlay)` — base scripts first, then overlay
- `mergePostApply(base, overlay)` — base scripts first, then overlay
- Deep copy, no deduplication
- `.local.yaml` can contribute scripts too (same merge as third layer)

## Resolve Rules

Same as current init resolution:

- Only `Run` field gets `${facet:...}` substitution
- `Name` field is not resolved
- Undefined variable → error with script index and name

## Removed Components

- `cmd/init_cmd.go` — command registration
- `internal/app/init.go` — init workflow
- `internal/app/init_test.go` — init unit tests
- `printInitReport` in `internal/app/report.go`
- Init command registration in `cmd/root.go`

## Type Rename

`InitScript` → `ScriptEntry` across all files (types, merger, resolver, tests).

## State File

`ApplyState` records results for stages that ran. Skipped stages carry forward from previous state.

## Documentation Updates

- Remove `internal/docs/topics/init.md`
- Create `internal/docs/topics/scripts.md` — covers both `pre_apply` and `post_apply`
- Update `internal/docs/topics/commands.md` — remove init command, add `--stages` flag with valid values
- Update `internal/docs/topics/config.md` — replace `init` with `pre_apply` and `post_apply`
- Update `internal/docs/topics/examples.md` — replace init examples

## E2E Tests

- Remove/replace `e2e/suites/11-init-scripts.sh`
- New suite testing: both script types run in correct order, `--stages` filtering, variable resolution, fail-fast, interactive stdin
- Update `e2e/fixtures/setup-basic.sh` — rename `init:` to `post_apply:`, add `pre_apply:`

## Unit Tests

- Remove `init_test.go`
- Update `apply_test.go` — test pre_apply/post_apply in apply, test stages filtering
- Update `merger_test.go` — rename init tests for both fields
- Update `resolver_test.go` — rename init tests for both fields
