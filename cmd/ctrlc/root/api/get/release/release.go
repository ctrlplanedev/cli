package release

import (
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
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

			client, err := api.NewConnectClient(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID, err := client.GetWorkspaceID(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			resp, err := client.Release.GetRelease(cmd.Context(), connect.NewRequest(&apiv1.GetReleaseRequest{
				WorkspaceId: workspaceID.String(),
				ReleaseId:   releaseID,
			}))
			if err != nil {
				return fmt.Errorf("failed to get release: %w", err)
			}

			return cliutil.HandleProtoOutput(cmd, resp.Msg)
		},
	}

	cmd.Flags().StringVarP(&releaseID, "id", "i", "", "Release ID")
	cmd.MarkFlagRequired("id")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
