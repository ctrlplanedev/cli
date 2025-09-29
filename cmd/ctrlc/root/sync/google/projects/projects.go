package projects

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/cloudresourcemanager/v1"
)

// NewSyncProjectsCmd creates a new cobra command for syncing Google Cloud projects
func NewSyncProjectsCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Sync Google Cloud projects into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure Google Cloud credentials are configured via environment variables or application default credentials
			
			# Sync all projects
			$ ctrlc sync google projects
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Syncing Google Cloud projects into Ctrlplane")

			ctx := context.Background()

			// Create Cloud Resource Manager client
			crm, err := cloudresourcemanager.NewService(ctx)
			if err != nil {
				return fmt.Errorf("failed to create Cloud Resource Manager client: %w", err)
			}

			// Create span for listing projects
			_, listSpan := telemetry.StartSpan(ctx, "google.cloudresourcemanager.list_projects",
				trace.WithSpanKind(trace.SpanKindClient),
			)

			// List all projects
			resp, err := crm.Projects.List().Do()

			if err != nil {
				telemetry.SetSpanError(listSpan, err)
				listSpan.End()
				return fmt.Errorf("failed to list projects: %w", err)
			}

			telemetry.AddSpanAttribute(listSpan, "google.projects.projects_found", len(resp.Projects))
			telemetry.SetSpanSuccess(listSpan)
			listSpan.End()

			log.Info("Found projects", "count", len(resp.Projects))

			resources := []api.CreateResource{}

			// Process each project
			for _, project := range resp.Projects {
				metadata := map[string]string{
					"account/id":          project.ProjectId,
					"account/name":        project.Name,
					"account/number":      fmt.Sprintf("%d", project.ProjectNumber),
					"account/state":       project.LifecycleState,
					"account/parent-id":   project.Parent.Id,
					"account/parent-type": project.Parent.Type,

					"google/project":     project.ProjectId,
					"google/number":      fmt.Sprintf("%d", project.ProjectNumber),
					"google/state":       project.LifecycleState,
					"google/parent-id":   project.Parent.Id,
					"google/parent-type": project.Parent.Type,
				}

				// Add labels as metadata
				for key, value := range project.Labels {
					metadata[fmt.Sprintf("labels/%s", key)] = value
				}

				resources = append(resources, api.CreateResource{
					Version:    "ctrlplane.dev/cloud/account/v1",
					Kind:       "GoogleProject",
					Name:       project.Name,
					Identifier: project.ProjectId,
					Config: map[string]any{
						"id":            project.ProjectId,
						"name":          project.Name,
						"projectNumber": project.ProjectNumber,
						"state":         project.LifecycleState,
						"parent": map[string]any{
							"id":   project.Parent.Id,
							"type": project.Parent.Type,
						},
						"labels": project.Labels,
					},
					Metadata: metadata,
				})
			}

			// Upsert resources to Ctrlplane
			if name == "" {
				name = "google-projects"
			}

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")

			ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			log.Info("Upserting resource provider", "name", name)
			rp, err := api.NewResourceProvider(ctrlplaneClient, workspaceId, name)
			if err != nil {
				return fmt.Errorf("failed to create resource provider: %w", err)
			}

			upsertResp, err := rp.UpsertResource(ctx, resources)
			if err != nil {
				return fmt.Errorf("failed to upsert resources: %w", err)
			}

			log.Info("Response from upserting resources", "status", upsertResp.Status)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")

	return cmd
}
