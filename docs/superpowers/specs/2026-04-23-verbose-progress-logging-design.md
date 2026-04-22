# Design: Verbose Progress Logging for `facet apply`

**Date:** 2026-04-23  
**Status:** Approved

## Problem

`facet apply` (and `facet apply --force`) runs silently through all stages — profile loading, unapply, config deployment, packages, scripts, AI — and prints nothing until the final summary report. When stages are slow (e.g. resolving a remote `extends`, installing packages), users have no indication of what is happening or which layer is being processed.

## Solution

Add a `--verbose` / `-v` global flag that enables inline progress logging during `facet apply`. When active, each stage prints a header before it runs, and each item within the stage prints an indented line as it is processed.

## Architecture

### Reporter layer

**`internal/common/reporter/reporter.go`**

- Add `verbose bool` field to `Reporter`.
- Add `SetVerbose(bool)` method — not on the `Reporter` interface; called only by the cmd layer after Cobra parses flags.
- Add `Progress(msg string)` method — prints `<msg>\n` only when `verbose` is true. No additional prefix; callers control formatting (stage headers are plain, item lines are indented with `  → `).

**`internal/app/interfaces.go`**

- Add `Progress(msg string)` to the `Reporter` interface. App calls this freely; the concrete reporter decides whether to emit it.

### Wiring

**`cmd/root.go`**

- Add `VerboseSetter` interface: `SetVerbose(bool)`.
- `NewRootCmd` gains a second parameter `vs VerboseSetter`.
- Add `--verbose` / `-v` persistent flag (bool, default false).
- Add `PersistentPreRunE` that calls `vs.SetVerbose(verbose)` so the flag value is applied before any subcommand runs.

**`main.go`**

- Pass the concrete `*reporter.Reporter` (`r`) as the `VerboseSetter` argument: `cmd.NewRootCmd(application, r)`.

### Progress logging in Apply

`internal/app/apply.go` adds `a.reporter.Progress(...)` calls at these points:

| Point in Apply | Stage message | Item messages |
|---|---|---|
| Before loading profile | `"Loading profile"` | — |
| Before resolving extends | `"Resolving extends"` | — |
| Before merging layers | `"Merging layers"` | — |
| Before unapply (when triggered) | `"Unapplying previous state"` | `"  → removing <target>"` per config |
| Before configs stage | `"Deploying configs"` | `"  → <target>"` per target |
| Before pre_apply stage | `"Running pre_apply scripts"` | `"  → <script.Name>"` per script |
| Before packages stage | `"Installing packages"` | `"  → <pkg.Name>"` per package (listed upfront before `InstallAll`) |
| Before post_apply stage | `"Running post_apply scripts"` | `"  → <script.Name>"` per script |
| Before AI stage | `"Applying AI configuration"` | — |

The existing `printApplyReport` summary at the end is unchanged and always prints regardless of verbose mode.

## Example output (`--verbose`)

```
Loading profile
Resolving extends
Merging layers
Unapplying previous state
  → removing ~/.zshrc
  → removing ~/.gitconfig
Deploying configs
  → ~/.zshrc
  → ~/.gitconfig
Running pre_apply scripts
  → bootstrap
Installing packages
  → git
  → vim
Running post_apply scripts
  → post-setup
Applying AI configuration

Applied profile: work

Packages
  ✓ git                 brew install git
  ✓ vim                 brew install vim

Configs
  ✓ ~/.zshrc            → configs/.zshrc            (symlink)
  ✓ ~/.gitconfig        → configs/.gitconfig         (symlink)
```

## Testing

### Unit tests (`internal/app/apply_test.go`)

- Add test verifying that `mockReporter` captures `Progress` messages for each stage when all stages run.
- The `mockReporter` always captures progress (verbose is a reporter concern, not an App concern); tests verify the messages are present.

### Reporter tests (`internal/common/reporter/reporter_test.go`)

- `Progress` prints when `verbose=true`.
- `Progress` is silent when `verbose=false` (default).
- `SetVerbose(true)` enables progress; `SetVerbose(false)` disables it.

### E2E (`e2e/suites/16-verbose-flag.sh`)

- `facet apply work --verbose` → output contains stage lines (`Deploying configs`, `Installing packages`, etc.)
- `facet apply work` (no flag) → output does NOT contain stage lines.
- `-v` short form works identically.

## Documentation updates

**`internal/docs/topics/commands.md`**

- Add `--verbose` / `-v` to the Global Flags table.
- Add usage example to the `facet apply` section.

**`README.md`**

- Add `--verbose` / `-v` to the apply examples block and global flags description.

**`docs/architecture/v1-design-spec.md`**

- Add `--verbose` flag to the CLI flags section and note the Reporter interface addition.
