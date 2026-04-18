# facet — Requirements Document

This document defines **what** facet does and **why**. Read it alongside `facet-implementation-plan.md` which defines **how** to build it.

When the two documents conflict, this one wins — it is the source of truth for behavior.

---

## 1. What Is facet

facet is a CLI tool that manages developer environment setup across multiple machines. You describe what a machine should look like in YAML files, and facet makes it real.

It handles three things no single existing tool covers together:

1. **Package installation** — CLI tools and GUI apps, delegated to OS-native package managers
2. **Config file deployment** — dotfiles, shell snippets, and templated configs
3. **AI coding tool configuration** — MCP servers, permissions, and settings for Claude Code, Cursor, Codex, and Copilot, written from one canonical definition into each tool's native format

These are composable via profiles: a base layer shared everywhere, plus per-machine profiles that override and extend.

---

## 2. Core Concepts

### 2.1 The facet Directory

All configuration lives in `~/.facet/`, which is a git repo the user manages.

```
~/.facet/
├── base.yaml              # required — shared foundation
├── profiles/
│   └── <name>.yaml        # one per machine/context
├── configs/
│   ├── <dotfiles>          # actual config files
│   ├── shell/              # .d snippets
│   └── <profile>/          # profile-specific overrides
└── .local.yaml             # optional, gitignored, machine-specific
```

The user can override this location via the `FACET_DIR` environment variable.

### 2.2 Profiles

A profile is a YAML file that describes one machine's desired state. Every profile must declare `extends: base`. Only one level of extends is supported — no chaining.

A profile can define:
- `vars` — key-value pairs for template substitution
- `packages` — CLI tools to install
- `packages_gui` — GUI applications to install
- `configs` — mapping of target paths to source files
- `ai` — MCP servers, permissions, and per-tool settings

### 2.3 Layer Merge Order

When you run `facet apply <profile>`, three layers are merged in order:

```
1. base.yaml           — loaded first
2. profiles/<name>.yaml — merged on top of base
3. .local.yaml          — merged on top (if exists)
```

Later layers win on conflicts. The specific merge rule depends on the field type — see Section 5.

### 2.4 Variables

Variables are simple key-value string pairs defined in `vars:` blocks. They are referenced in config files and in certain YAML values as `{{.var_name}}`.

Variables can hold secrets (API keys, database URLs). These should be placed in `.local.yaml` which is gitignored.

There is no separate secrets system. No environment variable interpolation syntax. Just `{{.var_name}}` everywhere, resolved from the merged vars map.

If a variable is referenced but not defined in any layer, facet must exit with a clear error message naming the undefined variable.

### 2.5 Config File Deployment

The `configs:` block maps target paths (relative to `$HOME`) to source files (relative to the facet directory).

```yaml
configs:
  .gitconfig: configs/.gitconfig
  .config/nvim: configs/nvim
  .facet.d/01-path.sh: configs/shell/base-path.sh
```

Three deployment strategies, auto-detected:

| Condition | Strategy | Behavior |
|-----------|----------|----------|
| Source is a directory | Symlink | `$HOME/<target>` → `<facet>/<source>` |
| Source file contains `{{` | Template | Render with vars, write copy to `$HOME/<target>` |
| Source file has no `{{` | Symlink | `$HOME/<target>` → `<facet>/<source>` |

The user never specifies which strategy — facet detects it.

Parent directories are created automatically (`mkdir -p`).

### 2.6 The .facet.d/ Directory Pattern

Config entries targeting `.facet.d/<filename>` are treated specially:

- On every `facet apply`, the `~/.facet.d/` directory is **cleared and rebuilt from scratch**
- Only the entries from the resolved config (base + profile + .local merged) are placed there
- Files are always symlinked (even if they contain `{{` — shell snippets should not need templates)
- Numeric prefixes control load order: `01-*` loads before `10-*`

The user's `.zshrc` (or `.bashrc`) sources everything in this directory:

```bash
for f in ~/.facet.d/*.sh(N); do source "$f"; done
```

This is a convention, not enforced by facet. The `.zshrc` the user puts in their `configs/` directory must contain this line for the pattern to work. Facet does not auto-inject it.

