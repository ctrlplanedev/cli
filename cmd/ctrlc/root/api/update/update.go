package update

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/update/deploymentversion"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/update/release"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/api/update/system"
	"github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <command>",
		Short: "Update resources",
		Long:  `Commands for updating resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(release.NewUpdateReleaseCmd())
	cmd.AddCommand(deploymentversion.NewUpdateDeploymentVersionCmd())
	cmd.AddCommand(system.NewUpdateSystemCmd())

	return cmd
}
