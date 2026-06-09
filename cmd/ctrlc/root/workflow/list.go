package workflow

import (
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewListCmd() *cobra.Command {
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long:  `List all workflows in the workspace.`,
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

			if limit < 0 {
				return fmt.Errorf("invalid --limit %d, must be non-negative", limit)
			}
			if offset < 0 {
				return fmt.Errorf("invalid --offset %d, must be non-negative", offset)
			}

			resp, err := client.Workflow.ListWorkflows(cmd.Context(), connect.NewRequest(&apiv1.ListWorkflowsRequest{
				WorkspaceId: workspaceID.String(),
				Limit:       int32(limit),
				Offset:      int32(offset),
			}))
			if err != nil {
				return fmt.Errorf("failed to list workflows: %w", err)
			}

			return cliutil.HandleProtoOutput(cmd, resp.Msg)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Offset the results")

	return cmd
}
