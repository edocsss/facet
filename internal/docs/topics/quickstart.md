# Quickstart

Set up facet from scratch in 4 steps.

## 1. Initialize the config repo

```bash
mkdir ~/dotfiles && cd ~/dotfiles
facet scaffold
```

This creates:

```text
~/dotfiles/
├── facet.yaml
├── base.yaml
├── profiles/
└── configs/
```

It also creates `~/.facet/.local.yaml` if it does not exist.

## 2. Edit `.local.yaml`

The file at `~/.facet/.local.yaml` holds machine-specific values such as secrets.
facet requires the file to exist before `facet apply` will run.

```yaml
vars:
  github_token: ghp_xxxxxxxxxxxx
```

If you have no secrets yet, keep the file empty or leave only comments in it.

## 3. Create a profile

Create `profiles/work.yaml`:

```yaml
extends: base

vars:
  git_email: you@company.com

packages:
  - name: docker
    install: brew install docker

configs:
  ~/.gitconfig: configs/work/.gitconfig
```

`extends: base` is required.

## 4. Apply

```bash
facet apply work
```

This loads `base.yaml`, your profile, and `.local.yaml`, merges them, resolves
variables, deploys config files, runs scripts, installs packages, applies any AI
settings, and writes state to `~/.facet/.state.json`.

Run `facet status` to inspect the applied state.

## Next Steps

Run `facet docs <topic>` to learn more:

- `facet docs config` for the full YAML format
- `facet docs ai` for AI agent configuration
- `facet docs examples` for complete working examples
