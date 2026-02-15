package cloudrun

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	ctrlp "github.com/ctrlplanedev/cli/internal/common"
	"github.com/spf13/cobra"
	"google.golang.org/api/run/v1"
)

func validateFlags(project *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *project == "" {
			return fmt.Errorf("project is required")
		}
		return nil
	}
}

func initCloudRunClient(ctx context.Context) (*run.APIService, error) {
	cloudRunService, err := run.NewService(ctx)
	if err != nil {
		return nil, err
	}
	return cloudRunService, nil
}

func getLinks(service *run.Service) ([]byte, error) {
	links := map[string]string{}

	location := service.Metadata.Labels["cloud.googleapis.com/location"]
	if location != "" {
		links["google-cloudrun"] = fmt.Sprintf("https://console.cloud.google.com/run/detail/%s/%s", location, service.Metadata.Name)
	}

	linksJson, err := json.Marshal(links)
	if err != nil {
		return nil, err
	}

	return linksJson, nil
}

func processServiceMetadata(service *run.Service) map[string]string {
	metadata := map[string]string{}

	for key, value := range service.Metadata.Annotations {
		metadata[key] = value
	}

	for key, value := range service.Metadata.Labels {
		metadata[key] = value
	}

	for key, value := range service.Spec.Template.Metadata.Annotations {
		metadata[key] = value
	}

	for key, value := range service.Spec.Template.Metadata.Labels {
		metadata[key] = value
	}

	if service.Metadata.SelfLink != "" {
		metadata["selfLink"] = service.Metadata.SelfLink
	}

	if len(service.Spec.Template.Spec.Containers) > 0 {
		metadata["image"] = service.Spec.Template.Spec.Containers[0].Image
	}

	linksJson, err := getLinks(service)
	if err != nil {
		log.Error("Failed to get links", "error", err)
	}
	if err == nil {
		metadata["ctrlplane/links"] = string(linksJson)
	}

	return metadata
}

func processService(service *run.Service) api.ResourceProviderResource {
	resource := api.ResourceProviderResource{
		Name:       service.Metadata.Name,
		Identifier: service.Metadata.SelfLink,
		Version:    "ctrlplane.dev/container/service/v1",
		Kind:       "GoogleCloudRunService",
		Metadata:   processServiceMetadata(service),
		Config: map[string]interface{}{
			"image": service.Spec.Template.Spec.Containers[0].Image,
		},
	}

	return resource
}

func runSync(project, providerName *string, regions *[]string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Info("Syncing Cloud Run services into Ctrlplane", "project", *project)

		ctx := context.Background()

		cloudRunService, err := initCloudRunClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize Cloud Run client: %w", err)
		}

		services, err := cloudRunService.Projects.Locations.Services.List(fmt.Sprintf("projects/%s/locations/-", *project)).Do()
		if err != nil {
			return fmt.Errorf("failed to list Cloud Run services: %w", err)
		}

		allResources := make([]api.ResourceProviderResource, 0)
		for _, service := range services.Items {
			resource := processService(service)
			allResources = append(allResources, resource)
		}

		// Set default provider name if not provided
		if *providerName == "" {
			*providerName = fmt.Sprintf("google-cloudrun-%s", *project)
		}

		// Upsert resources to Ctrlplane
		return ctrlp.UpsertResources(ctx, allResources, providerName)
	}
}

func NewSyncCloudRunCmd() *cobra.Command {
	var project string
	var providerName string
	var regions []string

	cmd := &cobra.Command{
		Use:   "cloudrun",
		Short: "Sync Google Cloud Run services into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure Google Cloud credentials are configured via environment variables or application default credentials
			
			# Sync all Cloud Run services from a project
			$ ctrlc sync google-cloud cloudrun --project my-project
		`),
		PreRunE: validateFlags(&project),
		RunE:    runSync(&project, &providerName, &regions),
	}

	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&project, "project", "c", "", "Google Cloud Project ID")
	cmd.MarkFlagRequired("project")

	return cmd
}
