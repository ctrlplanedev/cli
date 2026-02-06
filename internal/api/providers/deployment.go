package providers

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"gopkg.in/yaml.v3"
)

const deploymentTypeName = "Deployment"

type DeploymentProvider struct{}

func init() {
	RegisterProvider(&DeploymentProvider{})
}

func (p *DeploymentProvider) TypeName() string {
	return deploymentTypeName
}

func (p *DeploymentProvider) Order() int {
	return 700
}

func (p *DeploymentProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec DeploymentSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse deployment document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("deployment document missing required 'name' field")
	}
	if spec.System == "" {
		return nil, fmt.Errorf("deployment document missing required 'system' field")
	}
	return &spec, nil
}

type AgentConfig struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config"`
}

type DeploymentSpec struct {
	Type             string            `yaml:"type,omitempty"`
	DisplayName      string            `yaml:"name"`
	System           string            `yaml:"system"`
	Slug             *string           `yaml:"slug,omitempty"`
	Description      *string           `yaml:"description,omitempty"`
	ResourceSelector *string           `yaml:"resourceSelector,omitempty"`
	Metadata         map[string]string `yaml:"metadata,omitempty"`
	Agent            *AgentConfig      `yaml:"agent,omitempty"`
}

func (d *DeploymentSpec) Name() string {
	return d.DisplayName
}

func (d *DeploymentSpec) Identity() string {
	return fmt.Sprintf("%s/%s", d.System, d.slugValue())
}

func (d *DeploymentSpec) Lookup(ctx Context) (string, error) {
	systemID, err := d.resolveSystemID(ctx)
	if err != nil {
		return "", err
	}

	deploymentsResp, err := ctx.APIClient().
		ListDeploymentsWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), &api.ListDeploymentsParams{})
	if err != nil {
		return "", fmt.Errorf("failed to list deployments: %w", err)
	}
	if deploymentsResp.JSON200 == nil {
		return "", fmt.Errorf("failed to list deployments: %s", string(deploymentsResp.Body))
	}

	deploymentSlug := d.slugValue()
	systemIDValue := systemID.String()
	for _, dep := range deploymentsResp.JSON200.Items {
		if dep.Deployment.Slug == deploymentSlug && dep.Deployment.SystemId == systemIDValue {
			return dep.Deployment.Id, nil
		}
	}

	return "", nil
}

func (d *DeploymentSpec) Create(ctx Context, id string) error {
	return d.upsert(ctx, id)
}

func (d *DeploymentSpec) Update(ctx Context, existingID string) error {
	return d.upsert(ctx, existingID)
}

func (d *DeploymentSpec) Delete(ctx Context, existingID string) error {
	deleteResp, err := ctx.APIClient().RequestDeploymentDeletionWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), existingID)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	if deleteResp.StatusCode() == 404 {
		return nil
	}

	if deleteResp.StatusCode() >= 400 {
		return fmt.Errorf("failed to delete deployment: %s", string(deleteResp.Body))
	}

	return nil
}

func (d *DeploymentSpec) upsert(ctx Context, id string) error {
	systemID, err := d.resolveSystemID(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve system '%s': %w", d.System, err)
	}

	resourceSelector, err := buildDeploymentSelector(d.ResourceSelector)
	if err != nil {
		return fmt.Errorf("failed to build resource selector: %w", err)
	}

	var jobAgentID *string
	if d.Agent != nil && d.Agent.Ref != "" {
		agentID, err := d.resolveJobAgentID(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve job agent '%s': %w", d.Agent.Ref, err)
		}
		agentIDValue := agentID.String()
		jobAgentID = &agentIDValue
	}

	jobAgentConfig := map[string]any{}
	if d.Agent != nil {
		jobAgentConfig = d.Agent.Config
	}

	upsertReq := api.RequestDeploymentCreationJSONRequestBody{
		Name:             d.DisplayName,
		Slug:             d.slugValue(),
		SystemId:         systemID.String(),
		Description:      d.Description,
		ResourceSelector: resourceSelector,
		JobAgentId:       jobAgentID,
		JobAgentConfig:   &jobAgentConfig,
	}

	upsertResp, err := ctx.APIClient().
		RequestDeploymentCreationWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), upsertReq)
	if err != nil {
		return fmt.Errorf("failed to upsert deployment: %w", err)
	}
	if upsertResp.JSON202 == nil {
		return fmt.Errorf("failed to upsert deployment: %s", string(upsertResp.Body))
	}

	return nil
}

func (d *DeploymentSpec) resolveSystemID(ctx Context) (uuid.UUID, error) {
	resolver := ctx.ResolverProvider()
	if resolver == nil {
		return uuid.Nil, fmt.Errorf("resolver is not configured")
	}
	return ctx.ResolverProvider().ResolveSystemID(ctx.Ctx(), d.System)
}

func (d *DeploymentSpec) resolveJobAgentID(ctx Context) (uuid.UUID, error) {
	resolver := ctx.ResolverProvider()
	if resolver == nil {
		return uuid.Nil, fmt.Errorf("resolver is not configured")
	}
	return ctx.ResolverProvider().ResolveJobAgentID(ctx.Ctx(), d.Agent.Ref)
}

func (d *DeploymentSpec) slugValue() string {
	deploymentSlug := slug.Make(d.DisplayName)
	if d.Slug != nil && *d.Slug != "" {
		deploymentSlug = *d.Slug
	}
	return deploymentSlug
}

func buildDeploymentSelector(raw *string) (*api.Selector, error) {
	if raw == nil {
		return nil, nil
	}

	var selector api.Selector
	if err := selector.FromCelSelector(api.CelSelector{Cel: *raw}); err != nil {
		return nil, fmt.Errorf("failed to create CEL selector: %w", err)
	}
	return &selector, nil
}
