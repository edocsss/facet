# Init Command Design Spec

## Overview

A new `facet init <profile>` command that runs post-install initialization scripts defined in profile YAML files. The user runs `facet apply <profile>` first to deploy configs and install packages, then `facet init <profile>` to run initialization scripts.

The existing `facet init` (config repo scaffolding) is renamed to `facet scaffold`.

## YAML Schema

New `init` key in `base.yaml` and profile YAML files:

```yaml
# base.yaml
init:
  - name: configure git
    run: git config --global user.email "${facet:git.email}"
  - name: setup gpg
    run: gpg --import ./keys/signing.asc

# profiles/work.yaml
init:
  - name: authenticate gcloud
    run: gcloud auth login --brief
  - name: setup docker
    run: |
      export REGISTRY="${facet:docker.registry}"
      ./scripts/docker-setup.sh
```

Each entry has:
- `name` (string, required): Human-readable label for reporting.
- `run` (string, required): Shell command or multi-line script to execute.

## Merge Rules

Two-layer merge: base scripts first, then profile scripts appended at the bottom.

- **No deduplication.** If base and profile both define a script with the same name, both run.
- **No conflict detection.** This is simple concatenation of the two `init` slices.
- **`.local.yaml` does not participate.** It remains for variables only.

This differs from packages (union by name) and configs (shallow merge by key).

## Variable Resolution

`${facet:var.name}` is resolved in the `run` field only. The `name` field is not resolved.

Variables come from the full merged var set (base + profile + .local.yaml), same as for packages and configs.

No automatic environment variable injection. No file templating. Users explicitly export what they need before calling external scripts:

```yaml
init:
  - name: run setup script
    run: |
      export DB_URL="${facet:db.url}"
      ./scripts/setup.sh
```

## CLI

```
facet init <profile> [--config-dir/-c <path>] [--state-dir/-s <path>]
```

- One required positional argument: profile name.
- Same global flags as `apply` (`--config-dir`, `--state-dir`).
- No `--force`, `--dry-run`, or `--skip-failure` flags.

## Execution Flow

1. Load `facet.yaml` (validate config dir).
2. Load `base.yaml`.
3. Load `profiles/<name>.yaml`, validate `extends: base`.
4. Load `.local.yaml` from state dir (for variable resolution).
5. Merge layers: base + profile for init scripts, base + profile + local for vars.
6. Resolve `${facet:var.name}` in all init script `run` strings.
7. Execute scripts sequentially via `sh -c "<command>"`.
8. Fail fast on non-zero exit code.
9. Print report.

**Working directory:** Scripts run with the config dir as the working directory, so relative paths like `./scripts/setup.sh` resolve against the config repo.

**No state tracking.** `init` does not write to `.state.json`. Running it again re-runs all scripts. Scripts should be idempotent by convention.

## Error Handling

- Steps 1-6 (loading, merging, resolving): fatal error, no scripts run.
- Step 7 (script execution): fail fast on non-zero exit code. Print which script failed (name + exit code), list remaining scripts that were skipped, exit non-zero.

## Reporter Output

**On success:**
```
Init  work

  ✓ configure git
  ✓ setup gpg
  ✓ authenticate gcloud
  ✓ setup docker

Init completed (4 scripts)
```

**On failure:**
```
Init  work

  ✓ configure git
  ✗ setup gpg (exit code 1)

  Skipped:
    - authenticate gcloud
    - setup docker

Init failed at "setup gpg"
```

## Scaffold Rename

The current `facet init` command (creates `facet.yaml`, `base.yaml`, `profiles/`, `configs/`, `.local.yaml`) is renamed to `facet scaffold`. All behavior stays the same, only the command name changes.

## Changes

### New/Modified Files

| File | Change |
|---|---|
| `internal/profile/types.go` | Add `InitScript` struct (`Name`, `Run` fields). Add `Init []InitScript` field to `FacetConfig`. |
| `internal/profile/merger.go` | Add concatenation merge for `Init` slices (base first, then profile appended). |
| `internal/profile/resolver.go` | Resolve `${facet:var.name}` in `InitScript.Run` fields. |
| `internal/app/interfaces.go` | Add `ScriptRunner` interface: `Run(ctx context.Context, command string, dir string) error`. Executes via `sh -c`, returns error on non-zero exit. |
| `internal/app/app.go` | Add `ScriptRunner` dependency to `App` struct and constructor. |
| `internal/app/scaffold.go` | Renamed from `init.go`. Method renamed from `Init` to `Scaffold`. `InitOpts` renamed to `ScaffoldOpts`. |
| `internal/app/init.go` | New file. `App.Init(InitOpts)` method implementing the init pipeline. |
| `cmd/scaffold_cmd.go` | Renamed from `init_cmd.go`. Command renamed from `init` to `scaffold`. |
| `cmd/init_cmd.go` | New file. `facet init <profile>` Cobra command. |
| `cmd/root.go` | Register new `init` command, update `scaffold` registration. |
| `cmd/docs.go` | Add init command documentation. |
| `main.go` | Wire `ScriptRunner` concrete implementation. |
| `docs/` | Update architecture docs with init command details. |

### Test Files

| File | Change |
|---|---|
| `internal/profile/merger_test.go` | Test init script concatenation merge. |
| `internal/profile/resolver_test.go` | Test variable resolution in init script run strings. |
| `internal/app/scaffold_test.go` | Renamed from `init_test.go`. |
| `internal/app/init_test.go` | New unit tests for init pipeline (mock ScriptRunner). |
| `e2e/` | E2E test: full init flow (apply then init with real scripts). |
| `e2e/` | E2E test: init failure (script exits non-zero, remaining skipped). |
| `e2e/` | E2E test: scaffold command still works under new name. |
