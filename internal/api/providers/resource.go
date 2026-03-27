package providers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go"
	"github.com/charmbracelet/log"
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

	patchReq := api.SetResourceProviderResourcesJSONRequestBody{
		Resources: resources,
	}
	resp, err := ctx.APIClient().SetResourceProviderResourcesWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), providerID, patchReq)
	if err != nil {
		return fmt.Errorf("failed to upsert resource: %w", err)
	}
	if resp.StatusCode() != http.StatusAccepted {
		return fmt.Errorf("failed to upsert resource: %s", resp.Status())
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

	createResp, err := ctx.APIClient().RequestResourceProviderUpsertWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), api.RequestResourceProviderUpsertJSONRequestBody{
		Name: providerName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create resource provider: %w", err)
	}
	if createResp.StatusCode() != http.StatusAccepted {
		return "", fmt.Errorf("failed to create resource provider: %s", createResp.Status())
	}
	return createResp.JSON202.Id, nil
}

// BatchUpsertResources groups resources by provider and makes one
// SetResourceProviderResources call per provider with all resources in that
// group. This avoids the overwrite problem where sequential single-resource
// calls replace the entire provider's resource set.
// Resources with no provider are upserted individually via the regular
// resource upsert endpoint (PATCH /resources/identifier/{identifier}).
func BatchUpsertResources(ctx Context, specs []*ResourceItemSpec) []Result {
	var noProviderSpecs []*ResourceItemSpec
	byProvider := make(map[string][]*ResourceItemSpec)
	for _, spec := range specs {
		providerName := spec.Provider
		if providerName == "" {
			noProviderSpecs = append(noProviderSpecs, spec)
			continue
		}
		byProvider[providerName] = append(byProvider[providerName], spec)
	}

	var results []Result

	for _, spec := range noProviderSpecs {
		results = append(results, spec.upsertWithoutProvider(ctx))
	}

	for providerName, group := range byProvider {
		providerID, err := group[0].getProviderID(ctx)
		if err != nil {
			for _, spec := range group {
				results = append(results, Result{
					Type:  resourceTypeName,
					Name:  spec.DisplayName,
					Error: fmt.Errorf("failed to get provider %q: %w", providerName, err),
				})
			}
			continue
		}

		apiResources := make([]api.ResourceProviderResource, 0, len(group))
		for _, spec := range group {
			metadata := spec.Metadata
			if metadata == nil {
				metadata = make(map[string]string)
			}
			config := spec.Config
			if config == nil {
				config = make(map[string]any)
			}
			apiResources = append(apiResources, api.ResourceProviderResource{
				Identifier: spec.Identifier,
				Name:       spec.DisplayName,
				Kind:       spec.Kind,
				Version:    spec.Version,
				Config:     config,
				Metadata:   metadata,
			})
		}

		log.Debug("Upserting resources", "workspaceID", ctx.WorkspaceIDValue(), "provider", providerName, "providerID", providerID)
		resp, err := ctx.APIClient().SetResourceProviderResourcesWithResponse(
			ctx.Ctx(), ctx.WorkspaceIDValue(), providerID,
			api.SetResourceProviderResourcesJSONRequestBody{Resources: apiResources},
		)
		if err != nil {
			for _, spec := range group {
				results = append(results, Result{
					Type:  resourceTypeName,
					Name:  spec.DisplayName,
					Error: fmt.Errorf("failed to upsert resources: %w", err),
				})
			}
			continue
		}
		if resp.StatusCode() != http.StatusAccepted {
			for _, spec := range group {
				results = append(results, Result{
					Type:  resourceTypeName,
					Name:  spec.DisplayName,
					Error: fmt.Errorf("failed to upsert resources: %s", resp.Status()),
				})
			}
			continue
		}

		for _, spec := range group {
			result := Result{
				Type:   resourceTypeName,
				Name:   spec.DisplayName,
				ID:     spec.Identifier,
				Action: "upserted",
			}
			if err := spec.syncVariables(ctx); err != nil {
				result.Error = err
			}
			results = append(results, result)
		}
	}

	return results
}

func (r *ResourceItemSpec) upsertWithoutProvider(ctx Context) Result {
	result := Result{
		Type: resourceTypeName,
		Name: r.DisplayName,
	}

	metadata := r.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	config := r.Config
	if config == nil {
		config = make(map[string]any)
	}

	log.Debug("Upserting resource directly (no provider)", "identifier", r.Identifier)
	resp, err := ctx.APIClient().UpsertResourceByIdentifierWithResponse(
		ctx.Ctx(), ctx.WorkspaceIDValue(), r.Identifier,
		api.UpsertResourceByIdentifierJSONRequestBody{
			Name:     r.DisplayName,
			Kind:     r.Kind,
			Version:  r.Version,
			Config:   &config,
			Metadata: &metadata,
		},
	)
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert resource: %w", err)
		return result
	}
	if resp.StatusCode() != http.StatusAccepted {
		result.Error = fmt.Errorf("failed to upsert resource: %s", resp.HTTPResponse.Status)
		return result
	}

	result.ID = r.Identifier
	result.Action = "upserted"
	if err := r.syncVariables(ctx); err != nil {
		result.Error = err
	}
	return result
}

func (r *ResourceItemSpec) syncVariables(ctx Context) error {
	vars := r.Variables
	if vars == nil {
		vars = make(map[string]any)
	}

	err := retry.Do(
		func() error {
			varsResp, err := ctx.APIClient().RequestResourceVariablesUpdateWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), r.Identifier, api.RequestResourceVariablesUpdateJSONRequestBody(vars))
			if err != nil {
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: %w", err))
			}
			if varsResp == nil {
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: empty response"))
			}
			if varsResp.StatusCode() == 404 {
				return fmt.Errorf("resource not found yet, retrying")
			}
			if varsResp.StatusCode() != 202 {
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
