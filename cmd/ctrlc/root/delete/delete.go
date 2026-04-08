package delete

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/delete/resource"
	"github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <command>",
		Short: "Delete resources and other objects",
		Long:  `Commands for deleting resources and other objects.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(resource.NewResourceCmd())

	return cmd
}
