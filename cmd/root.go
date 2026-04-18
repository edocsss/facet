package cmd

import (
	"fmt"
	"os"

	"facet/internal/app"

	"github.com/spf13/cobra"
)

// NewRootCmd builds the full command tree and returns the root command.
func NewRootCmd(application *app.App) *cobra.Command {
	var configDir, stateDir string

	rootCmd := &cobra.Command{
		Use:     "facet",
		Short:   "Developer environment configuration manager",
		Version: "0.1.0",
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "Path to facet config repo (default: current directory)")
	rootCmd.PersistentFlags().StringVarP(&stateDir, "state-dir", "s", "", "Path to machine-local state directory (default: ~/.facet)")

	rootCmd.AddCommand(newApplyCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newDocsCmd())
	rootCmd.AddCommand(newScaffoldCmd(application, &configDir, &stateDir))
	rootCmd.AddCommand(newStatusCmd(application, &configDir, &stateDir))

	aiCmd := newAICmd()
	skillsCmd := newAISkillsCmd()
	skillsCmd.AddCommand(newAISkillsCheckCmd(application))
	skillsCmd.AddCommand(newAISkillsUpdateCmd(application))
	aiCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(aiCmd)

	return rootCmd
}

// resolveConfigDir returns the config directory path.
// Uses --config-dir flag if set, otherwise current working directory.
func resolveConfigDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return os.Getwd()
}

// resolveStateDir returns the state directory path.
// Uses --state-dir flag if set, otherwise ~/.facet/.
func resolveStateDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return home + "/.facet", nil
}
