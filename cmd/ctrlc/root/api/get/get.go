package get

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/get/deployments"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/get/environments"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/get/resources"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/get/systems"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <command>",
		Short: "Get resources",
		Long:  `Commands for getting resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(resources.NewResourcesCmd())
	cmd.AddCommand(deployments.NewDeploymentsCmd())
	cmd.AddCommand(systems.NewSystemsCmd())
	cmd.AddCommand(environments.NewEnvironmentsCmd())

	return cmd
}
