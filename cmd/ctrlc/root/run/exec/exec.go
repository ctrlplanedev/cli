package exec

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type JobAgentType string

const (
	JobAgentTypeLinux   JobAgentType = "exec-linux"
	JobAgentTypeWindows JobAgentType = "exec-windows"
)

func NewRunExecCmd() *cobra.Command {
	var name string
	var jobAgentType string

	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute commands directly when a job is received",
		Example: heredoc.Doc(`
			$ ctrlc run exec --name "my-script-agent" --workspace 123e4567-e89b-12d3-a456-426614174000 
			$ ctrlc run exec --name "my-script-agent" --workspace 123e4567-e89b-12d3-a456-426614174000 --type windows
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			if name == "" {
				return fmt.Errorf("name is required")
			}
			if workspaceId == "" {
				return fmt.Errorf("workspace is required")
			}
			validTypes := map[string]bool{
				string(JobAgentTypeLinux):   true,
				string(JobAgentTypeWindows): true,
			}
			if !validTypes[jobAgentType] {
				return fmt.Errorf("invalid type: %s. Must be one of: linux, windows", jobAgentType)
			}

			ja, err := jobagent.NewJobAgent(
				client,
				api.UpsertJobAgentJSONRequestBody{
					Name:        name,
					Type:        jobAgentType,
					WorkspaceId: workspaceId,
				},
				&ExecRunner{},
			)
			if err != nil {
				return fmt.Errorf("failed to create job agent: %w", err)
			}
			if err := ja.RunQueuedJobs(); err != nil {
				log.Error("failed to run queued jobs", "error", err)
			}
			if err := ja.UpdateRunningJobs(); err != nil {
				log.Error("failed to check for jobs", "error", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the job agent")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&jobAgentType, "type", "exec-linux", "Type of the job agent, defaults to linux")
	return cmd
}
