package workflow

import (
	"fmt"
	"strings"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewTriggerCmd() *cobra.Command {
	var inputFlags []string

	cmd := &cobra.Command{
		Use:   "trigger <id-or-slug>",
		Short: "Trigger a workflow run",
		Long:  `Trigger a workflow run with the given inputs. The argument is auto-detected as a UUID or slug.`,
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

			inputs := make(map[string]interface{})
			for _, input := range inputFlags {
				key, value, found := strings.Cut(input, "=")
				if !found {
					return fmt.Errorf("invalid input format %q, expected key=value", input)
				}
				if key == "" {
					return fmt.Errorf("invalid input format %q, empty key, expected key=value", input)
				}
				inputs[key] = value
			}

			if _, err := uuid.Parse(identifier); err == nil {
				resp, err := client.CreateWorkflowRun(cmd.Context(), workspaceID.String(), identifier, api.CreateWorkflowRunJSONRequestBody{Inputs: inputs})
				if err != nil {
					return fmt.Errorf("failed to trigger workflow: %w", err)
				}
				return cliutil.HandleResponseOutput(cmd, resp)
			}

			resp, err := client.CreateWorkflowRunBySlug(cmd.Context(), workspaceID.String(), identifier, api.CreateWorkflowRunBySlugJSONRequestBody{Inputs: inputs})
			if err != nil {
				return fmt.Errorf("failed to trigger workflow by slug: %w", err)
			}
			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Input key=value pair (can be specified multiple times)")

	return cmd
}
