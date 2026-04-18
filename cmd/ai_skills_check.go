package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

// newAISkillsCheckCmd returns the "ai skills check" command.
func newAISkillsCheckCmd(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for available skill updates",
		Long:  "Checks all globally installed skills for available updates via npx skills check.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return application.AISkillsCheck()
		},
	}
}
