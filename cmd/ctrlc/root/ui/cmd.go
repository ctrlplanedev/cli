package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewUICmd creates the `ctrlc ui` command
func NewUICmd() *cobra.Command {
	var refreshInterval int

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Interactive terminal UI for browsing ctrlplane resources",
		Long:  `Launch a k9s-style terminal UI to browse deployments, resources, jobs, environments, and deployment versions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			if apiKey == "" {
				return fmt.Errorf("api-key is required: set via --api-key, CTRLPLANE_API_KEY, or config file")
			}
			if workspace == "" {
				return fmt.Errorf("workspace is required: set via --workspace, CTRLPLANE_WORKSPACE, or config file")
			}

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Resolve workspace slug to UUID
			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)
			if workspaceID.String() == "00000000-0000-0000-0000-000000000000" {
				return fmt.Errorf("failed to resolve workspace: %s", workspace)
			}

			// Load last-viewed resource type (default: resources)
			startView := loadLastView()

			interval := time.Duration(refreshInterval) * time.Second
			model := NewModel(client, workspaceID.String(), interval, startView, workspace, apiURL)

			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&refreshInterval, "refresh", 10, "Auto-refresh interval in seconds (0 to disable)")

	return cmd
}
