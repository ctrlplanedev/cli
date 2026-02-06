package providers

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const environmentTypeName = "Environment"

type EnvironmentProvider struct{}

func init() {
	RegisterProvider(&EnvironmentProvider{})
}

func (p *EnvironmentProvider) TypeName() string {
	return environmentTypeName
}

func (p *EnvironmentProvider) Order() int {
	return 600
}

func (p *EnvironmentProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec EnvironmentSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse environment document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("environment document missing required 'name' field")
	}
	if spec.System == "" {
		return nil, fmt.Errorf("environment document missing required 'system' field")
	}
	return &spec, nil
}

type EnvironmentSpec struct {
	Type             string  `yaml:"type,omitempty"`
	DisplayName      string  `yaml:"name"`
	System           string  `yaml:"system"`
	Description      *string `yaml:"description,omitempty"`
	ResourceSelector *string `yaml:"resourceSelector,omitempty"`
}

func (e *EnvironmentSpec) Name() string {
	return e.DisplayName
}

func (e *EnvironmentSpec) Identity() string {
	return fmt.Sprintf("%s/%s", e.System, e.DisplayName)
}

func (e *EnvironmentSpec) Lookup(ctx Context) (string, error) {
	systemID, err := e.resolveSystemID(ctx)
	if err != nil {
		return "", err
	}

	environmentsResp, err := ctx.APIClient().ListEnvironmentsWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), &api.ListEnvironmentsParams{})
	if err != nil {
		return "", fmt.Errorf("failed to list environments: %w", err)
	}
	if environmentsResp.JSON200 == nil {
		return "", fmt.Errorf("failed to list environments: %s", string(environmentsResp.Body))
	}

	systemIDValue := systemID.String()
	var existingID string
	matchingCount := 0
	for _, env := range environmentsResp.JSON200.Items {
		if env.Name == e.DisplayName && env.SystemId == systemIDValue {
			matchingCount++
			if matchingCount == 1 {
				existingID = env.Id
			}
		}
	}

	if matchingCount > 1 {
		return "", fmt.Errorf("multiple environments found with name '%s' in system '%s', unable to determine which to update", e.DisplayName, e.System)
	}

	return existingID, nil
}

func (e *EnvironmentSpec) Create(ctx Context, id string) error {
	return e.upsert(ctx, id)
}

func (e *EnvironmentSpec) Update(ctx Context, existingID string) error {
	return e.upsert(ctx, existingID)
}

func (e *EnvironmentSpec) Delete(ctx Context, existingID string) error {
	deleteResp, err := ctx.APIClient().RequestEnvironmentDeletionWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), existingID)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	if deleteResp.StatusCode() == 404 {
		return nil
	}

	if deleteResp.StatusCode() >= 400 {
		return fmt.Errorf("failed to delete environment: %s", string(deleteResp.Body))
	}

	return nil
}

func (e *EnvironmentSpec) upsert(ctx Context, id string) error {
	systemID, err := e.resolveSystemID(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve system '%s': %w", e.System, err)
	}

	resourceSelector, err := buildEnvironmentSelector(e.ResourceSelector)
	if err != nil {
		return fmt.Errorf("failed to build resource selector: %w", err)
	}

	upsertReq := api.RequestEnvironmentCreationJSONRequestBody{
		Name:             e.DisplayName,
		SystemId:         systemID.String(),
		Description:      e.Description,
		ResourceSelector: resourceSelector,
	}

	upsertResp, err := ctx.APIClient().RequestEnvironmentCreationWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), upsertReq)
	if err != nil {
		return fmt.Errorf("failed to upsert environment: %w", err)
	}
	if upsertResp.JSON202 == nil {
		return fmt.Errorf("failed to upsert environment: %s", string(upsertResp.Body))
	}

	return nil
}

func (e *EnvironmentSpec) resolveSystemID(ctx Context) (uuid.UUID, error) {
	resolver := ctx.ResolverProvider()
	if resolver == nil {
		return uuid.Nil, fmt.Errorf("resolver is not configured")
	}
	return resolver.ResolveSystemID(ctx.Ctx(), e.System)
}

func buildEnvironmentSelector(raw *string) (*api.Selector, error) {
	if raw == nil {
		return nil, nil
	}

	var selector api.Selector
	if err := selector.FromCelSelector(api.CelSelector{Cel: *raw}); err != nil {
		return nil, fmt.Errorf("failed to create CEL selector: %w", err)
	}
	return &selector, nil
}
