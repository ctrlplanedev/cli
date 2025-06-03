package policy

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewDeletePolicyCmd() *cobra.Command {
	var policyId string
	var workspaceId string
	var name string

	cmd := &cobra.Command{
		Use:   "policy [flags]",
		Short: "Delete a policy",
		Long:  `Delete a policy by specifying a policy ID or name.`,
		Example: heredoc.Doc(`
			$ ctrlc delete policy --id 123e4567-e89b-12d3-a456-426614174000
			$ ctrlc delete policy --name "My Policy" --workspace 123e4567-e89b-12d3-a456-426614174000
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if policyId == "" && name == "" {
				return fmt.Errorf("either --id or --name must be provided")
			}

			workspace := workspaceId
			if workspace == "" {
				workspace = viper.GetString("workspace")
			}

			if name != "" && workspace == "" {
				return fmt.Errorf("workspace is required when using --name")
			}

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			if policyId != "" {
				return fmt.Errorf("policy ID is not yet supported")
			}

			resp, err := client.DeletePolicyByName(cmd.Context(), uuid.MustParse(workspace), name)
			if err != nil {
				return fmt.Errorf("failed to delete policy: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVar(&policyId, "id", "", "Policy ID")
	cmd.Flags().StringVar(&name, "name", "", "Policy name")
	cmd.Flags().StringVar(&workspaceId, "workspace", "", "Workspace ID")

	return cmd
}
