package cmd

import "github.com/spf13/cobra"

// newAISkillsCmd returns the "ai skills" command group.
func newAISkillsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skills",
		Short: "Manage AI agent skills",
	}
}
