package resources

import (
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourcesCmd() *cobra.Command {
	var query string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Get resources",
		Long:  `Commands for getting resources.`,
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

			req := &apiv1.ListResourcesRequest{
				WorkspaceId: workspaceID.String(),
				Page: &apiv1.Page{
					Limit:  int32(limit),
					Offset: int32(offset),
				},
			}
			if query != "" {
				req.Selector = &query
			}

			resp, err := client.Resource.ListResources(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return fmt.Errorf("failed to get resources: %w", err)
			}

			return cliutil.HandleProtoOutput(cmd, resp.Msg)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "CEL filter")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Offset the results")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