### 2.7 AI Tool Configuration

The `ai:` block has four sub-sections:

```yaml
ai:
  mcp:          # MCP servers — shared across all AI tools
  permissions:  # allow/deny lists — shared across all AI tools
  claude:       # Claude Code specific settings
  cursor:       # Cursor specific settings
  codex:        # Codex specific settings
  copilot:      # Copilot specific settings (future)
```

#### MCP Servers

Defined once, projected to every installed AI tool in its native format.

Two types:

```yaml
# stdio server (has command + args)
mcp:
  filesystem:
    command: npx
    args: [-y, "@anthropic/mcp-filesystem"]
    env:
      ALLOWED_DIRS: ~/code

# URL/SSE server (has url)
mcp:
  linear:
    url: https://mcp.linear.app/sse
```

#### Permissions

Universal allow/deny lists applied to all AI tools:

```yaml
permissions:
  allow:
    - Bash(npm test)
    - Bash(npm run build)
    - Read(~/code/work)
    - Write(~/code/work)
  deny:
    - Bash(rm -rf *)
    - Bash(sudo *)
    - Write(~/.ssh)
```

Permissions compose across layers — see Section 5 for merge rules.

#### Tool-Specific Settings

Each tool block holds settings unique to that tool. These are opaque to facet — they're passed through to the adapter.

```yaml
claude:
  model: sonnet
  skills:
    - configs/ai/code-review.md

cursor:
  rules: |
    Use TypeScript strict mode.
  rules_file: configs/ai/cursor-rules.md

codex:
  model: o3
  approval_mode: suggest
```

---

## 3. CLI Commands

### 3.1 `facet apply [profile]`

The primary command. Reads the config, merges layers, and makes the machine match the desired state.

**Arguments:**
- `profile` (required) — name of the profile to apply (matches `profiles/<name>.yaml`)

**Behavior:**
1. Load `base.yaml`
2. Load `profiles/<profile>.yaml`, verify it has `extends: base`
3. Load `.local.yaml` if it exists
4. Merge all three layers
5. Resolve all `{{.var_name}}` references — error on any unresolved
6. Detect OS and package manager
7. Install missing packages (skip already-installed)
8. Deploy config files (symlink or template)
9. Clear and rebuild `~/.facet.d/`
10. For each installed AI tool: write MCP servers + permissions + settings in native format
11. Write `.state.json` recording what was applied
12. Print a summary report

**Package behavior:**
- Packages are **additive only** — facet installs missing packages but never uninstalls
- Already-installed packages are skipped silently
- Failed package installs are reported but do not abort the run — other steps continue
- GUI packages (`packages_gui`) are only installed on non-headless systems (if no display, skip with warning)

