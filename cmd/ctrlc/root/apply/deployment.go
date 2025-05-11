package apply

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
)

func processDeployment(
	ctx context.Context,
	client *api.ClientWithResponses,
	systemID uuid.UUID,
	deployment Deployment,
	deploymentWg *sync.WaitGroup,
) {
	defer deploymentWg.Done()

	body := createDeploymentRequestBody(systemID, deployment)

	if deployment.JobAgent != nil {
		jobAgentUUID, err := uuid.Parse(deployment.JobAgent.Id)
		if err != nil {
			log.Error("Failed to parse job agent ID as UUID", "id", deployment.JobAgent.Id, "error", err)
			return
		}
		body.JobAgentId = &jobAgentUUID
		body.JobAgentConfig = &deployment.JobAgent.Config
	}

	_, err := upsertDeployment(ctx, client, body)
	if err != nil {
		log.Error("Failed to create deployment", "name", deployment.Name, "error", err)
	}
}

func createDeploymentRequestBody(systemID uuid.UUID, deployment Deployment) api.CreateDeploymentJSONBody {
	return api.CreateDeploymentJSONBody{
		SystemId:         systemID,
		Slug:             deployment.Slug,
		Name:             deployment.Name,
		Description:      deployment.Description,
		ResourceSelector: deployment.ResourceSelector,
	}
}

func upsertDeployment(
	ctx context.Context,
	client *api.ClientWithResponses,
	deployment api.CreateDeploymentJSONBody,
) (string, error) {
	resp, err := client.CreateDeploymentWithResponse(ctx, api.CreateDeploymentJSONRequestBody(deployment))

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

func processEnvironment(
	ctx context.Context,
	client *api.ClientWithResponses,
	systemID uuid.UUID,
	environment Environment,
	environmentWg *sync.WaitGroup,
) {
	defer environmentWg.Done()

	body := api.CreateEnvironmentJSONRequestBody{
		SystemId:         systemID.String(),
		Name:             environment.Name,
		Description:      environment.Description,
		ResourceSelector: environment.ResourceSelector,
		Metadata:         environment.Metadata,
	}

	_, err := client.CreateEnvironmentWithResponse(ctx, body)
	if err != nil {
		log.Error("Failed to create environment", "name", environment.Name, "error", err)
	}
}
