package apply

import (
	"encoding/json"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"gopkg.in/yaml.v3"
)

const (
	TypeDeployment DocumentType = "Deployment"
)

// DeploymentDocument represents a Deployment in YAML
type DeploymentDocument struct {
	BaseDocument     `yaml:",inline"`
	System           string         `yaml:"system"`                     // System name or ID
	Slug             *string        `yaml:"slug,omitempty"`             // Optional slug, auto-generated from name if not provided
	Description      *string        `yaml:"description,omitempty"`      // Optional description
	ResourceSelector *string        `yaml:"resourceSelector,omitempty"` // Optional resource selector (CEL or JSON)
	JobAgent         *string        `yaml:"jobAgent,omitempty"`         // Optional job agent name
	JobAgentConfig   map[string]any `yaml:"jobAgentConfig,omitempty"`   // Optional job agent configuration
}

func ParseDeployment(raw []byte) (*DeploymentDocument, error) {
	var doc DeploymentDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse deployment document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("deployment document missing required 'name' field")
	}
	if doc.System == "" {
		return nil, fmt.Errorf("deployment document missing required 'system' field")
	}
	return &doc, nil
}

var _ Document = &DeploymentDocument{}

func (d *DeploymentDocument) Order() int {
	return 700 // Deployments depend on JobAgents (900)
}

func (d *DeploymentDocument) Apply(ctx *DocContext) (ApplyResult, error) {
	result := ApplyResult{
		Type: TypeDeployment,
		Name: d.Name,
	}

	// Resolve system ID
	systemID, err := ctx.Resolver.ResolveSystemID(ctx.Context, d.System)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve system '%s': %w", d.System, err)
		return result, result.Error
	}

	// Generate slug from name if not provided
	deploymentSlug := slug.Make(d.Name)
	if d.Slug != nil && *d.Slug != "" {
		deploymentSlug = *d.Slug
	}

	// Build resource selector
	var resourceSelector *api.Selector
	if d.ResourceSelector != nil {
		resourceSelector, err = buildSelector(&SelectorConfig{
			Cel: d.ResourceSelector,
		})
		if err != nil {
			result.Error = fmt.Errorf("failed to build resource selector: %w", err)
			return result, result.Error
		}
	}

	// Resolve job agent ID if specified
	var jobAgentId *string
	if d.JobAgent != nil && *d.JobAgent != "" {
		agentID, err := resolveJobAgentID(ctx, *d.JobAgent)
		if err != nil {
			result.Error = fmt.Errorf("failed to resolve job agent '%s': %w", *d.JobAgent, err)
			return result, result.Error
		}
		jobAgentId = &agentID
	}

	// List existing deployments to check if one with the same slug exists
	deploymentsResp, err := ctx.Client.ListDeploymentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListDeploymentsParams{})
	if err != nil {
		result.Error = fmt.Errorf("failed to list deployments: %w", err)
		return result, result.Error
	}
	if deploymentsResp.JSON200 == nil {
		result.Error = fmt.Errorf("failed to list deployments: %s", string(deploymentsResp.Body))
		return result, result.Error
	}

	// Find existing deployment by slug within the same system
	var existingDeployment *api.Deployment
	for _, dep := range deploymentsResp.JSON200.Items {
		if dep.Deployment.Slug == deploymentSlug && dep.Deployment.SystemId == systemID {
			existingDeployment = &dep.Deployment
			break
		}
	}

	// Generate new ID or use existing
	deploymentID := uuid.New().String()
	if existingDeployment != nil {
		deploymentID = existingDeployment.Id
	}

	var jobAgentConfig api.DeploymentJobAgentConfig
	if d.JobAgentConfig != nil {
		jobAgentConfigBytes, err := json.Marshal(d.JobAgentConfig)
		if err != nil {
			result.Error = fmt.Errorf("failed to marshal job agent config: %w", err)
			return result, result.Error
		}
		if err := json.Unmarshal(jobAgentConfigBytes, &jobAgentConfig); err != nil {
			result.Error = fmt.Errorf("failed to parse job agent config: %w", err)
			return result, result.Error
		}
	}

	upsertReq := api.UpsertDeploymentJSONRequestBody{
		Name:             d.Name,
		Slug:             deploymentSlug,
		SystemId:         systemID,
		Description:      d.Description,
		ResourceSelector: resourceSelector,
		JobAgentId:       jobAgentId,
		JobAgentConfig:   &jobAgentConfig,
	}

	// Upsert the deployment
	upsertResp, err := ctx.Client.UpsertDeploymentWithResponse(ctx.Context, ctx.WorkspaceID, deploymentID, upsertReq)
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert deployment: %w", err)
		return result, result.Error
	}
	if upsertResp.JSON202 == nil {
		result.Error = fmt.Errorf("failed to upsert deployment: %s", string(upsertResp.Body))
		return result, result.Error
	}

	result.ID = upsertResp.JSON202.Id
	if existingDeployment != nil {
		result.Action = "updated"
	} else {
		result.Action = "created"
	}

	return result, nil
}

