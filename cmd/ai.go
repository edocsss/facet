package cmd

import "github.com/spf13/cobra"

// newAICmd returns the top-level "ai" command group.
func newAICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ai",
		Short: "AI agent management commands",
	}
}
