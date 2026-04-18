package docs

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed topics/*.md
var topicFS embed.FS

// Topic holds a topic's name and description for the index.
type Topic struct {
	Name        string
	Description string
}

func allTopics() []Topic {
	return []Topic{
		{Name: "quickstart", Description: "Set up facet from scratch (start here)"},
		{Name: "config", Description: "YAML config format and file structure"},
		{Name: "variables", Description: "Variable substitution syntax and rules"},
		{Name: "packages", Description: "Package installation entries"},
		{Name: "scripts", Description: "Pre-apply and post-apply scripts"},
		{Name: "deploy", Description: "Config file deployment (symlink vs template)"},
		{Name: "ai", Description: "AI agent configuration (permissions, MCPs, skills)"},
		{Name: "merge", Description: "How base, profile, and .local layers combine"},
		{Name: "commands", Description: "CLI commands and flags reference"},
		{Name: "examples", Description: "Complete working config examples"},
	}
}

// Topics returns the ordered list of available topics.
func Topics() []Topic {
	return allTopics()
}

// Render returns the markdown content for a given topic name.
func Render(topic string) (string, error) {
	for _, t := range allTopics() {
		if t.Name != topic {
			continue
		}

		data, err := topicFS.ReadFile("topics/" + topic + ".md")
		if err != nil {
			return "", fmt.Errorf("failed to read topic %q: %w", topic, err)
		}
		return string(data), nil
	}

	return "", fmt.Errorf("unknown topic %q; run \"facet docs\" to see available topics", topic)
}

// Overview returns the text printed by `facet docs` with no topic argument.
func Overview() string {
	topics := allTopics()

	var b strings.Builder
	b.WriteString("# facet\n\n")
	b.WriteString("facet manages developer environment setup across machines. You describe packages,\n")
	b.WriteString("config files, and AI tool configuration in YAML profiles, and facet makes it real.\n\n")
	b.WriteString("## Usage\n\n")
	b.WriteString("  facet docs <topic>\n\n")
	b.WriteString("## Topics\n\n")

	maxLen := 0
	for _, t := range topics {
		if len(t.Name) > maxLen {
			maxLen = len(t.Name)
		}
	}

	for _, t := range topics {
		padding := strings.Repeat(" ", maxLen-len(t.Name))
		fmt.Fprintf(&b, "  %s%s   %s\n", t.Name, padding, t.Description)
	}

	b.WriteString("\nRun \"facet docs <topic>\" to read a specific topic.\n")
	return b.String()
}
