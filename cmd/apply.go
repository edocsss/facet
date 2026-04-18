package cmd

import (
	"facet/internal/app"

	"github.com/spf13/cobra"
)

func newApplyCmd(application *app.App, configDir, stateDir *string) *cobra.Command {
	var force, skipFailure, dryRun bool
	var stages string

	cmd := &cobra.Command{
		Use:   "apply <profile>",
		Short: "Apply a configuration profile",
		Long: `Loads, merges, and applies a configuration profile to this machine.

Stages run in this order: configs, pre_apply, packages, post_apply, ai.
Use --stages to run only specific stages (comma-separated).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, err := resolveConfigDir(*configDir)
			if err != nil {
				return err
			}
			stDir, err := resolveStateDir(*stateDir)
			if err != nil {
				return err
			}
			return application.Apply(args[0], app.ApplyOpts{
				ConfigDir:   cfgDir,
				StateDir:    stDir,
				Force:       force,
				SkipFailure: skipFailure,
				DryRun:      dryRun,
				Stages:      stages,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Unapply + apply, skip prompts for conflicting files")
	cmd.Flags().BoolVar(&skipFailure, "skip-failure", false, "Warn on config deploy failure instead of rolling back")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would happen without making changes")
	cmd.Flags().StringVar(&stages, "stages", "", "Comma-separated list of stages to run (default: all). Valid: configs, pre_apply, packages, post_apply, ai")

	return cmd
}
