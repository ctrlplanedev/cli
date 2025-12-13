package apply

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	TypeEnvironment DocumentType = "Environment"
)

// EnvironmentDocument represents an Environment in YAML
type EnvironmentDocument struct {
	BaseDocument     `yaml:",inline"`
	System           string  `yaml:"system"`                     // System name, slug, or ID
	Description      *string `yaml:"description,omitempty"`      // Optional description
	ResourceSelector *string `yaml:"resourceSelector,omitempty"` // Optional resource selector (CEL or JSON)
}

func ParseEnvironment(raw []byte) (*EnvironmentDocument, error) {
	var doc EnvironmentDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse environment document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("environment document missing required 'name' field")
	}
	if doc.System == "" {
		return nil, fmt.Errorf("environment document missing required 'system' field")
	}
	return &doc, nil
}

var _ Document = &EnvironmentDocument{}

func (d *EnvironmentDocument) Order() int {
	return 600 // Environments depend on Systems, processed after Deployments (700)
}

func (d *EnvironmentDocument) Apply(ctx *DocContext) (ApplyResult, error) {
	result := ApplyResult{
		Type: TypeEnvironment,
		Name: d.Name,
	}

	// Resolve system ID
	systemID, err := ctx.Resolver.ResolveSystemID(ctx.Context, d.System)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve system '%s': %w", d.System, err)
		return result, result.Error
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

	// List existing environments to check if one with the same name exists in the system
	environmentsResp, err := ctx.Client.ListEnvironmentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListEnvironmentsParams{})
	if err != nil {
		result.Error = fmt.Errorf("failed to list environments: %w", err)
		return result, result.Error
	}
	if environmentsResp.JSON200 == nil {
		result.Error = fmt.Errorf("failed to list environments: %s", string(environmentsResp.Body))
		return result, result.Error
	}

	// Find existing environment by name within the same system, error if multiple found
	var existingEnvironment *api.Environment
	matchingCount := 0
	for _, env := range environmentsResp.JSON200.Items {
		if env.Name == d.Name && env.SystemId == systemID {
			matchingCount++
			if matchingCount == 1 {
				existingEnvironment = &env
			}
		}
	}
	if matchingCount > 1 {
		result.Error = fmt.Errorf("multiple environments found with name '%s' in system '%s', unable to determine which to update", d.Name, d.System)
		return result, result.Error
	}

	// Generate new ID or use existing
	environmentID := uuid.New().String()
	if existingEnvironment != nil {
		environmentID = existingEnvironment.Id
	}

	// Build request body
	upsertReq := api.UpsertEnvironmentByIdJSONRequestBody{
		Name:             d.Name,
		SystemId:         systemID,
		Description:      d.Description,
		ResourceSelector: resourceSelector,
	}

	// Upsert the environment
	upsertResp, err := ctx.Client.UpsertEnvironmentByIdWithResponse(ctx.Context, ctx.WorkspaceID, environmentID, upsertReq)
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert environment: %w", err)
		return result, result.Error
	}
	if upsertResp.JSON200 == nil {
		result.Error = fmt.Errorf("failed to upsert environment: %s", string(upsertResp.Body))
		return result, result.Error
	}

	result.ID = upsertResp.JSON200.Id
	if existingEnvironment != nil {
		result.Action = "updated"
	} else {
		result.Action = "created"
	}

	return result, nil
}

func (d *EnvironmentDocument) Delete(ctx *DocContext) (DeleteResult, error) {
	result := DeleteResult{
		Type: TypeEnvironment,
		Name: d.Name,
	}

	// Resolve system ID
	systemID, err := ctx.Resolver.ResolveSystemID(ctx.Context, d.System)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve system '%s': %w", d.System, err)
		return result, result.Error
	}

	// List environments to find the one to delete
	environmentsResp, err := ctx.Client.ListEnvironmentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListEnvironmentsParams{})
	if err != nil {
		result.Error = fmt.Errorf("failed to list environments: %w", err)
		return result, result.Error
	}
	if environmentsResp.JSON200 == nil {
		result.Error = fmt.Errorf("failed to list environments: %s", string(environmentsResp.Body))
		return result, result.Error
	}

	// Find existing environment by name within the same system
	var environmentID string
	matchingCount := 0
	for _, env := range environmentsResp.JSON200.Items {
		if env.Name == d.Name && env.SystemId == systemID {
			matchingCount++
			if matchingCount == 1 {
				environmentID = env.Id
			}
		}
	}
	if matchingCount > 1 {
		result.Error = fmt.Errorf("multiple environments found with name '%s' in system '%s', unable to determine which to delete", d.Name, d.System)
		return result, result.Error
	}
	if environmentID == "" {
		result.Action = "not_found"
		return result, nil
	}

	// Delete the environment
	deleteResp, err := ctx.Client.DeleteEnvironmentWithResponse(ctx.Context, ctx.WorkspaceID, environmentID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete environment: %w", err)
		return result, result.Error
	}

	if deleteResp.StatusCode() == 404 {
		result.Action = "not_found"
		return result, nil
	}

	if deleteResp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete environment: %s", string(deleteResp.Body))
		return result, result.Error
	}

	result.ID = environmentID
	result.Action = "deleted"
	return result, nil
}
