package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newScaffoldCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold",
		Short: "Create a new facet config repository",
		Long:  "Creates a facet config repo in the current directory and initializes the state directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, err := resolveConfigDir(*configDir)
			if err != nil {
				return err
			}
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Scaffold(app.ScaffoldOpts{
				ConfigDir: cfgDir,
				StateDir:  stDir,
			})
		},
	}
}
