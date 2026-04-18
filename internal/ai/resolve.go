package ai

import "facet/internal/profile"

// Resolve takes a merged AIConfig and produces per-agent effective configs.
// Permissions are already in agent-native terms.
// Returns nil if cfg is nil.
func Resolve(cfg *profile.AIConfig) EffectiveAIConfig {
	if cfg == nil {
		return nil
	}

	result := make(EffectiveAIConfig, len(cfg.Agents))

	for _, agent := range cfg.Agents {
		// Step 1: look up per-agent permissions
		perms := ResolvedPermissions{}
		if agentPerms, ok := cfg.Permissions[agent]; ok && agentPerms != nil {
			perms.Allow = append([]string{}, agentPerms.Allow...)
			perms.Deny = append([]string{}, agentPerms.Deny...)
		}

		// Step 2: filter skills and flatten each SkillEntry into ResolvedSkills.
		// An entry with empty Skills means "all from source" — emit a single
		// ResolvedSkill with an empty Name as a sentinel.
		// When a skill entry has no explicit agents, only DefaultSkillAgents
		// are targeted to keep the home directory clean.
		var skills []ResolvedSkill
		for _, entry := range cfg.Skills {
			if skillAgentIncluded(agent, entry.Agents) {
				if len(entry.Skills) == 0 {
					skills = append(skills, ResolvedSkill{
						Source: entry.Source,
						Name:   "",
					})
				} else {
					for _, skillName := range entry.Skills {
						skills = append(skills, ResolvedSkill{
							Source: entry.Source,
							Name:   skillName,
						})
					}
				}
			}
		}

		// Step 3: filter MCPs
		var mcps []ResolvedMCP
		for _, entry := range cfg.MCPs {
			if agentIncluded(agent, entry.Agents) {
				argsCopy := append([]string{}, entry.Args...)
				envCopy := make(map[string]string, len(entry.Env))
				for k, v := range entry.Env {
					envCopy[k] = v
				}
				mcps = append(mcps, ResolvedMCP{
					Name:    entry.Name,
					Command: entry.Command,
					Args:    argsCopy,
					Env:     envCopy,
				})
			}
		}

		result[agent] = EffectiveAgentConfig{
			Permissions: perms,
			Skills:      skills,
			MCPs:        mcps,
		}
	}

	return result
}

// DefaultSkillAgents is the set of agents that receive skills when a skill
// entry does not specify an explicit agents list. This keeps the user's home
// directory clean by not creating skill folders for every possible agent.
var DefaultSkillAgents = map[string]bool{
	"claude-code": true,
	"cursor":      true,
	"codex":       true,
}

// agentIncluded returns true if itemAgents is empty (meaning all agents)
// or contains the given agent name.
func agentIncluded(agent string, itemAgents []string) bool {
	if len(itemAgents) == 0 {
		return true
	}
	for _, a := range itemAgents {
		if a == agent {
			return true
		}
	}
	return false
}

// skillAgentIncluded returns true if the agent should receive this skill.
// When itemAgents is empty, only DefaultSkillAgents are included.
func skillAgentIncluded(agent string, itemAgents []string) bool {
	if len(itemAgents) == 0 {
		return DefaultSkillAgents[agent]
	}
	for _, a := range itemAgents {
		if a == agent {
			return true
		}
	}
	return false
}
