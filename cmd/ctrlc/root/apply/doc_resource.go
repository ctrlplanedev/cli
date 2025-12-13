package apply

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const (
	TypeResource DocumentType = "Resource"
)

// ResourceDocument represents a Resource in YAML
type ResourceDocument struct {
	BaseDocument `yaml:",inline"`
	Identifier   string            `yaml:"identifier"`          // Unique identifier for the resource
	Kind         string            `yaml:"kind"`                // The kind of resource (e.g., "Cluster", "Namespace")
	Version      string            `yaml:"version"`             // Version string for the resource
	Config       map[string]any    `yaml:"config,omitempty"`    // Arbitrary configuration
	Metadata     map[string]string `yaml:"metadata,omitempty"`  // Key-value metadata
	Variables    map[string]any    `yaml:"variables,omitempty"` // Key-value variables
	Provider     string            `yaml:"provider,omitempty"`  // Optional: Name of the resource provider
}

func ParseResource(raw []byte) (*ResourceDocument, error) {
	var doc ResourceDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse resource document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("resource document missing required 'name' field")
	}
	if doc.Identifier == "" {
		return nil, fmt.Errorf("resource document missing required 'identifier' field")
	}
	if doc.Kind == "" {
		return nil, fmt.Errorf("resource document missing required 'kind' field")
	}
	if doc.Version == "" {
		return nil, fmt.Errorf("resource document missing required 'version' field")
	}
	return &doc, nil
}

var _ Document = &ResourceDocument{}

func (d *ResourceDocument) Order() int {
	return 300 // Lower priority - resources are processed later
}

func (d *ResourceDocument) Apply(ctx *DocContext) (ApplyResult, error) {
	result := ApplyResult{
		Type: TypeResource,
		Name: d.Name,
	}

	// Use default provider name if not specified
	providerName := d.Provider
	if providerName == "" {
		providerName = "ctrlc-apply"
	}

	// First, try to find the resource provider by name
	providerResp, err := ctx.Client.GetResourceProviderByNameWithResponse(ctx.Context, ctx.WorkspaceID, providerName)
	if err != nil {
		result.Error = fmt.Errorf("failed to get resource provider: %w", err)
		return result, result.Error
	}

	var providerID string
	if providerResp.JSON200 != nil {
		providerID = providerResp.JSON200.Id
	} else {
		// Provider doesn't exist, create it
		createResp, err := ctx.Client.UpsertResourceProviderWithResponse(ctx.Context, ctx.WorkspaceID, api.UpsertResourceProviderJSONRequestBody{
			Id:   providerName,
			Name: providerName,
		})
		if err != nil {
			result.Error = fmt.Errorf("failed to create resource provider: %w", err)
			return result, result.Error
		}
		if createResp.JSON200 == nil {
			result.Error = fmt.Errorf("failed to create resource provider: %s", string(createResp.Body))
			return result, result.Error
		}
		providerID = createResp.JSON200.Id
	}

	// Initialize metadata if nil
	metadata := d.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Initialize config if nil
	config := d.Config
	if config == nil {
		config = make(map[string]any)
	}

	// Create the resource via the provider using PATCH (upsert behavior)
	resources := []api.ResourceProviderResource{
		{
			Identifier: d.Identifier,
			Name:       d.Name,
			Kind:       d.Kind,
			Version:    d.Version,
			Config:     config,
			Metadata:   metadata,
		},
	}

	resp, err := ctx.Client.SetResourceProvidersResourcesPatchWithResponse(ctx.Context, ctx.WorkspaceID, providerID, api.SetResourceProvidersResourcesPatchJSONRequestBody{
		Resources: resources,
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert resource: %w", err)
		return result, result.Error
	}

	if resp.JSON202 == nil {
		result.Error = fmt.Errorf("failed to upsert resource: %s", string(resp.Body))
		return result, result.Error
	}

	result.ID = d.Identifier
	result.Action = "upserted"

	// Update variables if provided
	vars := d.Variables
	if vars == nil {
		vars = map[string]any{}
	}
	varsResp, err := ctx.Client.UpdateVariablesForResourceWithResponse(
		ctx.Context,
		ctx.WorkspaceID,
		d.Identifier,
		api.UpdateVariablesForResourceJSONRequestBody(vars),
	)
	if err != nil {
		result.Error = fmt.Errorf("failed to update resource variables: %w", err)
		return result, result.Error
	}
	if varsResp == nil || varsResp.StatusCode() != 204 {
		body := ""
		if varsResp != nil {
			body = string(varsResp.Body)
		}
		result.Error = fmt.Errorf("failed to update resource variables: %s", body)
		return result, result.Error
	}

	return result, nil
}

func (d *ResourceDocument) Delete(ctx *DocContext) (DeleteResult, error) {
	return DeleteResult{
		Type: TypeResource,
		Name: d.Name,
	}, fmt.Errorf("delete not implemented for resources")
}
