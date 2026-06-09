package deploymentversion

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const deploymentVersionStatusReady = "ready"

func safeConvertToDeploymentVersionStatus(status string) (string, error) {
	statusLower := strings.ToLower(status)
	switch statusLower {
	case "ready", "":
		return deploymentVersionStatusReady, nil
	case "building":
		return "building", nil
	case "failed":
		return "failed", nil
	}
	return "", fmt.Errorf("invalid deployment version status: %s", status)
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
	var jobAgentConfigFile string

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
			client, err := api.NewConnectClient(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			var parsedTime *timestamppb.Timestamp
			if createdAt != "" {
				t, err := time.Parse(time.RFC3339, createdAt)
				if err != nil {
					return fmt.Errorf("failed to parse created_at time: %w", err)
				}
				parsedTime = timestamppb.New(t)
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

			workspaceID, err := client.GetWorkspaceID(cmd.Context(), workspace)
			if err != nil {
				return err
			}

			config := cliutil.ConvertConfigArrayToNestedMap(configArray)

			var jobAgentConfig map[string]any
			if jobAgentConfigFile != "" {
				data, err := os.ReadFile(jobAgentConfigFile)
				if err != nil {
					return fmt.Errorf("failed to read job agent config file: %w", err)
				}
				if err := json.Unmarshal(data, &jobAgentConfig); err != nil {
					return fmt.Errorf("failed to parse job agent config file: %w", err)
				}
			}

			var response *apiv1.DeploymentVersion
			for _, id := range deploymentID {
				resp, err := client.Deployment.CreateDeploymentVersion(cmd.Context(), connect.NewRequest(&apiv1.CreateDeploymentVersionRequest{
					WorkspaceId:    workspaceID.String(),
					DeploymentId:   id,
					Tag:            tag,
					Metadata:       metadata,
					CreatedAt:      parsedTime,
					Config:         api.NewStruct(config),
					Name:           name,
					Status:         stat,
					JobAgentConfig: api.NewStruct(jobAgentConfig),
				}))
				if err != nil {
					return fmt.Errorf("failed to create deployment version: %w", err)
				}
				response = resp.Msg
			}

			return cliutil.HandleProtoOutput(cmd, response)
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
	cmd.Flags().StringVarP(&status, "status", "s", deploymentVersionStatusReady, "Status of the deployment version (one of: ready, building, failed)")
	cmd.Flags().StringVar(&message, "message", "", "Message of the deployment version")
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
