# Pi Extensions

The `pi:` block manages Pi coding-agent extensions during `facet apply`.

```yaml
pi:
  extensions:
    - pi-interactive-shell
    - pi-lens
    - pi-subagents
    - "@juicesharp/rpiv-btw"
    - "@gotgenes/pi-session-tools"
```

## Behavior

Facet installs every declared extension with:

```sh
pi extension install <name>
```

Facet removes only extensions that were previously managed by Facet and are no
longer declared in the resolved config:

```sh
pi extension remove <name>
```

Manually installed Pi extensions are left untouched.

## Stage

Pi extensions run in the `pi` apply stage, after `packages` and before
`post_apply`:

```text
configs → pre_apply → packages → pi → post_apply → ai
```

Use `--stages pi` to run only Pi extension reconciliation.
