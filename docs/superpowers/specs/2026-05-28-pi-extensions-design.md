# Pi Extensions Design

## Goal

Model Pi coding-agent extensions as first-class Facet configuration so they can be installed and removed predictably, without abusing `packages` or `post_apply` scripts.

## User-facing configuration

Add a top-level `pi` block:

```yaml
pi:
  extensions:
    - pi-interactive-shell
    - pi-lens
    - pi-subagents
    - "@juicesharp/rpiv-btw"
    - "@juicesharp/rpiv-ask-user-question"
    - "@gotgenes/pi-session-tools"
```

`pi.extensions` is a list of Pi extension package identifiers passed directly to the Pi CLI.

## Apply behavior

Facet manages only extensions declared in `pi.extensions` and recorded in `.state.json` from prior applies.

On `facet apply`:

1. Install every declared extension with `pi extension install <name>`.
2. Remove previously managed extensions that are no longer declared with `pi extension remove <name>`.
3. Leave manually installed, unmanaged Pi extensions untouched.
4. Record successfully managed extensions in `.state.json`.

State shape:

```json
{
  "pi": {
    "extensions": ["pi-interactive-shell", "pi-lens"]
  }
}
```

## Stage order

Add a new `pi` apply stage:

```text
configs → pre_apply → packages → pi → post_apply → ai
```

`pi` runs after `packages` because the `pi` CLI is commonly installed as a package. It runs before `post_apply` so custom post-apply scripts can assume declared Pi extensions are already present.

`--stages` accepts `pi`; skipped `pi` preserves previous Pi state the same way skipped `ai` and `configs` preserve their state.

## Merge and resolve rules

`pi.extensions` merges by package name:

- base entries are kept first
- overlay entries add new extensions
- duplicate extension names are deduplicated
- `.local.yaml` can add local-only extensions

Extension names support `${facet:...}` substitution.

## Failure behavior

Pi extension operations are non-fatal per item, mirroring AI orchestration behavior:

- failed installs warn and are not recorded in state
- failed removes warn and do not stop the apply
- if the `pi` CLI is missing, each operation reports a clear warning

## Boundaries

This feature only manages Pi extensions. It does not manage Pi skills, prompts, themes, agent models, or Pi itself.
