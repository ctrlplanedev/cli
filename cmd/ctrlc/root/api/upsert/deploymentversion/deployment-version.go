package deploymentversion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func safeConvertToDeploymentVersionStatus(status string) (*api.DeploymentVersionStatus, error) {
	statusLower := strings.ToLower(status)
	if statusLower == "ready" || statusLower == "" {
		s := api.DeploymentVersionStatusReady
		return &s, nil
	}
	if statusLower == "building" {
		s := api.DeploymentVersionStatusBuilding
		return &s, nil
	}
	if statusLower == "failed" {
		s := api.DeploymentVersionStatusFailed
		return &s, nil
	}
	return nil, fmt.Errorf("invalid deployment version status: %s", status)
}

func NewUpsertDeploymentVersionCmd() *cobra.Command {
	var tag string
	var workspace string
	var deploymentID []string
	var metadata map[string]string
	var configArray map[string]string
	var links map[string]string
	var createdAt string
	var name string
	var status string
	var message string

	cmd := &cobra.Command{
		Use:   "version [flags]",
		Short: "Upsert a deployment version",
		Long:  `Upsert a deployment version with the specified tag and configuration.`,
		Example: heredoc.Doc(`
			# Upsert a deployment version
			$ ctrlc upsert version --tag v1.0.0 --workspace 00000000-0000-0000-0000-000000000000 --deployment 1234567890

			# Upsert a deployment version using Go template syntax
			$ ctrlc upsert version --tag v1.0.0 --workspace my-workspace --deployment 1234567890 --template='{{.status.phase}}'

			# Upsert a new version for multiple deployments
			$ ctrlc upsert version --tag v1.0.0 --workspace my-workspace --deployment 1234567890 --deployment 0987654321
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

			stat, err := safeConvertToDeploymentVersionStatus(status)
			if err != nil {
				return fmt.Errorf("failed to convert deployment version status: %w", err)
			}
			if stat == nil {
				s := api.DeploymentVersionStatusReady
				stat = &s
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			config := cliutil.ConvertConfigArrayToNestedMap(configArray)
			var response *http.Response
			for _, id := range deploymentID {
				resp, err := client.CreateDeploymentVersion(cmd.Context(), workspaceID.String(), id, api.CreateDeploymentVersionJSONRequestBody{
					Tag:          tag,
					Metadata:     &metadata,
					CreatedAt:    parsedTime,
					Config:       &config,
					Name:         name,
					Status:       *stat,
				})
				if err != nil {
					return fmt.Errorf("failed to create deployment version: %w", err)
				}
				response = resp
			}

			return cliutil.HandleResponseOutput(cmd, response)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag of the deployment version (required)")
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace (ID or slug) of the workspace (required)")
	cmd.Flags().StringArrayVarP(&deploymentID, "deployment", "d", []string{}, "IDs of the deployments (required, supports multiple)")
	cmd.Flags().StringToStringVarP(&metadata, "metadata", "m", make(map[string]string), "Metadata key-value pairs (e.g. --metadata key=value)")
	cmd.Flags().StringToStringVarP(&configArray, "config", "c", make(map[string]string), "Config key-value pairs with nested values (can be specified multiple times)")
	cmd.Flags().StringToStringVarP(&links, "link", "l", make(map[string]string), "Links key-value pairs (can be specified multiple times)")
	cmd.Flags().StringVarP(&createdAt, "created-at", "r", "", "Created at timestamp (e.g. --created-at 2024-01-01T00:00:00Z) for the deployment version")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name of the deployment version")
	cmd.Flags().StringVarP(&status, "status", "s", string(api.DeploymentVersionStatusReady), "Status of the deployment version (one of: ready, building, failed)")
	cmd.Flags().StringVar(&message, "message", "", "Message of the deployment version")

	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("workspace")
	cmd.MarkFlagRequired("deployment")

	return cmd
}
