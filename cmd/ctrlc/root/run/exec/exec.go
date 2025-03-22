package exec

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRunExecCmd() *cobra.Command {
	var (
		name string
		workspaceID string
		interval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute commands directly when a job is received",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Create the ExecRunner with API client
			runner := NewExecRunner(client)

			// Create job agent config
			agentConfig := api.UpsertJobAgentJSONRequestBody{
				Name: name,
				Type: "exec",
			}
			
			// Set workspace ID if provided
			if workspaceID != "" {
				agentConfig.WorkspaceId = workspaceID
			}

			// Create job agent
			ja, err := jobagent.NewJobAgent(
				client,
				agentConfig,
				runner,
			)
			if err != nil {
				return fmt.Errorf("failed to create job agent: %w", err)
			}

			log.Info("Exec runner started", "name", name, "workspaceID", workspaceID)

			// Setup signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			go func() {
				<-sigCh
				log.Info("Shutting down gracefully...")
				runner.ExitAll(true)
				cancel()
			}()

			// Main polling loop
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Run initial job check
			if err := ja.RunQueuedJobs(); err != nil {
				log.Error("Failed to run queued jobs", "error", err)
			}
			if err := ja.UpdateRunningJobs(); err != nil {
				log.Error("Failed to check for running jobs", "error", err)
			}

			// Polling loop
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := ja.RunQueuedJobs(); err != nil {
						log.Error("Failed to run queued jobs", "error", err)
					}
					if err := ja.UpdateRunningJobs(); err != nil {
						log.Error("Failed to check for running jobs", "error", err)
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "exec-runner", "Name of the job agent")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Workspace ID to pull jobs from")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Polling interval for checking jobs")

	return cmd
}
