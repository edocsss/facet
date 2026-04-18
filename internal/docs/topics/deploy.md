# Config File Deployment

## The `configs` Block

`configs:` maps target paths on your machine to source paths inside the config repo.

```yaml
configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.config/nvim: configs/nvim
  ~/.zshrc: configs/.zshrc
```

## Target Path Expansion

Target paths support environment expansion:

- `~` expands to your home directory
- `$VAR` and `${VAR}` expand from the OS environment

After expansion, the target path must be absolute.

## Source Path Rules

Source paths are relative to the config repo. Absolute paths or paths that escape the
repo with `../` are rejected.

Source paths can contain `${facet:...}` variables, which are resolved before use.

## Deploy Strategy

facet decides the strategy automatically:

| Source | Contains `${facet:` | Strategy |
|--------|---------------------|----------|
| File | No | Symlink |
| File | Yes | Template |
| Directory | N/A | Symlink |

You do not configure the strategy manually.

## Existing Target Behavior

1. Correct symlink already exists: no-op
2. Wrong symlink exists: remove and recreate it
3. Existing file or directory managed by facet: replace it
4. Existing file or directory not managed by facet: return an error unless `--force` is set
5. Missing target: create parent directories and deploy it

## Template Behavior

Templated files are rendered with `${facet:...}` substitution and written as regular
files. They are rewritten on each apply.

## Profile Switching

When switching profiles, facet removes configs that were managed by the previous
profile but are no longer part of the new one.
