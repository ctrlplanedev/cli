package workflow

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id-or-slug>",
		Short: "Get a workflow by UUID or slug",
		Long:  `Get a single workflow. The argument is auto-detected as a UUID or slug.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			client, err := api.NewAPIKeyClientWithResponses(viper.GetString("url"), viper.GetString("api-key"))
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID, err := client.GetWorkspaceID(cmd.Context(), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			if _, err := uuid.Parse(identifier); err == nil {
				resp, err := client.GetWorkflow(cmd.Context(), workspaceID.String(), identifier)
				if err != nil {
					return fmt.Errorf("failed to get workflow: %w", err)
				}
				return cliutil.HandleResponseOutput(cmd, resp)
			}

			resp, err := client.GetWorkflowBySlug(cmd.Context(), workspaceID.String(), identifier)
			if err != nil {
				return fmt.Errorf("failed to get workflow by slug: %w", err)
			}
			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	return cmd
}
