package exec

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

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

			// Get interval from the flag set by AddIntervalSupport
			intervalStr, _ := cmd.Flags().GetString("interval")

			interval := 10 * time.Second
			if intervalStr != "" {
				duration, err := time.ParseDuration(intervalStr)
				if err != nil {
					return fmt.Errorf("invalid interval format: %w", err)
				}
				interval = duration
			}

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

			// Polling loop
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := ja.RunQueuedJobs(); err != nil {
						log.Error("Failed to run queued jobs", "error", err)
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the job agent")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("workspace")
	return cmd
}
