package ai

// AIState tracks facet-managed AI configuration in .state.json.
type AIState struct {
	Skills      []SkillState               `json:"skills"`
	MCPs        []MCPState                 `json:"mcps"`
	Permissions map[string]PermissionState `json:"permissions"`
}

// SkillState records a managed skill and which agents it was installed for.
type SkillState struct {
	Source string   `json:"source"`
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
}

// MCPState records a managed MCP and which agents it was registered for.
type MCPState struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
}

// PermissionState records agent-native permission terms that were applied.
type PermissionState struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}
