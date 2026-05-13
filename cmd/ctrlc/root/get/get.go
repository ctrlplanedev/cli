package get

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/get/resource"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/get/resources"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <command>",
		Short: "Get resources and other objects",
		Long:  `Commands for retrieving resources and other objects.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(resource.NewResourceCmd())
	cmd.AddCommand(resources.NewResourcesCmd())

	return cmd
}
