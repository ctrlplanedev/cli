package exec

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRunExecCmd() *cobra.Command {
	var (
		name         string
		jobAgentType = "exec-bash"
	)

	if runtime.GOOS == "windows" {
		jobAgentType = "exec-powershell"
	}

	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute commands directly when a job is received",
		Example: heredoc.Doc(`
			$ ctrlc run exec --name "my-script-agent" --workspace 123e4567-e89b-12d3-a456-426614174000
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")
			if name == "" {
				return fmt.Errorf("name is required")
			}
			if workspaceId == "" {
				return fmt.Errorf("workspace is required")
			}
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			runner := NewExecRunner(client)
			jobAgentConfig := api.UpsertJobAgentJSONRequestBody{
				Name:        name,
				Type:        jobAgentType,
				WorkspaceId: workspaceId,
			}
			ja, err := jobagent.NewJobAgent(
				client,
				jobAgentConfig,
				runner,
			)
			if err != nil {
				return fmt.Errorf("failed to create job agent: %w", err)
			}

			// Set up a simple shutdown handler for non-interval mode
			// When used with AddIntervalSupport, this would only affect a single iteration
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-c
				log.Info("Shutting down gracefully...")
				runner.ExitAll(true)
			}()

			// Run job check - AddIntervalSupport will handle repeated execution
			if err := ja.RunQueuedJobs(); err != nil {
				return fmt.Errorf("failed to run queued jobs: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the job agent")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("workspace")
	return cmd
}
