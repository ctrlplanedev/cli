package environment

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCreateEnvironmentCmd() *cobra.Command {
	var nameFlag string
	var system string
	var resourceSelector string
	var metadata map[string]string
	cmd := &cobra.Command{
		Use:   "environment [flags]",
		Short: "Create a new environment",
		Long:  `Create a new environment with the specified name and configuration.`,
		Example: heredoc.Doc(`
			# Create a new environment
			$ ctrlc create environment --name my-environment --system 00000000-0000-0000-0000-000000000000

			# Create a new environment with both release channels and deployment version channels
			$ ctrlc create environment --name my-environment --system 00000000-0000-0000-0000-000000000000 --release-channel 00000000-0000-0000-0000-000000000001 --deployment-version-channel 00000000-0000-0000-0000-000000000002

			# Create a new environment using Go template syntax
			$ ctrlc create environment --name my-environment --system 00000000-0000-0000-0000-000000000000 --template='{{.id}}'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			body := api.CreateEnvironmentJSONRequestBody{}
			body.Name = nameFlag
			body.SystemId = system
			body.Metadata = cliutil.StringMapPtr(metadata)

			if resourceSelector != "" {
				var parsedSelector map[string]interface{}
				if err := json.Unmarshal([]byte(resourceSelector), &parsedSelector); err != nil {
					return fmt.Errorf("failed to parse target selector: %w", err)
				}
				body.ResourceSelector = &parsedSelector
			}

			resp, err := client.CreateEnvironment(cmd.Context(), body)
			if err != nil {
				return fmt.Errorf("failed to create environment: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&nameFlag, "name", "n", "", "Name of the environment (required)")
	cmd.Flags().StringVarP(&system, "system", "s", "", "ID of the system (required)")
	cmd.Flags().StringVarP(&resourceSelector, "resource-selector", "f", "", "Resource selector as JSON string")
	cmd.Flags().StringToStringVarP(&metadata, "metadata", "m", make(map[string]string), "Metadata key-value pairs (e.g. --metadata key=value)")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("system")

	return cmd
}
