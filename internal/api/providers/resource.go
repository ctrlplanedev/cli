package providers

import (
	"fmt"
	"time"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
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

	resources := []*apiv1.ResourceInput{
		{
			Identifier: r.Identifier,
			Name:       r.DisplayName,
			Kind:       r.Kind,
			Version:    r.Version,
			Config:     api.NewStruct(config),
			Metadata:   metadata,
		},
	}

	_, err = ctx.APIClient().Resource.SetResourceProviderResources(ctx.Ctx(), connect.NewRequest(&apiv1.SetResourceProviderResourcesRequest{
		WorkspaceId: ctx.WorkspaceIDValue(),
		ProviderId:  providerID,
		Resources:   resources,
	}))
	if err != nil {
		return fmt.Errorf("failed to upsert resource: %w", err)
	}

	return r.syncVariables(ctx)
}

func (r *ResourceItemSpec) getProviderID(ctx Context) (string, error) {
	providerName := r.Provider
	if providerName == "" {
		providerName = "ctrlc-apply"
	}

	providerResp, err := ctx.APIClient().Resource.GetResourceProviderByName(ctx.Ctx(), connect.NewRequest(&apiv1.GetResourceProviderByNameRequest{
		WorkspaceId: ctx.WorkspaceIDValue(),
		Name:        providerName,
	}))
	if err == nil {
		return providerResp.Msg.GetId(), nil
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		return "", fmt.Errorf("failed to get resource provider: %w", err)
	}

	createResp, err := ctx.APIClient().Resource.UpsertResourceProvider(ctx.Ctx(), connect.NewRequest(&apiv1.UpsertResourceProviderRequest{
		WorkspaceId: ctx.WorkspaceIDValue(),
		Name:        providerName,
	}))
	if err != nil {
		return "", fmt.Errorf("failed to create resource provider: %w", err)
	}
	return createResp.Msg.GetId(), nil
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

		apiResources := make([]*apiv1.ResourceInput, 0, len(group))
		for _, spec := range group {
			metadata := spec.Metadata
			if metadata == nil {
				metadata = make(map[string]string)
			}
			config := spec.Config
			if config == nil {
				config = make(map[string]any)
			}
			apiResources = append(apiResources, &apiv1.ResourceInput{
				Identifier: spec.Identifier,
				Name:       spec.DisplayName,
				Kind:       spec.Kind,
				Version:    spec.Version,
				Config:     api.NewStruct(config),
				Metadata:   metadata,
			})
		}

		log.Debug("Upserting resources", "workspaceID", ctx.WorkspaceIDValue(), "provider", providerName, "providerID", providerID)
		_, err = ctx.APIClient().Resource.SetResourceProviderResources(
			ctx.Ctx(), connect.NewRequest(&apiv1.SetResourceProviderResourcesRequest{
				WorkspaceId: ctx.WorkspaceIDValue(),
				ProviderId:  providerID,
				Resources:   apiResources,
			}),
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
	_, err := ctx.APIClient().Resource.UpsertResourceByIdentifier(
		ctx.Ctx(), connect.NewRequest(&apiv1.UpsertResourceByIdentifierRequest{
			WorkspaceId: ctx.WorkspaceIDValue(),
			Identifier:  r.Identifier,
			Name:        r.DisplayName,
			Kind:        r.Kind,
			Version:     r.Version,
			Config:      api.NewStruct(config),
			Metadata:    metadata,
		}),
	)
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert resource: %w", err)
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
			_, err := ctx.APIClient().Resource.ReplaceResourceVariables(ctx.Ctx(), connect.NewRequest(&apiv1.ReplaceResourceVariablesRequest{
				WorkspaceId:        ctx.WorkspaceIDValue(),
				ResourceIdentifier: r.Identifier,
				Variables:          api.NewStruct(vars).GetFields(),
			}))
			if err != nil {
				// The resource may not be queryable immediately after upsert;
				// retry on NotFound, treat everything else as terminal.
				if connect.CodeOf(err) == connect.CodeNotFound {
					return fmt.Errorf("resource not found yet, retrying")
				}
				return retry.Unrecoverable(fmt.Errorf("failed to update resource variables: %w", err))
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
