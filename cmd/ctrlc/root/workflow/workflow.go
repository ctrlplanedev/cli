package workflow

import (
	"github.com/spf13/cobra"
)

func NewWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow <subcommand>",
		Short: "Manage workflows",
		Long:  `Commands for listing and triggering workflows.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewTriggerCmd())

	return cmd
}
