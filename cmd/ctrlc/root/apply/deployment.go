package apply

import (
	"context"
	"encoding/json"
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

	if deployment.ExitHooks != nil {
		exitHooks := []api.ExitHook{}
		for _, exitHook := range *deployment.ExitHooks {
			jobAgentUUID, err := uuid.Parse(exitHook.JobAgent.Id)
			if err != nil {
				log.Error("Failed to parse job agent ID as UUID", "id", exitHook.JobAgent.Id, "error", err)
				return
			}
			exitHooks = append(exitHooks, api.ExitHook{
				Name:           exitHook.Name,
				JobAgentId:     jobAgentUUID,
				JobAgentConfig: exitHook.JobAgent.Config,
			})
		}
		body.ExitHooks = &exitHooks
	}

	id, err := upsertDeployment(ctx, client, body)
	if err != nil {
		log.Error("Failed to create deployment", "name", deployment.Name, "error", err)
		return
	}

	if deployment.Variables != nil {
		deploymentID, _ := uuid.Parse(id)
		var variableWg sync.WaitGroup
		for _, variable := range *deployment.Variables {
			variableWg.Add(1)
			go func(v DeploymentVariable) {
				defer variableWg.Done()
				upsertDeploymentVariable(ctx, client, deploymentID, v)
			}(variable)
		}
		variableWg.Wait()
	}
}

func upsertDeploymentVariable(
	ctx context.Context,
	client *api.ClientWithResponses,
	deploymentID uuid.UUID,
	variable DeploymentVariable,
) {
	vars := []api.VariableValue{}
	for _, value := range variable.Values {
		if value.Value != nil {
			directValue := api.DeploymentVariableDirectValue{}
			directValue.Default = value.Default
			directValue.Sensitive = value.Sensitive
			directValue.ValueType = "direct"
			directValue.ResourceSelector = value.ResourceSelector

			directValue.Value = api.DeploymentVariableDirectValue_Value{}
			valueData, err := json.Marshal(*value.Value)
			if err != nil {
				log.Error("Failed to marshal direct value", "error", err)
				continue
			}
			directValue.Value.UnmarshalJSON(valueData)

			var varDirect api.VariableValue
			varDirect.FromDeploymentVariableDirectValue(directValue)
			vars = append(vars, varDirect)
			continue
		}

		if value.Reference != nil && value.Path != nil {
			referenceValue := api.DeploymentVariableReferenceValue{}
			referenceValue.Default = value.Default
			referenceValue.Reference = *value.Reference
			referenceValue.Path = *value.Path
			referenceValue.ResourceSelector = value.ResourceSelector
			referenceValue.ValueType = "reference"

			if value.DefaultValue != nil {
				referenceValue.DefaultValue = &api.DeploymentVariableReferenceValue_DefaultValue{}
				valueData, err := json.Marshal(*value.DefaultValue)
				if err != nil {
					log.Error("Failed to marshal default value", "error", err)
					continue
				}
				referenceValue.DefaultValue.UnmarshalJSON(valueData)
			}

			varReference := api.VariableValue{}
			varReference.FromDeploymentVariableReferenceValue(referenceValue)
			vars = append(vars, varReference)
			continue
		}

		log.Error("Unsupported variable value type", "type", variable.Values)
	}

	log.Info("Creating deployment variable", "key", variable.Key, "values", len(vars))

	body := api.CreateDeploymentVariableJSONRequestBody{
		Key:         variable.Key,
		Description: variable.Description,
		Values:      &vars,
		Config:      variable.Config,
	}

	_, err := client.CreateDeploymentVariableWithResponse(ctx, deploymentID, body)
	if err != nil {
		log.Error("Failed to create deployment variable", "name", variable.Key, "error", err)
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
