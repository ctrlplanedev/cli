package apply

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func processResourceProvider(ctx context.Context, client *api.ClientWithResponses, workspaceID string, provider ResourceProvider) {
	if provider.Name == "" {
		log.Info("Resource provider not provided, skipping")
		return
	}

	rp, err := api.NewResourceProvider(client, workspaceID, provider.Name)
	if err != nil {
		log.Error("Failed to create resource provider", "name", provider.Name, "error", err)
		return
	}

	resources := make([]api.AgentResource, 0)
	for _, resource := range provider.Resources {
		resources = append(resources, api.AgentResource{
			Identifier: resource.Identifier,
			Name:       resource.Name,
			Version:    resource.Version,
			Kind:       resource.Kind,
			Config:     resource.Config,
			Metadata:   resource.Metadata,
		})
	}

	upsertResp, err := rp.UpsertResource(ctx, resources)
	if err != nil {
		log.Error("Failed to upsert resources", "name", provider.Name, "error", err)
		return
	}

	log.Info("Response from upserting resources", "status", upsertResp.Status)
}
