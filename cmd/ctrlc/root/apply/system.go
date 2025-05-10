package apply

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
)

func processAllSystems(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID uuid.UUID,
	systems []System,
) {
	var systemWg sync.WaitGroup

	for _, system := range systems {
		systemWg.Add(1)
		go processSystem(
			ctx,
			client,
			workspaceID,
			system,
			&systemWg,
		)
	}

	systemWg.Wait()
}

func processSystem(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID uuid.UUID,
	system System,
	systemWg *sync.WaitGroup,
) {
	defer systemWg.Done()

	log.Info("Upserting system", "name", system.Name)
	systemID, err := upsertSystem(ctx, client, workspaceID, system)
	if err != nil {
		log.Error("Failed to upsert system", "name", system.Name, "error", err)
		return
	}
	log.Info("System created successfully", "name", system.Name, "id", systemID)

	systemIDUUID, err := uuid.Parse(systemID)
	if err != nil {
		log.Error("Failed to parse system ID as UUID", "id", systemID, "error", err)
		return
	}

	processSystemDeployments(ctx, client, systemIDUUID, system)
}

func processSystemDeployments(
	ctx context.Context,
	client *api.ClientWithResponses,
	systemID uuid.UUID,
	system System,
) {
	var deploymentWg sync.WaitGroup
	var environmentWg sync.WaitGroup
	for _, deployment := range system.Deployments {
		deploymentWg.Add(1)
		log.Info("Creating deployment", "system", system.Name, "name", deployment.Name)
		go processDeployment(
			ctx,
			client,
			systemID,
			deployment,
			&deploymentWg,
		)
	}

	for _, environment := range system.Environments {
		environmentWg.Add(1)
		log.Info("Creating environment", "system", system.Name, "name", environment.Name)
		go processEnvironment(ctx, client, systemID, environment, &environmentWg)
	}

	environmentWg.Wait()
	deploymentWg.Wait()
}

func upsertSystem(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID uuid.UUID,
	system System,
) (string, error) {
	resp, err := client.CreateSystemWithResponse(ctx, api.CreateSystemJSONRequestBody{
		WorkspaceId: workspaceID,
		Name:        system.Name,
		Slug:        system.Slug,
		Description: &system.Description,
	})

	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("API returned error status: %d", resp.StatusCode())
	}

	if resp.JSON200 != nil {
		return resp.JSON200.Id.String(), nil
	}

	if resp.JSON201 != nil {
		return resp.JSON201.Id.String(), nil
	}

	return "", fmt.Errorf("unexpected response format")
}
