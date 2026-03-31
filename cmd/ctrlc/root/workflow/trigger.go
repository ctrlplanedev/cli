package workflow

import (
	"fmt"
	"strings"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewTriggerCmd() *cobra.Command {
	var inputFlags []string

	cmd := &cobra.Command{
		Use:   "trigger <workflow-id>",
		Short: "Trigger a workflow run",
		Long:  `Trigger a workflow run with the given inputs.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowID := args[0]

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			inputs := make(map[string]interface{})
			for _, input := range inputFlags {
				key, value, found := strings.Cut(input, "=")
				if !found {
					return fmt.Errorf("invalid input format %q, expected key=value", input)
				}
				inputs[key] = value
			}

			body := api.CreateWorkflowRunJSONRequestBody{
				Inputs: inputs,
			}

			resp, err := client.CreateWorkflowRun(cmd.Context(), workspaceID.String(), workflowID, body)
			if err != nil {
				return fmt.Errorf("failed to trigger workflow: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Input key=value pair (can be specified multiple times)")

	return cmd
}
