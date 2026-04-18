package profile

// AIConfig holds the AI tooling configuration for a facet profile.
type AIConfig struct {
	Agents      []string                      `yaml:"agents"`
	Permissions map[string]*PermissionsConfig `yaml:"permissions,omitempty"`
	Skills      []SkillEntry                  `yaml:"skills,omitempty"`
	MCPs        []MCPEntry                    `yaml:"mcps,omitempty"`
}

// PermissionsConfig defines allow/deny lists for AI agent tool permissions.
type PermissionsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// SkillEntry describes a set of skills to load from a source, optionally
// scoped to specific agents.
type SkillEntry struct {
	Source string   `yaml:"source"`
	Skills []string `yaml:"skills"`
	Agents []string `yaml:"agents,omitempty"`
}

// MCPEntry describes a Model Context Protocol server to configure, optionally
// scoped to specific agents.
type MCPEntry struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Agents  []string          `yaml:"agents,omitempty"`
}
