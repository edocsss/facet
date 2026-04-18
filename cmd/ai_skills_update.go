package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

// newAISkillsUpdateCmd returns the "ai skills update" command.
func newAISkillsUpdateCmd(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update all installed skills to latest versions",
		Long:  "Updates all globally installed skills to their latest versions via npx skills update.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return application.AISkillsUpdate()
		},
	}
}
