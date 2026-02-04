package providers

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const systemTypeName = "System"

type SystemProvider struct{}

func init() {
	RegisterProvider(&SystemProvider{})
}

func (p *SystemProvider) TypeName() string {
	return systemTypeName
}

func (p *SystemProvider) Order() int {
	return 900
}

func (p *SystemProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec SystemSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse system document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("system document missing required 'name' field")
	}
	return &spec, nil
}

type SystemSpec struct {
	Type        string  `yaml:"type,omitempty"`
	DisplayName string  `yaml:"name"`
	Description *string `yaml:"description,omitempty"`
}

func (s *SystemSpec) Name() string {
	return s.DisplayName
}

func (s *SystemSpec) Identity() string {
	return s.DisplayName
}

func (s *SystemSpec) Lookup(ctx Context) (string, error) {
	listResp, err := ctx.APIClient().ListSystemsWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to list systems: %w", err)
	}
	if listResp.JSON200 == nil {
		return "", fmt.Errorf("failed to list systems: %s", string(listResp.Body))
	}

	for _, sys := range listResp.JSON200.Items {
		if sys.Name == s.DisplayName || sys.Slug == s.DisplayName {
			return sys.Id, nil
		}
	}

	return "", nil
}

func (s *SystemSpec) Create(ctx Context, id string) error {
	return s.upsert(ctx, id)
}

func (s *SystemSpec) Update(ctx Context, existingID string) error {
	return s.upsert(ctx, existingID)
}

func (s *SystemSpec) Delete(ctx Context, existingID string) error {
	resp, err := ctx.APIClient().DeleteSystemWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), existingID)
	if err != nil {
		return fmt.Errorf("failed to delete system: %w", err)
	}
	if resp.StatusCode() == 404 {
		return nil
	}
	if resp.StatusCode() >= 400 {
		return fmt.Errorf("failed to delete system: %s", string(resp.Body))
	}
	return nil
}

func (s *SystemSpec) upsert(ctx Context, id string) error {
	upsertReq := api.UpsertSystemByIdJSONRequestBody{
		Name:        s.DisplayName,
		Description: s.Description,
	}

	resp, err := ctx.APIClient().UpsertSystemByIdWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), id, upsertReq)
	if err != nil {
		return fmt.Errorf("failed to upsert system: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("failed to upsert system: %s", string(resp.Body))
	}

	return nil
}