func (d *DeploymentDocument) Delete(ctx *DocContext) (DeleteResult, error) {
	result := DeleteResult{
		Type: TypeDeployment,
		Name: d.Name,
	}

	// Resolve system ID
	systemID, err := ctx.Resolver.ResolveSystemID(ctx.Context, d.System)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve system '%s': %w", d.System, err)
		return result, result.Error
	}

	// Generate slug from name if not provided
	deploymentSlug := slug.Make(d.Name)
	if d.Slug != nil && *d.Slug != "" {
		deploymentSlug = *d.Slug
	}

	// List deployments to find the one to delete
	deploymentsResp, err := ctx.Client.ListDeploymentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListDeploymentsParams{})
	if err != nil {
		result.Error = fmt.Errorf("failed to list deployments: %w", err)
		return result, result.Error
	}
	if deploymentsResp.JSON200 == nil {
		result.Error = fmt.Errorf("failed to list deployments: %s", string(deploymentsResp.Body))
		return result, result.Error
	}

	// Find existing deployment by slug within the same system
	var deploymentID string
	for _, dep := range deploymentsResp.JSON200.Items {
		if dep.Deployment.Slug == deploymentSlug && dep.Deployment.SystemId == systemID {
			deploymentID = dep.Deployment.Id
			break
		}
	}

	if deploymentID == "" {
		result.Action = "not_found"
		return result, nil
	}

	// Delete the deployment
	deleteResp, err := ctx.Client.DeleteDeploymentWithResponse(ctx.Context, ctx.WorkspaceID, deploymentID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete deployment: %w", err)
		return result, result.Error
	}

	if deleteResp.StatusCode() == 404 {
		result.Action = "not_found"
		return result, nil
	}

	if deleteResp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete deployment: %s", string(deleteResp.Body))
		return result, result.Error
	}

	result.ID = deploymentID
	result.Action = "deleted"
	return result, nil
}

// resolveJobAgentID resolves a job agent name to its ID
func resolveJobAgentID(ctx *DocContext, nameOrID string) (string, error) {
	// Check if it's already a valid UUID
	if _, err := uuid.Parse(nameOrID); err == nil {
		return nameOrID, nil
	}

	// List job agents to find by name
	agentsResp, err := ctx.Client.ListJobAgentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListJobAgentsParams{})
	if err != nil {
		return "", fmt.Errorf("failed to list job agents: %w", err)
	}
	if agentsResp.JSON200 == nil {
		return "", fmt.Errorf("failed to list job agents: %s", string(agentsResp.Body))
	}

	for _, agent := range agentsResp.JSON200.Items {
		if agent.Name == nameOrID {
			return agent.Id, nil
		}
	}

	return "", fmt.Errorf("job agent not found: %s", nameOrID)
}
