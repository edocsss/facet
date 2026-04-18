package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newStatusCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current facet status",
		Long:  "Displays the currently applied profile, packages, configs, and their validity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, _ := resolveConfigDir(*configDir) // best-effort for symlink source verification
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Status(app.StatusOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			})
		},
	}
}
