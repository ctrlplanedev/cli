package providers

import (
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/ctrlplanedev/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const resourceTypeName = "Resource"

type ResourceProvider struct{}

func init() {
	RegisterProvider(&ResourceProvider{})
}

func (p *ResourceProvider) TypeName() string {
	return resourceTypeName
}

func (p *ResourceProvider) Order() int {
	return 300
}

func (p *ResourceProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec ResourceItemSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse resource document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("resource document missing required 'name' field")
	}
	if spec.Identifier == "" {
		return nil, fmt.Errorf("resource document missing required 'identifier' field")
	}
	if spec.Kind == "" {
		return nil, fmt.Errorf("resource document missing required 'kind' field")
	}
	if spec.Version == "" {
		return nil, fmt.Errorf("resource document missing required 'version' field")
	}
	return &spec, nil
}

type ResourceItemSpec struct {
	Type        string            `yaml:"type,omitempty"`
	DisplayName string            `yaml:"name"`
	Identifier  string            `yaml:"identifier"`
	Kind        string            `yaml:"kind"`
	Version     string            `yaml:"version"`
	Config      map[string]any    `yaml:"config,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	Variables   map[string]any    `yaml:"variables,omitempty"`
	Provider    string            `yaml:"provider,omitempty"`
}

func (r *ResourceItemSpec) Name() string {
	return r.DisplayName
}

func (r *ResourceItemSpec) Identity() string {
	return r.Identifier
}

func (r *ResourceItemSpec) Lookup(ctx Context) (string, error) {
	if r.Identifier == "" {
		return "", nil
	}
	return r.Identifier, nil
}

func (r *ResourceItemSpec) Create(ctx Context, id string) error {
	return r.upsert(ctx)
}

func (r *ResourceItemSpec) Update(ctx Context, existingID string) error {
	return r.upsert(ctx)
}

func (r *ResourceItemSpec) Delete(ctx Context, existingID string) error {
	return fmt.Errorf("delete not implemented for resources")
}

func (r *ResourceItemSpec) upsert(ctx Context) error {
	providerID, err := r.getProviderID(ctx)
	if err != nil {
		return err
	}

	metadata := r.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	config := r.Config
	if config == nil {
		config = make(map[string]any)
	}

	resources := []api.ResourceProviderResource{
		{
			Identifier: r.Identifier,
			Name:       r.DisplayName,
			Kind:       r.Kind,
			Version:    r.Version,
			Config:     config,
			Metadata:   metadata,
		},
	}

	resp, err := ctx.APIClient().SetResourceProvidersResourcesPatchWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), providerID, api.SetResourceProvidersResourcesPatchJSONRequestBody{
		Resources: resources,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert resource: %w", err)
	}
	if resp.JSON202 == nil {
		return fmt.Errorf("failed to upsert resource: %s", string(resp.Body))
	}

	return r.syncVariables(ctx)
}

func (r *ResourceItemSpec) getProviderID(ctx Context) (string, error) {
	providerName := r.Provider
	if providerName == "" {
		providerName = "ctrlc-apply"
	}

	providerResp, err := ctx.APIClient().GetResourceProviderByNameWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), providerName)
	if err != nil {
		return "", fmt.Errorf("failed to get resource provider: %w", err)
	}

	if providerResp.JSON200 != nil {
		return providerResp.JSON200.Id, nil
	}

	createResp, err := ctx.APIClient().UpsertResourceProviderWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), api.UpsertResourceProviderJSONRequestBody{
		Name: providerName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create resource provider: %w", err)
	}
	if createResp.JSON200 == nil {
		return "", fmt.Errorf("failed to create resource provider: %s", string(createResp.Body))
	}
	return createResp.JSON200.Id, nil
}

func (r *ResourceItemSpec) syncVariables(ctx Context) error {
	vars := r.Variables
	if vars == nil {
		vars = make(map[string]any)
	}

	err := retry.Do(
		func() error {
			varsResp, err := ctx.APIClient().UpdateVariablesForResourceWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), r.Identifier, api.UpdateVariablesForResourceJSONRequestBody(vars))
			if err != nil {
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: %w", err))
			}
			if varsResp == nil {
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: empty response"))
			}
			if varsResp.StatusCode() == 404 {
				return fmt.Errorf("resource not found yet, retrying")
			}
			if varsResp.StatusCode() != 204 {
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: %s", string(varsResp.Body)))
			}
			return nil
		},
		retry.Attempts(10),
		retry.Delay(100*time.Millisecond),
		retry.MaxDelay(15*time.Second),
		retry.DelayType(retry.BackOffDelay),
	)
	return err
}
