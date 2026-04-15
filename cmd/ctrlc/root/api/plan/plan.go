package plan

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/plan/version"
	"github.com/spf13/cobra"
)

func NewPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan <command>",
		Short: "Plan resources",
		Long:  `Commands for creating deployment plans.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(version.NewPlanVersionCmd())

	return cmd
}