**Config behavior:**
- If a target file exists and is NOT a symlink pointing into the facet configs directory, facet should **warn and skip** — don't silently overwrite the user's existing file
- If a target file exists and IS a facet-managed symlink, replace it
- Templated files are always overwritten (they're generated output)
- `~/.facet.d/` is always cleared and rebuilt — this is expected

**AI tool behavior:**
- Each AI tool adapter checks if the tool is installed (`IsInstalled()`)
- Uninstalled tools are skipped with a warning, not an error
- For installed tools: read the existing config file (if any), merge facet-managed keys, preserve user-managed keys, write back
- MCP server config is fully owned by facet — all entries are replaced on apply
- Tool settings (model, permissions, etc.) are merged — facet keys overwrite, unknown keys preserved

**Exit codes:**
- 0: success (warnings are OK)
- 1: fatal error (bad YAML, missing profile, unresolved vars)

### 3.2 `facet status`

Shows the current state of the machine relative to facet.

**Behavior:**
- Read `.state.json` to determine the active profile and when it was last applied
- Report: profile name, applied timestamp, package count, config count, AI tools configured
- Quick health checks: are all symlinks still valid? Are expected packages still installed?

**Output example:**
```
Profile:  acme (applied 2h ago)
Packages: 14 installed, 0 missing
Configs:  6 active (3 symlinked, 3 templated)
Shell:    4 snippets in .facet.d/
AI tools: claude ✓  cursor ✓  codex ✓  copilot ✗ (not installed)
```

**If no profile has been applied:** print a message suggesting `facet apply <profile>`.

### 3.3 `facet diff [profile]`

Preview what would change if a profile were applied.

**Arguments:**
- `profile` (optional) — profile to diff against. If omitted, use the currently active profile.

**Behavior:**
- Load and resolve the target profile
- Compare against the current machine state (installed packages, deployed configs, AI tool configs)
- Show what would be added, removed, or changed

**Output example:**
```
Switching from acme → personal:

Packages:
  + python3, poetry, deno
  - docker, awscli, terraform (still installed, not in personal)

Configs:
  ~ .gitconfig (email: sarah@acme-corp.com → sarah@hey.com)
  - .npmrc (removed)
  - .aws/config (removed)

Shell (.facet.d/):
  - 10-acme-env.sh, 11-acme-aliases.sh, 12-acme-aws.sh
  + 10-personal-env.sh, 11-personal-aliases.sh

AI:
  MCP: -postgres, -linear, -sentry, +memory
  Permissions: -2 allows, -2 denies
  Claude: model sonnet → opus
```

Packages shown with `-` are NOT uninstalled (facet doesn't uninstall) — the output clarifies they're "still installed, not in profile."

### 3.4 `facet init [--from <repo>]`

Initialize a new facet config directory.

**Without `--from`:**
1. Create `~/.facet/`
2. Create starter `base.yaml` with comments explaining the structure
3. Create empty `profiles/`, `configs/`, `configs/shell/` directories
4. Create `.gitignore` containing `.local.yaml` and `.state.json`
5. Run `git init`

**With `--from <repo>`:**
1. `git clone <repo> ~/.facet/`

**Error if `~/.facet/` already exists** — don't overwrite.

### 3.5 `facet doctor`

Diagnose common problems.

**Checks:**
- Is `~/.facet/` a directory?
- Is it a git repo?
- Does `base.yaml` exist and parse correctly?
- Is `.local.yaml` in `.gitignore`?
- For the active profile: do all referenced config source files exist?
- For the active profile: are all `{{.var_name}}` references resolvable?
- Are there broken symlinks in deployed configs?
- Is the OS-appropriate package manager available?
- Which AI tools are installed?

**Output:** one line per check, pass/fail/warning.

---

## 4. YAML Schema

### 4.1 base.yaml / profile / .local.yaml

All three file types share the same schema. The only difference is that profiles have an `extends` field.

```yaml
# All fields are optional except 'extends' in profiles

extends: base                      # profiles only, must be "base"

vars:
  <key>: <value>                   # string key-value pairs

packages:                          # CLI tools
  - <name>                         # simple: just the package name
  - name: <name>                   # advanced: with version or custom install
    version: "<version>"
    install: <command>             # string: single command
    install:                       # or map: per-OS commands
      macos: <command>
      linux: <command>

packages_gui:                      # GUI applications (same format as packages)
  - <name>

configs:
  <target>: <source>               # target relative to $HOME, source relative to facet dir

ai:
  mcp:
    <server_name>:
      command: <command>           # for stdio servers
      args: [<arg>, ...]
      env:
        <KEY>: <value>
      url: <url>                   # for URL/SSE servers (mutually exclusive with command)

  permissions:
    allow:
      - <permission_string>
    deny:
      - <permission_string>

  claude:
    <key>: <value>                 # passed through to Claude adapter
  cursor:
    <key>: <value>                 # passed through to Cursor adapter
  codex:
    <key>: <value>                 # passed through to Codex adapter
  copilot:
    <key>: <value>                 # passed through to Copilot adapter
```

### 4.2 Package Entry

A package entry can be either:
- A plain string: `ripgrep` (most common)
- An object with `name` and optionally `version`, `install`

```yaml
packages:
  - ripgrep                         # plain string
  - name: node                      # object with version
    version: "22"
  - name: claude-code               # object with custom install
    install: npm install -g @anthropic-ai/claude-code
  - name: lazydocker                # object with per-OS install
    install:
      macos: brew install lazydocker
      linux: go install github.com/jesseduffield/lazydocker@latest
```

### 4.3 Permission Strings

Permissions follow a `Type(pattern)` format:

```
Bash(<command_pattern>)     — shell command permission
Read(<path_pattern>)        — file/directory read permission
Write(<path_pattern>)       — file/directory write permission
```

Patterns can use `*` as a wildcard. Examples:
```
Bash(npm test)              — exact command
Bash(npm run *)             — any npm run subcommand
Bash(docker compose *)      — any docker compose subcommand
Read(~/code)                — read access to ~/code
Write(~/code/work/.env*)    — write access to .env files
```

Facet does not interpret or validate these patterns — they are passed through to the AI tool adapters as strings.

### 4.4 .state.json

Written by `facet apply`, read by `facet status` and `facet diff`.

```json
{
  "profile": "acme",
  "applied_at": "2025-03-15T10:30:00Z",
  "facet_version": "0.1.0",
  "packages_installed": ["tree"],
  "packages_skipped": ["git", "curl", "jq"],
  "configs_deployed": {
    ".gitconfig": "template",
    ".zshrc": "symlink",
    ".npmrc": "symlink",
    ".facet.d/01-path.sh": "symlink",
    ".facet.d/02-aliases.sh": "symlink",
    ".facet.d/10-acme-env.sh": "symlink",
    ".facet.d/11-acme-aliases.sh": "symlink"
  },
  "ai_tools": {
    "claude": {"mcp_count": 2, "allow_count": 4, "deny_count": 5},
    "cursor": {"mcp_count": 2, "rules": true},
    "codex": {"mcp_count": 2, "model": "o3"}
  }
}
```

---

## 5. Merge Rules

These are the exact rules for how layers combine. This section is authoritative — if the implementation differs, it's a bug.

### 5.1 vars

**Rule: shallow merge, last writer wins.**

```
base:    {git_name: "Sarah", editor: "nvim"}
profile: {git_email: "sarah@acme.com", editor: "cursor"}
.local:  {acme_db_url: "postgres://..."}

result:  {git_name: "Sarah", editor: "cursor", git_email: "sarah@acme.com", acme_db_url: "postgres://..."}
```

### 5.2 packages / packages_gui

**Rule: union by package name, last writer wins on version/install conflict.**

```
base:    [git, ripgrep, fd]
profile: [docker, node@22, fd]         # fd appears in both — no conflict (same name)
.local:  [some-local-tool]

result:  [git, ripgrep, fd, docker, node@22, some-local-tool]
```

If base has `fd` (plain string) and profile has `{name: fd, version: "9"}`, the profile's version wins.

### 5.3 configs

**Rule: shallow merge of target→source map, same target = later layer's source wins.**

```
base:    {.gitconfig: configs/.gitconfig, .zshrc: configs/.zshrc}
profile: {.gitconfig: configs/acme/.gitconfig, .npmrc: configs/acme/.npmrc}

result:  {.gitconfig: configs/acme/.gitconfig,   ← profile overrode base
          .zshrc: configs/.zshrc,                 ← inherited from base
          .npmrc: configs/acme/.npmrc}            ← added by profile
```

This is how profile-specific configs work — the profile points the same target (`.gitconfig`) to a different source file.

### 5.4 ai.mcp

**Rule: union by server name, last writer wins on conflict.**

```
base:    {filesystem: {...}}
profile: {postgres: {...}, sentry: {...}}
.local:  {my-experiment: {...}}

result:  {filesystem: {...}, postgres: {...}, sentry: {...}, my-experiment: {...}}
```

If base defines a server named `filesystem` and .local also defines `filesystem`, .local's definition replaces it entirely.

### 5.5 ai.permissions.allow

**Rule: union, deduplicated.**

```
base:    allow: [Read(~/.facet)]
profile: allow: [Bash(npm test), Bash(npm run build)]
.local:  allow: [Bash(make deploy-staging)]

result:  allow: [Read(~/.facet), Bash(npm test), Bash(npm run build), Bash(make deploy-staging)]
```

Order doesn't matter. Duplicates are removed.

### 5.6 ai.permissions.deny

**Rule: union, deduplicated, and immutable from parent layers.**

```
base:    deny: [Bash(rm -rf *), Bash(sudo *)]
profile: deny: [Bash(terraform apply)]

result:  deny: [Bash(rm -rf *), Bash(sudo *), Bash(terraform apply)]
```

**Immutability constraint:** a child layer (profile or .local) CANNOT remove a deny that was defined in a parent layer. Deny lists only grow, never shrink.

In practice, this means: if base denies `Bash(sudo *)`, there is no syntax to un-deny it in a profile. This is intentional — it prevents a profile from accidentally opening security holes defined in the shared base.

Implementation note: to enforce this, track which denies came from which layer during merge. If a later layer attempts to exclude a parent deny (future feature), reject it.

### 5.7 ai.claude / ai.cursor / ai.codex / ai.copilot

**Rule: shallow merge per tool, last writer wins on same key.**

```
base:    claude: {model: opus}
profile: claude: {model: sonnet, skills: [code-review.md]}

result:  claude: {model: sonnet, skills: [code-review.md]}
```

These are opaque maps — facet doesn't interpret the keys beyond passing them to the adapter.

---

## 6. AI Tool Adapters

Each AI tool adapter translates the canonical `ai:` config into the tool's native format.

### 6.1 Claude Code

**Config files written:**

| File | Contents |
|------|----------|
| `~/.claude.json` | MCP servers |
| `~/.claude/settings.json` | Permissions + tool settings |

**MCP format:**
```json
{
  "mcpServers": {
    "<name>": {
      "type": "stdio",
      "command": "<command>",
      "args": ["<args>"],
      "env": {"<KEY>": "<VALUE>"}
    }
  }
}
```

For URL servers: `"type": "url"` and `"url": "<url>"` instead of command/args.

**Settings format:**
```json
{
  "permissions": {
    "allow": ["<permission>", ...],
    "deny": ["<permission>", ...]
  }
}
```

Additional keys from `ai.claude` (like `model`) are merged into the settings object.

**Detection:** `claude` binary exists in `$PATH`.

### 6.2 Cursor

**Config files written:**

| File | Contents |
|------|----------|
| `~/.cursor/mcp.json` | MCP servers |
| `~/.cursor/rules/facet.mdc` | Cursor rules (if `rules` or `rules_file` set) |

**MCP format:**
```json
{
  "mcpServers": {
    "<name>": {
      "command": "<command>",
      "args": ["<args>"],
      "env": {"<KEY>": "<VALUE>"}
    }
  }
}
```

Note: Cursor MCP format does NOT include a `type` field.

For URL servers: `"url": "<url>"` instead of command/args.

**Rules:** If `ai.cursor.rules` is set (inline string), write it as `~/.cursor/rules/facet.mdc`. If `ai.cursor.rules_file` is set (path), copy that file.

**Detection:** `~/.cursor/` directory exists or `cursor` binary in `$PATH`.

### 6.3 Codex

**Config files written:**

| File | Contents |
|------|----------|
| `~/.codex/config.toml` | MCP servers + settings |

**Format:**
```toml
model = "<model>"
approval_mode = "<mode>"

[mcp_servers.<name>]
command = "<command>"
args = ["<args>"]

[mcp_servers.<name>.env]
KEY = "<value>"
```

For URL servers: `url = "<url>"` instead of command/args.

**Detection:** `codex` binary exists in `$PATH`.

### 6.4 Adapter Behavior Rules

These apply to ALL adapters:

1. **Read before write.** Always read the existing config file first. Preserve keys that facet doesn't manage.
2. **MCP is fully managed.** The `mcpServers` (or `mcp_servers`) section is entirely replaced on every apply. Facet owns this section.
3. **Settings are merged.** Facet-managed keys overwrite. Unknown keys are preserved.
4. **Consistent formatting.** Write JSON with 2-space indentation, sorted keys. Write TOML with standard formatting.
5. **Skip if not installed.** If `IsInstalled()` returns false, skip with a warning — not an error.
6. **Create directories.** If `~/.claude/` doesn't exist but `claude` is installed, create it.

---

## 7. Package Manager Behavior

### 7.1 OS Detection

| OS | How detected |
|---|---|
| macOS | `runtime.GOOS == "darwin"` |
| Ubuntu/Debian | `/etc/os-release` contains `ID=ubuntu` or `ID=debian` |
| Fedora/RHEL | `/etc/os-release` contains `ID=fedora` or `ID=rhel` |
| Arch | `/etc/os-release` contains `ID=arch` |

### 7.2 Package Manager Selection

| OS | CLI packages | GUI packages |
|---|---|---|
| macOS | `brew install` | `brew install --cask` |
| Ubuntu/Debian | `sudo apt-get install -y` | `sudo snap install` or `sudo apt-get install -y` |
| Arch | `sudo pacman -S --noconfirm` | `yay -S --noconfirm` |

### 7.3 Package Name Mapping

Some packages have different names across package managers. Facet maintains an internal mapping table:

```
canonical     brew         apt              pacman
─────────     ────         ───              ──────
fd            fd           fd-find          fd
ripgrep       ripgrep      ripgrep          ripgrep
bat           bat          bat              bat
```

When a package is not in the mapping table, facet uses the canonical name as-is and lets the package manager handle it. If it fails, facet reports the error with a suggestion to use a custom `install:` command.

### 7.4 Custom Install Commands

If a PackageEntry has an `install` field, facet runs that command directly instead of using the package manager:

```yaml
- name: claude-code
  install: npm install -g @anthropic-ai/claude-code
```

Per-OS custom commands:
```yaml
- name: lazydocker
  install:
    macos: brew install lazydocker
    linux: go install github.com/jesseduffield/lazydocker@latest
```

### 7.5 "Is Installed?" Check

Before installing, facet checks if a package is already present:

| Package Manager | Check |
|---|---|
| Homebrew | `brew list --formula <name>` or `brew list --cask <name>` exit code |
| apt | `dpkg -l <name>` exit code |
| pacman | `pacman -Q <name>` exit code |
| Custom install | `which <name>` exit code (check if binary exists in PATH) |

---

## 8. Acceptance Criteria

These are the specific, testable behaviors that facet must satisfy. Each maps to one or more E2E test suites.

### AC-1: Profile resolution

- Given `base.yaml` with packages `[git, ripgrep]` and `profiles/work.yaml` with packages `[docker]`, when I run `facet apply work`, the resolved package list is `[git, ripgrep, docker]`.
- Given `base.yaml` with configs `{.gitconfig: configs/.gitconfig}` and `profiles/work.yaml` with configs `{.gitconfig: configs/work/.gitconfig}`, when I run `facet apply work`, the `.gitconfig` source is `configs/work/.gitconfig` (profile overrides base for same target).
- Given `.local.yaml` with vars `{acme_db_url: postgres://...}` and a profile referencing `{{.acme_db_url}}` in an MCP env value, when I run `facet apply`, the resolved MCP env contains the actual URL.
- If a profile references `{{.undefined_var}}` and no layer defines it, `facet apply` exits with code 1 and names the undefined variable in the error message.

### AC-2: Config deployment

- A config file containing no `{{` is deployed as a symlink.
- A config file containing `{{.git_email}}` is rendered with the var value and written as a regular file (not a symlink).
- A directory source is always deployed as a symlink.
- After apply, `~/.facet.d/` contains exactly the `.facet.d/*` entries from the resolved config — no more, no less.
- `.facet.d/` entries are symlinks even if the source contains `{{`.
- Parent directories (e.g., `~/.config/`) are created automatically if they don't exist.

### AC-3: Profile switching

- When switching from profile A to profile B, configs that exist in A but not in B are **removed** (symlinks deleted, templated files deleted).
- `.facet.d/` is fully cleared and rebuilt with profile B's entries.
- AI tool configs reflect profile B's MCP servers and permissions — profile A's are gone.
- `.state.json` shows profile B as the active profile.

### AC-4: AI tool config generation

- MCP servers defined in `ai.mcp` appear in `~/.claude.json`, `~/.cursor/mcp.json`, and `~/.codex/config.toml` in each tool's native format.
- Permissions from `ai.permissions` appear in `~/.claude/settings.json`.
- Vars referenced in MCP env values (like `{{.acme_db_url}}`) are resolved to their actual values in the generated config files.
- If an AI tool is not installed, its config is skipped with a warning — not an error.
- URL-type MCP servers use `url` instead of `command`/`args`.

### AC-5: Permission stacking

- Allow lists from base + profile + .local are unioned.
- Deny lists from base + profile + .local are unioned.
- A deny defined in base cannot be removed by a profile or .local.
- The generated AI tool configs contain the full merged allow and deny lists.

### AC-6: Package installation

- Packages listed in the resolved config that are already installed are skipped.
- Packages that are not installed are installed via the OS-native package manager.
- A failed package install does not abort the entire apply — other steps continue.
- Custom install commands (`install: npm install -g ...`) are used instead of the package manager when specified.

### AC-7: Idempotency

- Running `facet apply <profile>` twice in a row produces identical filesystem state.
- The second apply installs zero packages (all already present).
- Generated AI tool configs are byte-identical between the two runs.

### AC-8: Shell integration

- After `facet apply acme`, running `zsh -c "source $HOME/.zshrc && echo $COMPANY"` returns `acme` (proving the full chain: .zshrc → .facet.d/ sourcing → snippet environment).
- After switching to personal, `$COMPANY` is unset and `$SIDE_PROJECTS` is set.
- Aliases from the active profile's snippets are available; aliases from the previous profile are not.

### AC-9: Error handling

- Applying a non-existent profile exits with code 1.
- Malformed YAML exits with code 1 and reports the file and line number.
- An unresolved `{{.var}}` exits with code 1 and names the variable.
- A missing config source file exits with code 1 and names the file.
- `facet apply` without arguments prints usage, not an error.
- `.local.yaml` not existing is fine — not an error.

### AC-10: Init

- `facet init` creates `~/.facet/` with `base.yaml`, `profiles/`, `configs/`, `.gitignore`, and a git repo.
- `.gitignore` contains `.local.yaml` and `.state.json`.
- `facet init` when `~/.facet/` already exists exits with code 1.
- `facet init --from <repo>` clones the repo to `~/.facet/`.

---

## 9. Non-Requirements (Explicit Exclusions)

These are things facet intentionally does NOT do. If someone asks for them, the answer is "not in scope."

| Feature | Why excluded |
|---|---|
| Multi-level extends | Complexity for minimal gain. One level is enough. |
| Package uninstallation | Too dangerous. Facet is additive only. |
| Git operations (push/pull/commit) | The user manages their own git repo. Facet doesn't wrap git. |
| Hooks / pre/post-apply scripts | Use a Makefile. Facet applies config, not arbitrary scripts. |
| Template logic (if/else/loops) | Just `{{.var_name}}` substitution. No conditionals. |
| Windows support | macOS and Linux only for now. |
| GUI / TUI | CLI only. Use your editor for YAML files. |
| Plugin system | Adapters are built-in. Adding one is a code change, not a plugin. |
| Remote apply via SSH | Run facet on the target machine. |
| Environment variable interpolation | No `${ENV_VAR}` syntax. Use vars in .local.yaml for machine-specific values. |
| Config file conflict resolution UI | Facet warns and skips. The user resolves manually. |

---

## 10. Glossary

| Term | Meaning |
|---|---|
| **Profile** | A YAML file describing one machine's desired state |
| **Base** | The shared config that all profiles extend |
| **Layer** | One of: base, profile, or .local — merged in order |
| **Vars** | Key-value pairs used for template substitution |
| **Config** | A dotfile or config file managed by facet |
| **Snippet** | A shell script fragment placed in `.facet.d/` |
| **.facet.d/** | Directory of shell snippets sourced by the user's shell |
| **MCP server** | A Model Context Protocol server used by AI coding tools |
| **Adapter** | Code that translates facet's canonical config into one AI tool's native format |
| **Facet directory** | `~/.facet/` (or `$FACET_DIR`) — the config repo |
| **Sandbox** | Temp directory used during E2E tests to isolate from real HOME |
