# Pi Extensions Design

## Goal

Model Pi coding-agent extensions as first-class Facet AI configuration so they
can be installed and removed predictably, without abusing `packages` or
`post_apply` scripts.

## User-facing configuration

Pi extensions live under the existing `ai` block:

```yaml
ai:
  pi:
    extensions:
      - pi-interactive-shell
      - pi-lens
      - pi-subagents
      - "@juicesharp/rpiv-btw"
      - "@juicesharp/rpiv-ask-user-question"
      - "@gotgenes/pi-session-tools"
```

`ai.pi.extensions` is a list of Pi extension package identifiers passed directly
to the Pi CLI. `ai.pi` may be used without `ai.agents` when no agent-scoped AI
configuration is present.

## Apply behavior

Pi extensions are reconciled as part of the existing `ai` apply stage. Facet
manages only extensions declared in `ai.pi.extensions` and recorded in
`.state.json` from prior applies.

On `facet apply` with the `ai` stage enabled:

1. Install every declared extension with `pi extension install <name>`.
2. Remove previously managed extensions that are no longer declared with
   `pi extension remove <name>`.
3. Leave manually installed, unmanaged Pi extensions untouched.
4. Record successfully managed extensions in `.state.json` under AI state.

State shape:

```json
{
  "ai": {
    "pi": {
      "extensions": ["pi-interactive-shell", "pi-lens"]
    }
  }
}
```

Legacy top-level state is still readable for migration:

```json
{
  "pi": {
    "extensions": ["pi-lens"]
  }
}
```

## Stage order

No separate Pi stage exists. The stage order remains:

```text
configs → pre_apply → packages → post_apply → ai
```

Use `--stages ai` to run AI configuration, including Pi extension
reconciliation. Skipping the `ai` stage preserves previous AI/Pi state.

## Merge and resolve rules

`ai.pi.extensions` merges by package name:

- base entries are kept first
- overlay entries add new extensions
- duplicate extension names are deduplicated
- `.local.yaml` can add local-only extensions

Extension names support `${facet:...}` substitution.

## Failure behavior

Pi extension operations are non-fatal per item, mirroring AI orchestration
behavior:

- failed installs warn and are not recorded in state
- failed removes warn and do not stop the apply
- if the `pi` CLI is missing, each operation reports a clear warning

## Boundaries

This feature only manages Pi extensions. It does not manage Pi skills, prompts,
themes, agent models, or Pi itself.
