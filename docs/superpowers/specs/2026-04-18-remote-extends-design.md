# facet remote extends — Design Spec

## Summary

Allow a local profile to inherit its base layer from a git repository, local directory, or local `base.yaml` file via the `extends` field. The primary use case is a company-local profile that points at a personal shared base stored elsewhere, while still layering local `.local.yaml` secrets on top.

The first version stays intentionally narrow:

- the invoked profile is always local
- the remote source only supplies `base.yaml`
- git sources are cloned fresh on every `facet apply`
- no network-based tests
- docs and E2E coverage are required as part of the feature

## Design Decisions

### Local profile, external base
The profile selected by `facet apply <profile>` remains a local file under `profiles/`. `extends` answers one question only: where should facet load the base layer from?

Merge order becomes:

1. resolved base from `extends`
2. selected local profile
3. local `.local.yaml`

### `extends` is a locator string
`extends` no longer accepts only `base`. It becomes a raw locator string that can identify:

- a git repository over HTTPS
- a git repository over SSH
- a local directory containing `base.yaml`
- a local path directly to `base.yaml`

### Ref syntax uses `@`
Git locators may include an optional `@<ref>` suffix:

```yaml
extends: https://github.com/me/personal-dotfiles.git
extends: https://github.com/me/personal-dotfiles.git@main
extends: git@github.com:me/personal-dotfiles.git@v1.2.0
extends: git@github.com:me/personal-dotfiles.git@3f2c9f1
extends: /Users/me/personal-dotfiles
extends: /Users/me/personal-dotfiles/base.yaml
```

Rules:

- for git locators, `@<ref>` means branch, tag, or commit
- if no ref is present, use the remote default branch
- local paths are treated as paths and are not split on `@`
- local relative paths are resolved relative to the config directory passed to `facet apply`
- custom URL schemes are out of scope

Parser precedence:

- strings beginning with `/`, `./`, `../`, or `~` are local paths
- strings ending in `.yaml` with no URL scheme are local file paths
- everything else that matches git transport syntax is parsed as a git locator

### Repo root only
For git and local-directory locators, facet reads `base.yaml` from the repository or directory root. Subpath syntax is explicitly out of scope for v1.

### Fresh clone on every apply
Git-based extends are materialized fresh for each `facet apply` run.

- no persistent git cache
- no reused clone across applies
- the temporary clone is removed before command exit

This favors operational simplicity and zero leftover repo state over maximum speed.

### No auth management
Facet invokes git non-interactively and does not manage credentials, SSH prompts, or remote rewriting. If git cannot access the remote, the command fails and surfaces the git error.

---

## User Contract

### Supported `extends` forms

| Form | Example | Meaning |
|------|---------|---------|
| HTTPS git repo | `https://github.com/me/personal-dotfiles.git` | Clone repo, use remote default branch, read root `base.yaml` |
| HTTPS git repo with ref | `https://github.com/me/personal-dotfiles.git@main` | Clone repo, resolve `main`, read root `base.yaml` |
| SSH git repo with ref | `git@github.com:me/personal-dotfiles.git@v1.2.0` | Clone repo, resolve tag or commit, read root `base.yaml` |
| Local directory | `/Users/me/personal-dotfiles` | Read `/Users/me/personal-dotfiles/base.yaml` |
| Local file | `/Users/me/personal-dotfiles/base.yaml` | Read that file directly |

### Default branch behavior
If a git locator omits `@<ref>`, facet uses the remote repository's default branch. It does not guess `main` or `master`.

### Scope limits for v1

- no multi-hop extends
- no profile-to-profile inheritance
- no remote subpath selection
- no custom locator scheme
- no offline cache of prior remote clones

---

## Architecture

Keep this feature in the profile-loading path. Do not spread git-specific behavior across deploy, packages, or reporting.

### New parsing type

`internal/profile` gets a small parser for `extends`, for example:

```go
type ExtendsKind string

const (
    ExtendsGit  ExtendsKind = "git"
    ExtendsDir  ExtendsKind = "dir"
    ExtendsFile ExtendsKind = "file"
)

type ExtendsSpec struct {
    Raw     string
    Kind    ExtendsKind
    Locator string
    Ref     string
}
```

Responsibilities:

- parse the raw `extends` string
- classify it as `git`, `dir`, or `file`
- extract an optional ref for git locators

### Base resolver boundary

Add a resolver dependency at the app boundary in `internal/app/interfaces.go`, defined where it is consumed.

