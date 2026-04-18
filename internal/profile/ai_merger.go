package profile

import (
	"sort"
	"strings"
)

// mergePermissions performs per-agent last-writer-wins on permissions maps.
// For each agent key, if overlay has it, overlay wins. Otherwise base is kept.
// Both nil → nil.
func mergePermissions(base, overlay map[string]*PermissionsConfig) map[string]*PermissionsConfig {
	if base == nil && overlay == nil {
		return nil
	}

	result := clonePermissionsMap(base, len(base)+len(overlay))
	if result == nil {
		result = make(map[string]*PermissionsConfig, len(base)+len(overlay))
	}
	for agent, perms := range overlay {
		result[agent] = clonePermissionsConfig(perms)
	}
	return result
}

func clonePermissionsMap(src map[string]*PermissionsConfig, capacity int) map[string]*PermissionsConfig {
	if src == nil {
		return nil
	}
	result := make(map[string]*PermissionsConfig, capacity)
	for agent, perms := range src {
		result[agent] = clonePermissionsConfig(perms)
	}
	return result
}

func clonePermissionsConfig(src *PermissionsConfig) *PermissionsConfig {
	if src == nil {
		return nil
	}
	return &PermissionsConfig{
		Allow: append([]string{}, src.Allow...),
		Deny:  append([]string{}, src.Deny...),
	}
}

// skillTuple holds a single normalized (source, skillName, agents) triple.
type skillTuple struct {
	source    string
	skillName string
	agents    []string
}

// mergeSkills unions two SkillEntry slices by (source, skill_name) tuple key.
// An entry with an empty Skills list means "all skills from this source" and is
// treated as an atomic unit: it replaces all individual tuples for that source
// from the base, and is itself replaced by specific entries from the overlay.
// Overlay tuples overwrite base tuples on conflict. Results are re-grouped by source.
// Both nil → nil.
func mergeSkills(base, overlay []SkillEntry) []SkillEntry {
	if base == nil && overlay == nil {
		return nil
	}

	// allSources tracks sources where the latest layer specified "all".
	type allEntry struct {
		agents []string
		order  int
	}
	allSources := make(map[string]allEntry)

	// Track insertion order by tuple key for specific skills.
	type entry struct {
		key   string // "source\x00skillName"
		tuple skillTuple
	}

	seen := make(map[string]int) // key → index in ordered
	ordered := []entry{}
	orderCounter := 0

	addSpecific := func(source, skillName string, agents []string) {
		key := source + "\x00" + skillName
		t := skillTuple{source: source, skillName: skillName, agents: agents}
		if idx, exists := seen[key]; exists {
			ordered[idx].tuple = t
		} else {
			seen[key] = len(ordered)
			ordered = append(ordered, entry{key: key, tuple: t})
		}
	}

	addAll := func(source string, agents []string) {
		// Remove any individual tuples for this source.
		for key, idx := range seen {
			if ordered[idx].tuple.source == source {
				ordered[idx].key = "" // mark for removal
				delete(seen, key)
			}
		}
		allSources[source] = allEntry{agents: agents, order: orderCounter}
		orderCounter++
	}

	addLayer := func(entries []SkillEntry) {
		for _, e := range entries {
			if len(e.Skills) == 0 {
				addAll(e.Source, e.Agents)
			} else {
				// Specific skills replace any "all" for this source.
				delete(allSources, e.Source)
				for _, skill := range e.Skills {
					addSpecific(e.Source, skill, e.Agents)
				}
			}
		}
	}

	addLayer(base)
	addLayer(overlay)

	// Collect "all" entries sorted by their insertion order.
	type allResult struct {
		source string
		agents []string
		order  int
	}
	var allResults []allResult
	for source, ae := range allSources {
		allResults = append(allResults, allResult{source: source, agents: ae.agents, order: ae.order})
	}
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].order < allResults[j].order
	})

	// Re-group specific tuples by (source, agents), preserving insertion order.
	type groupEntry struct {
		source string
		skills []string
		agents []string
	}
	groupIdx := make(map[string]int) // "source\x00sortedAgents" → index in groups
	groups := []groupEntry{}

	for _, oe := range ordered {
		if oe.key == "" {
			continue // marked for removal
		}
		t := oe.tuple
		key := t.source + "\x00" + sortedAgentsKey(t.agents)
		if idx, exists := groupIdx[key]; exists {
			groups[idx].skills = append(groups[idx].skills, t.skillName)
		} else {
			groupIdx[key] = len(groups)
			groups = append(groups, groupEntry{
				source: t.source,
				skills: []string{t.skillName},
				agents: t.agents,
			})
		}
	}

	result := make([]SkillEntry, 0, len(allResults)+len(groups))
	for _, ar := range allResults {
		result = append(result, SkillEntry{
			Source: ar.source,
			Agents: ar.agents,
		})
	}
	for _, g := range groups {
		result = append(result, SkillEntry{
			Source: g.source,
			Skills: g.skills,
			Agents: g.agents,
		})
	}
	return result
}

func sortedAgentsKey(agents []string) string {
	if len(agents) == 0 {
		return ""
	}
	sorted := append([]string{}, agents...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

// mergeMCPs unions two MCPEntry slices by name. Overlay wins on name conflict.
// Both nil → nil.
func mergeMCPs(base, overlay []MCPEntry) []MCPEntry {
	if base == nil && overlay == nil {
		return nil
	}

	byName := make(map[string]MCPEntry)
	var order []string

	for _, m := range base {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range overlay {
		if _, exists := byName[m.Name]; !exists {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}

	result := make([]MCPEntry, 0, len(order))
	for _, name := range order {
		result = append(result, byName[name])
	}
	return result
}

// mergeAI orchestrates the merge of two AIConfig values.
// agents: last writer wins. Both nil → nil. One nil → return the other.
func mergeAI(base, overlay *AIConfig) *AIConfig {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := &AIConfig{}

	// agents: last writer wins
	if overlay.Agents != nil {
		result.Agents = overlay.Agents
	} else {
		result.Agents = base.Agents
	}

	// permissions: per-agent last writer wins
	result.Permissions = mergePermissions(base.Permissions, overlay.Permissions)
	if len(result.Agents) > 0 && len(result.Permissions) > 0 {
		agentSet := make(map[string]struct{}, len(result.Agents))
		for _, agent := range result.Agents {
			agentSet[agent] = struct{}{}
		}
		for agent := range result.Permissions {
			if _, ok := agentSet[agent]; !ok {
				delete(result.Permissions, agent)
			}
		}
	}

	// skills: union by (source, skill) tuple
	result.Skills = mergeSkills(base.Skills, overlay.Skills)

	// mcps: union by name
	result.MCPs = mergeMCPs(base.MCPs, overlay.MCPs)

	return result
}
