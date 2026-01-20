package release

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewReleaseCmd() *cobra.Command {
	var releaseID string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Get a release",
		Long:  "Get a release by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)
			resp, err := client.GetRelease(cmd.Context(), workspaceID.String(), releaseID)
			if err != nil {
				return fmt.Errorf("failed to get release: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&releaseID, "id", "i", "", "Release ID")
	cmd.MarkFlagRequired("id")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
