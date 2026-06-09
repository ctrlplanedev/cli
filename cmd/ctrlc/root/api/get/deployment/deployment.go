package deployment

import (
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewDeploymentCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Get a deployment",
		Long:  "Get a deployment by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewConnectClient(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID, err := client.GetWorkspaceID(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			resp, err := client.Deployment.GetDeploymentByName(cmd.Context(), connect.NewRequest(&apiv1.GetDeploymentByNameRequest{
				WorkspaceId: workspaceID.String(),
				Name:        name,
			}))
			if err != nil {
				return fmt.Errorf("failed to get deployment: %w", err)
			}

			return cliutil.HandleProtoOutput(cmd, resp.Msg)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Deployment name")
	cmd.MarkFlagRequired("name")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
