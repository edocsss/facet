#!/bin/bash
# e2e/suites/17-named-skill-verification.sh - named skill post-install verification
SUITE_DIR="${SUITE_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$SUITE_DIR/helpers.sh"

setup_basic

mkdir -p "$HOME/dotfiles/profiles"
cat > "$HOME/dotfiles/profiles/work.yaml" << 'YAML'
extends: base
YAML

mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.agents"

# Test 1: Named skill that is in the lock - recorded in state, no warning.
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {
    "real-skill": {"source": "@org/skills"}
  }
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@org/skills"
      skills: [real-skill]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'real-skill'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].source' '@org/skills'
assert_file_not_contains "$HOME/.apply-out" "not found in the skill lock"
echo "  confirmed skill: recorded in state, no warning"

# Test 2: Named skill not in the lock - absent from state, warning in output.
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {}
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@org/skills"
      skills: [ghost-skill]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_json_field "$HOME/.facet/.state.json" '.ai.skills' 'null'
assert_file_contains "$HOME/.apply-out" "not found in the skill lock"
echo "  ghost skill: absent from state, warning emitted"

# Test 3: Unreadable/malformed lock - fallback records requested names, warning in output.
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills":
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code]
  permissions:
    claude-code:
      allow: [Read]
      deny: []
  skills:
    - source: "@org/skills"
      skills: [fallback-skill]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'fallback-skill'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].source' '@org/skills'
assert_file_contains "$HOME/.apply-out" "could not verify via skill lock"
echo "  malformed lock: requested skill recorded as fallback, warning emitted"

# Test 4: Pi is a supported npx skills agent name.
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {
    "pi-helper": {"source": "@org/pi-skills"}
  }
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [pi]
  skills:
    - source: "@org/pi-skills"
      skills: [pi-helper]
      agents: [pi]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_file_contains "$HOME/.mock-ai" "npx skills add @org/pi-skills --skill pi-helper -a pi -g -y"
assert_file_not_contains "$HOME/.apply-out" "no provider for agent \"pi\""
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'pi-helper'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].source' '@org/pi-skills'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].agents[0]' 'pi'
echo "  pi skill: installed using npx skills agent name pi"

# Test 5: Unscoped skills default to the supported skill agents, including pi.
cat > "$HOME/.agents/.skill-lock.json" << 'JSON'
{
  "version": 3,
  "skills": {
    "default-helper": {"source": "@org/default-skills"}
  }
}
JSON

cat > "$HOME/dotfiles/base.yaml" << 'YAML'
ai:
  agents: [claude-code, cursor, codex, pi]
  skills:
    - source: "@org/default-skills"
      skills: [default-helper]
YAML

: > "$HOME/.mock-ai"
facet -c "$HOME/dotfiles" -s "$HOME/.facet" apply work > "$HOME/.apply-out" 2>&1
assert_file_contains "$HOME/.mock-ai" "npx skills add @org/default-skills --skill default-helper -a claude-code -a codex -a cursor -a pi -g -y"
assert_file_not_contains "$HOME/.apply-out" "no provider for agent \"pi\""
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].name' 'default-helper'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].agents | length' '4'
assert_json_field "$HOME/.facet/.state.json" '.ai.skills[0].agents[3]' 'pi'
echo "  default skill agents: unscoped skills include pi"
