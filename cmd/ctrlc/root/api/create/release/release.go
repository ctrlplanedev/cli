package release

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewReleaseCmd() *cobra.Command {
	var versionFlag string
	var deploymentID string
	var metadata map[string]string

	cmd := &cobra.Command{
		Use:   "release [flags]",
		Short: "Create a new release",
		Long:  `Create a new release with the specified version and configuration.`,
		Example: heredoc.Doc(`
			# Create a new release
			$ ctrlc create release --version v1.0.0

			# Create a new release using Go template syntax
			$ ctrlc create release --version v1.0.0 --template='{{.status.phase}}'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			resp, err := client.CreateRelease(cmd.Context(), api.CreateReleaseJSONRequestBody{
				Version:      versionFlag,
				DeploymentId: deploymentID,
				Metadata:     &metadata,
			})
			if err != nil {
				return fmt.Errorf("failed to create release: %w", err)
			}

			return cliutil.HandleOutput(cmd, resp)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&versionFlag, "version", "", "Version of the release (required)")
	cmd.Flags().StringVar(&deploymentID, "deployment-id", "", "ID of the deployment (required)")
	cmd.Flags().StringToStringVar(&metadata, "metadata", make(map[string]string), "Metadata key-value pairs (e.g. --metadata key=value)")
	cmd.MarkFlagRequired("version")
	cmd.MarkFlagRequired("deployment-id")

	return cmd
}