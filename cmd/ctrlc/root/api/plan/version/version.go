package version

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewPlanVersionCmd() *cobra.Command {
	var tag string
	var workspace string
	var deploymentID string
	var metadata map[string]string
	var configArray map[string]string
	var links map[string]string
	var name string
	var jobAgentConfigFile string

	cmd := &cobra.Command{
		Use:   "version [flags]",
		Short: "Create a deployment plan for a version",
		Long:  `Create a deployment plan to preview what changes a version would produce across release targets.`,
		Example: heredoc.Doc(`
			# Plan a version
			$ ctrlc api plan version --tag v1.0.0 --workspace my-workspace --deployment 1234567890

			# Plan with GitHub metadata (enables PR comments)
			$ ctrlc api plan version --tag v1.0.0 --workspace my-workspace --deployment 1234567890 \
			  --metadata github/owner=wandb --metadata github/repo=deployments --metadata git/sha=abc123
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			if len(links) > 0 {
				linksJSON, err := json.Marshal(links)
				if err != nil {
					return fmt.Errorf("failed to marshal links: %w", err)
				}
				metadata["ctrlplane/links"] = string(linksJSON)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			config := cliutil.ConvertConfigArrayToNestedMap(configArray)

			var jobAgentConfig *map[string]any
			if jobAgentConfigFile != "" {
				data, err := os.ReadFile(jobAgentConfigFile)
				if err != nil {
					return fmt.Errorf("failed to read job agent config file: %w", err)
				}
				var cfg map[string]any
				if err := json.Unmarshal(data, &cfg); err != nil {
					return fmt.Errorf("failed to parse job agent config file: %w", err)
				}
				jobAgentConfig = &cfg
			}

			versionReq := api.DeploymentPlanVersion{
				Tag:            tag,
				Name:           &name,
				Metadata:       &metadata,
				Config:         &config,
				JobAgentConfig: jobAgentConfig,
			}

			resp, err := client.CreateDeploymentPlan(
				cmd.Context(),
				workspaceID.String(),
				deploymentID,
				api.CreateDeploymentPlanJSONRequestBody{
					Version: versionReq,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to create deployment plan: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag of the deployment version (required)")
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace (ID or slug) (required)")
	cmd.Flags().StringVarP(&deploymentID, "deployment", "d", "", "ID of the deployment (required)")
	cmd.Flags().StringToStringVarP(&metadata, "metadata", "m", make(map[string]string), "Metadata key-value pairs (e.g. --metadata key=value)")
	cmd.Flags().StringToStringVarP(&configArray, "config", "c", make(map[string]string), "Config key-value pairs with nested values")
	cmd.Flags().StringToStringVarP(&links, "link", "l", make(map[string]string), "Links key-value pairs")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name of the deployment version")
	cmd.Flags().StringVar(&jobAgentConfigFile, "job-agent-config-file", "", "Path to JSON file containing job agent configuration")

	mustMarkFlagRequired(cmd, "tag")
	mustMarkFlagRequired(cmd, "workspace")
	mustMarkFlagRequired(cmd, "deployment")

	return cmd
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("failed to mark flag required: %s: %v", name, err))
	}
}