Example shape:

```go
type BaseResolveResult struct {
    BasePath string
    Cleanup  func() error
}

type BaseResolver interface {
    Resolve(spec profile.ExtendsSpec) (*BaseResolveResult, error)
}
```

Responsibilities:

- local file: return the file path with a no-op cleanup
- local directory: return `<dir>/base.yaml` with a no-op cleanup
- git repo: clone fresh into a temp dir, resolve the requested ref if present, return `<temp>/base.yaml`, provide cleanup that removes the temp dir

### App flow changes

`App.Apply` changes in this order:

1. load `facet.yaml`
2. load the selected local profile
3. validate that `extends` exists and parses
4. resolve `extends` into a concrete `base.yaml`
5. load that base config
6. load local `.local.yaml`
7. merge base, then profile, then `.local.yaml`
8. continue with existing validation, variable resolution, deployment, packages, scripts, and AI stages

The profile chosen on the CLI remains the local source of truth for machine-specific overrides.

---

## Git Materialization Flow

For git locators, use a fresh temporary clone per apply:

1. create a temp directory
2. clone the repo into it
3. if a ref is present, checkout that ref
4. if no ref is present, stay on the cloned remote default branch
5. load `<temp>/base.yaml`
6. remove the temp directory before `apply` returns

Implementation notes:

- favor the simplest fast path first, such as shallow clone when it works cleanly with the requested ref
- if ref handling makes shallow behavior unreliable for tags or commits, correctness wins
- cleanup should be registered immediately after temp creation so failures do not leak directories

The feature goal is operational simplicity first. Performance should be improved only within that model.

---

## Validation and Errors

### Profile validation

Profiles must still define `extends`, but validation changes from a hard-coded `base` check to a locator-parse check.

Fatal errors:

- missing `extends`
- malformed `extends`
- local file path does not exist
- local directory does not contain `base.yaml`
- git clone fails
- requested ref cannot be resolved
- resolved source does not contain root `base.yaml`
- `base.yaml` cannot be read or parsed

Cleanup failure should be reported as a warning, not as a config-merge failure.

### Non-goals

Facet does not:

- prompt for credentials
- retry with alternate auth methods
- normalize or rewrite user-provided remotes
- claim a remote branch is current without cloning it in that apply run

---

## Testing

### Unit tests

Add focused tests for:

- parsing HTTPS git locators with and without `@ref`
- parsing SSH git locators with `@branch`, `@tag`, and `@commit`
- parsing local directory paths
- parsing local file paths
- rejecting malformed or ambiguous inputs
- updated profile validation behavior

### App-level tests

Add or update tests so `Apply` verifies:

- base config is loaded from the resolved external source
- merge order is external base, then local profile, then `.local.yaml`
- git-based resolution cleanup runs even on error paths

### E2E tests

Add hermetic E2E coverage under `e2e/suites/` for:

- local directory extends
- local file extends
- git extends from a local test repo fixture
- explicit branch ref
- explicit tag ref
- explicit commit ref
- omitted ref using the repo default branch
- invalid ref failure
- missing remote `base.yaml` failure

The full git flow must be tested without network. Use local hermetic git repositories created by the fixture/test harness rather than external remotes.

---

## Documentation Requirements

This feature is not complete until user-facing docs are updated.

Implementation must update all affected locations:

- `internal/docs/topics/`
- `README.md`
- `docs/architecture/v1-design-spec.md`

Those updates must cover:

- new `extends` semantics
- supported locator forms
- merge order
- default-branch behavior
- examples for HTTPS, SSH, and local paths
- any new error expectations that users need to understand

---

## File Layout Impact

Expected touched areas:

```text
internal/profile/
  loader.go               # profile validation changes
  extends.go              # locator parsing
  extends_test.go         # parser tests

internal/app/
  interfaces.go           # BaseResolver interface
  apply.go                # resolve base before load/merge
  apply_test.go           # app-level coverage

internal/common/ or internal/profile/
  git resolver implementation and tests

e2e/suites/
  new numbered suite for remote/local extends

internal/docs/topics/
README.md
docs/architecture/v1-design-spec.md
```

Exact package placement for the git resolver should follow existing boundaries, but the interface belongs at the app consumption point.

---

## What This Does NOT Do

- does not make remote profiles first-class CLI targets
- does not support chaining one remote base into another
- does not add a refresh command or cache management
- does not support selecting a subdirectory within a repo
- does not perform network-based E2E tests
