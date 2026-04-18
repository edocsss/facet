# Apply Scripts

Pre-apply and post-apply scripts run shell commands during `facet apply`.

- `pre_apply`: runs after config deployment, before package installation
- `post_apply`: runs after package installation

## YAML Format

Define scripts in `base.yaml` and profile YAML files:

```yaml
pre_apply:
  - name: setup ssh keys
    run: ssh-keygen -t ed25519 -C "${facet:git.email}"

post_apply:
  - name: configure git
    run: git config --global user.email "${facet:git.email}"
  - name: setup editor plugins
    run: |
      export EDITOR_HOME="${facet:editor.home}"
      ./scripts/setup-editor.sh
```

Each entry has:

- `name`: human-readable label shown in the output
- `run`: shell command or multi-line script executed via `sh -c`

## Variable Resolution

`${facet:var.name}` references are resolved in the `run` field using the full
merged variable set (base + profile + `.local.yaml`). The `name` field is not resolved.

For external scripts, pass variables explicitly:

```yaml
pre_apply:
  - name: run python setup
    run: |
      export DB_URL="${facet:db.url}"
      python3 ./scripts/setup.py
```

## Merge Rules

Base scripts run first, then profile scripts are appended. No deduplication: if both
layers define a script with the same name, both run. `.local.yaml` can also contribute
scripts (merged as the third layer).

## Execution

Scripts run sequentially in order. If any script exits with a non-zero code, execution
stops immediately and `facet apply` returns an error.

Scripts run with the config directory as the working directory, so relative paths like
`./scripts/setup.sh` resolve against the config repo.

## Interactive Scripts

Scripts inherit the terminal's stdin, stdout, and stderr. This means scripts can
prompt for user input, display real-time progress, and work with tools that require a
TTY (like `ssh-keygen` or `gpg --gen-key`).

```yaml
pre_apply:
  - name: generate ssh key
    run: ssh-keygen -t ed25519 -C "${facet:git.email}"

post_apply:
  - name: login to registry
    run: docker login registry.acme.com
```

Output from scripts streams directly to the terminal as the script runs. If a script
fails, the exit code is reported — any error output will already be visible above the
facet summary.

## Stages

Scripts are part of the apply stage pipeline. Use `--stages` to control which stages
run:

```bash
facet apply work --stages pre_apply,packages
facet apply work --stages configs,post_apply
```

Valid stages (in execution order):

| Stage | Description |
|-------|-------------|
| `configs` | Deploy symlinks and templates |
| `pre_apply` | Run pre_apply scripts |
| `packages` | Install packages |
| `post_apply` | Run post_apply scripts |
| `ai` | Apply AI configuration |

Default (no `--stages` flag): all stages run.
