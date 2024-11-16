package release

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewReleaseCmd() *cobra.Command {
	var versionFlag string
	var deploymentID []string
	var metadata map[string]string
	var configArray map[string]string
	var links map[string]string
	var createdAt string
	var name string

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

			var parsedTime *time.Time
			if createdAt != "" {
				t, err := time.Parse(time.RFC3339, createdAt)
				if err != nil {
					return fmt.Errorf("failed to parse created_at time: %w", err)
				}
				parsedTime = &t
			}

			if len(links) > 0 {
				linksJSON, err := json.Marshal(links)
				if err != nil {
					return fmt.Errorf("failed to marshal links: %w", err)
				}
				metadata["ctrlplane/links"] = string(linksJSON)
			}

			config := cliutil.ConvertConfigArrayToNestedMap(configArray)
			var response *http.Response
			for _, id := range deploymentID {
				resp, err := client.CreateRelease(cmd.Context(), api.CreateReleaseJSONRequestBody{
					Version:      versionFlag,
					DeploymentId: id,
					Metadata:     &metadata,
					CreatedAt:    parsedTime,
					Config:       &config,
					Name:         &name,
				})
				if err != nil {
					return fmt.Errorf("failed to create release: %w", err)
				}
				response = resp
			}

			return cliutil.HandleOutput(cmd, response)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&versionFlag, "version", "", "Version of the release (required)")
	cmd.Flags().StringArrayVar(&deploymentID, "deployment", []string{}, "IDs of the deployments (required, supports multiple)")
	cmd.Flags().StringToStringVar(&metadata, "metadata", make(map[string]string), "Metadata key-value pairs (e.g. --metadata key=value)")
	cmd.Flags().StringToStringVar(&configArray, "config", make(map[string]string), "Config key-value pairs with nested values (can be specified multiple times)")
	cmd.Flags().StringToStringVar(&links, "link", make(map[string]string), "Links key-value pairs (can be specified multiple times)")
	cmd.Flags().StringVar(&createdAt, "created-at", "", "Created at timestamp (e.g. --created-at 2024-01-01T00:00:00Z) for the release channel")
	cmd.Flags().StringVar(&name, "name", "", "Name of the release channel")

	cmd.MarkFlagRequired("version")
	cmd.MarkFlagRequired("deployment-id")

	return cmd
}
