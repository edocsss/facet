# Variables

## Syntax

```text
${facet:var_name}
```

Variables come from the merged `vars:` blocks across `base.yaml`, the selected
profile, and `.local.yaml`.

## Nested Variables

Nested maps use dot notation:

```yaml
vars:
  git:
    name: Sarah Chen
    email: sarah@work.com
  aws:
    region: us-east-1
```

Examples:

- `${facet:git.email}`
- `${facet:aws.region}`

## Where Variables Are Resolved

- Package install commands
- Config source paths on the right side of `configs:`
- Templated config file contents
- Script `run` fields (in `pre_apply` and `post_apply`)
- MCP `command`, `args`, and `env` values

## Where Variables Are Not Resolved

- Config target paths on the left side of `configs:`. Those use `~`, `$VAR`, and `${VAR}` environment expansion instead.
- Variable values themselves. facet does not do recursive substitution.

## Undefined Variables

Referencing an undefined variable is a fatal error.

```text
undefined variable: ${facet:db_url} — define it in .local.yaml or your profile's vars
```

## Rules

- No recursive resolution inside `vars`
- Merge order is `base.yaml` then profile then `.local.yaml`
- Later layers win on conflicts
- Keep secrets in `.local.yaml`
