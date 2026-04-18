package cmd

import (
	"fmt"

	"facet/internal/docs"

	"github.com/spf13/cobra"
)

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs [topic]",
		Short: "Show documentation for AI agents",
		Long:  "Prints markdown documentation about facet's configuration format and capabilities.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), docs.Overview())
				return nil
			}

			content, err := docs.Render(args[0])
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), content)
			return nil
		},
	}
}
