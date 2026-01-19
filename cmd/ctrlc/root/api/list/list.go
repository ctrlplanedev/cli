package list

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/list/deployments"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/list/environments"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/list/systems"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <command>",
		Short: "List types of resources in ctrlplane",
		Long:  `Command for listing resources int ctrlplane ie: systems|environments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(systems.NewListSystemsCmd())
	cmd.AddCommand(environments.NewListEnvironmentsCmd())
	cmd.AddCommand(deployments.NewListDeploymentsCmd())

	return cmd
}
