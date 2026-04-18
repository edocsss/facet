package ai

// ResolvedPermissions holds agent-native permissions.
type ResolvedPermissions struct {
	Allow []string
	Deny  []string
}

// ResolvedSkill identifies a single skill to install.
type ResolvedSkill struct {
	Source string
	Name   string
}

// ResolvedMCP holds a fully resolved MCP configuration.
type ResolvedMCP struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// EffectiveAgentConfig holds the fully resolved config for a single agent.
type EffectiveAgentConfig struct {
	Permissions ResolvedPermissions
	Skills      []ResolvedSkill
	MCPs        []ResolvedMCP
}

// EffectiveAIConfig maps agent name to its resolved configuration.
type EffectiveAIConfig map[string]EffectiveAgentConfig
