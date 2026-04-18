# Examples

A complete working configuration with packages, config files, variables, and AI
settings.

## `base.yaml`

```yaml
vars:
  git_name: Sarah Chen

packages:
  - name: git
    install: brew install git
  - name: ripgrep
    check: which rg
    install: brew install ripgrep
  - name: fd
    check:
      macos: which fd
      linux: which fdfind
    install:
      macos: brew install fd
      linux: sudo apt-get install -y fd-find

configs:
  ~/.gitconfig: configs/.gitconfig
  ~/.zshrc: configs/.zshrc

ai:
  agents:
    - claude-code
    - cursor
  permissions:
    allow:
      - read
      - edit
    deny:
      - bash
  mcps:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

## `profiles/work.yaml`

```yaml
extends: ./base.yaml

vars:
  git_email: sarah@acme.com

packages:
  - name: docker
    install: brew install docker
  - name: awscli
    install: brew install awscli

configs:
  ~/.gitconfig: configs/work/.gitconfig
  ~/.npmrc: configs/work/.npmrc

ai:
  mcps:
    - name: postgres
      command: npx
      args: ["-y", "@anthropic/mcp-postgres"]
      env:
        DATABASE_URL: ${facet:db_url}
      agents:
        - claude-code
  skills:
    - source: "@acme/dev-skills"
      skills:
        - deploy-helper
        - code-review
```

## `profiles/remote-work.yaml`

```yaml
extends: git@github.com:acme/shared-dotfiles.git@main

configs:
  ~/.gitconfig: configs/work/.gitconfig
```

Configs inherited from a git-based base are materialized into place. Local profile configs still use normal symlink/template behavior.

## Pre-apply scripts in `base.yaml`

```yaml
pre_apply:
  - name: setup ssh keys
    run: ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
```

## Post-apply scripts in `profiles/work.yaml`

```yaml
post_apply:
  - name: authenticate gcloud
    run: |
      export PROJECT="${facet:gcp_project}"
      gcloud config set project "$PROJECT"
```

## `~/.facet/.local.yaml`

```yaml
vars:
  db_url: postgres://sarah:secret@localhost:5432/acme
  github_token: ghp_xxxxxxxxxxxx
```

## What Happens On `facet apply work`

- `vars` are merged across all three layers
- Packages from base and profile are combined by name
- `~/.gitconfig` uses `configs/work/.gitconfig` because the profile overrides base
- `~/.zshrc` still comes from `configs/.zshrc`
- The `filesystem` MCP applies to both agents
- The `postgres` MCP applies only to `claude-code`
- Skills from `@acme/dev-skills` are installed for all configured agents because the entry has no `agents` filter

## What Happens During `facet apply work` (Scripts)

- Pre-apply scripts from base and profile run after configs are deployed
- Post-apply scripts run after packages are installed
- `${facet:...}` variables are resolved in the `run` fields
- Scripts run sequentially; any failure stops execution
- Use `--stages pre_apply,post_apply` to run only scripts
