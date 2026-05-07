package deployment

import (
	"fmt"

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

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)
			resp, err := client.GetDeploymentByName(cmd.Context(), workspaceID.String(), name)
			if err != nil {
				return fmt.Errorf("failed to get deployment: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Deployment name")
	cmd.MarkFlagRequired("name")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
