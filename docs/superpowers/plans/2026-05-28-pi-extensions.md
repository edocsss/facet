# Pi Extensions Implementation Plan

## Corrected requirement

Pi extension configuration belongs under the existing AI configuration and must
not introduce a separate apply stage.

Target schema:

```yaml
ai:
  pi:
    extensions:
      - pi-interactive-shell
      - pi-lens
```

Target stage behavior:

```text
configs → pre_apply → packages → post_apply → ai
```

`--stages ai` reconciles AI permissions, skills, MCPs, and Pi extensions.

## Implementation checklist

- Move `PiConfig` from top-level `FacetConfig` into `AIConfig.Pi`.
- Merge and resolve `ai.pi.extensions` alongside the rest of AI config.
- Relax AI validation so `ai.pi` can exist without `ai.agents` when there are no
  agent-scoped permissions, skills, or MCPs.
- Remove `pi` from valid apply stages and help text.
- Reuse the Pi manager from inside the `ai` apply stage.
- Store managed Pi extensions under `.state.json` as `ai.pi.extensions`.
- Continue reading legacy top-level `.state.json.pi` so existing installs can be
  reconciled and migrated.
- Preserve state-scoped removal: remove only extensions previously managed by
  Facet.
- Update docs and profile examples to use `ai.pi.extensions`.
- Add/update unit tests and E2E coverage for:
  - schema loading/merging/resolution
  - `${facet:...}` substitution errors under `ai.pi.extensions`
  - AI-only Pi config without `ai.agents`
  - state write/read under `ai.pi`
  - legacy top-level Pi state read
  - profile switch and same-profile removal reconciliation
  - `--stages packages` skips Pi reconciliation
  - `--stages ai` runs Pi reconciliation

## Verification

Run:

```sh
go test ./...
bash e2e/harness.sh
```
