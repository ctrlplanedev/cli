package providers

import (
	"encoding/json"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const jobAgentTypeName = "JobAgent"

type JobAgentProvider struct{}

func init() {
	RegisterProvider(&JobAgentProvider{})
}

func (p *JobAgentProvider) TypeName() string {
	return jobAgentTypeName
}

func (p *JobAgentProvider) Order() int {
	return 900
}

func (p *JobAgentProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec JobAgentSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse job agent document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("job agent document missing required 'name' field")
	}
	if spec.AgentType() == "" {
		return nil, fmt.Errorf("job agent document missing required 'config.type' field")
	}
	return &spec, nil
}

type JobAgentSpec struct {
	Type        string            `yaml:"type,omitempty"`
	DisplayName string            `yaml:"name"`
	Description *string           `yaml:"description,omitempty"`
	Config      map[string]any    `yaml:"config,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

func (j *JobAgentSpec) Name() string {
	return j.DisplayName
}

func (j *JobAgentSpec) Identity() string {
	return j.DisplayName
}

func (j *JobAgentSpec) AgentType() string {
	if j.Config == nil {
		return ""
	}
	if t, ok := j.Config["type"].(string); ok {
		return t
	}
	return ""
}

func (j *JobAgentSpec) Lookup(ctx Context) (string, error) {
	agentsResp, err := ctx.APIClient().ListJobAgentsWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), &api.ListJobAgentsParams{})
	if err != nil {
		return "", fmt.Errorf("failed to get existing job agents: %w", err)
	}
	if agentsResp.JSON200 == nil {
		return "", fmt.Errorf("could not retrieve job agent list: %s", string(agentsResp.Body))
	}

	var existingID string
	matchingCount := 0
	for _, agent := range agentsResp.JSON200.Items {
		if agent.Name == j.DisplayName {
			existingID = agent.Id
			matchingCount++
		}
	}

	if matchingCount > 1 {
		return "", fmt.Errorf("multiple job agents found with name '%s', unable to determine which to update", j.DisplayName)
	}

	return existingID, nil
}

func (j *JobAgentSpec) Create(ctx Context, id string) error {
	return j.upsert(ctx, id)
}

func (j *JobAgentSpec) Update(ctx Context, existingID string) error {
	return j.upsert(ctx, existingID)
}

func (j *JobAgentSpec) Delete(ctx Context, existingID string) error {
	return fmt.Errorf("failed to delete job agent")
}

func (j *JobAgentSpec) upsert(ctx Context, id string) error {
	config := j.Config
	if config == nil {
		config = make(map[string]any)
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal job agent config: %w", err)
	}

	var jobAgentConfig map[string]any
	if err := json.Unmarshal(configBytes, &jobAgentConfig); err != nil {
		return fmt.Errorf("failed to parse job agent config: %w", err)
	}

	jobAgentID := id
	if jobAgentID == "" {
		jobAgentID = uuid.New().String()
	}

	opts := api.UpsertJobAgentJSONRequestBody{
		Name:   j.DisplayName,
		Type:   j.AgentType(),
		Config: jobAgentConfig,
	}

	if j.Metadata != nil {
		opts.Metadata = &j.Metadata
	}

	upsertResp, err := ctx.APIClient().UpsertJobAgentWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), jobAgentID, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert job agent: %w", err)
	}
	if upsertResp.JSON202 == nil {
		return fmt.Errorf("failed to upsert job agent: %s", string(upsertResp.Body))
	}

	return nil
}
