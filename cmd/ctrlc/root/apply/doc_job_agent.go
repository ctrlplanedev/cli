package apply

import (
	"encoding/json"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	TypeJobAgent DocumentType = "JobAgent"
)

type JobAgentDocument struct {
	BaseDocument `yaml:",inline"`
	Description  *string           `yaml:"description,omitempty"`
	Config       map[string]any    `yaml:"config,omitempty"`
	Metadata     map[string]string `yaml:"metadata,omitempty"`
}

// AgentType extracts the type from the config map
func (d *JobAgentDocument) AgentType() string {
	if d.Config == nil {
		return ""
	}
	if t, ok := d.Config["type"].(string); ok {
		return t
	}
	return ""
}

func ParseJobAgent(raw []byte) (*JobAgentDocument, error) {
	var doc JobAgentDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse job agent document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("job agent document missing required 'name' field")
	}
	if doc.AgentType() == "" {
		return nil, fmt.Errorf("job agent document missing required 'config.type' field")
	}
	return &doc, nil
}

var _ Document = &JobAgentDocument{}

func (d *JobAgentDocument) Order() int {
	return 900 // High priority - job agents are processed first as deployments may reference them
}

func (d *JobAgentDocument) Apply(ctx *DocContext) (ApplyResult, error) {
	result := ApplyResult{
		Type: TypeJobAgent,
		Name: d.Name,
	}

	config := d.Config
	if config == nil {
		config = make(map[string]any)
	}

	// Get all job agents to check for existing by name.
	agentsResp, err := ctx.Client.ListJobAgentsWithResponse(ctx.Context, ctx.WorkspaceID, &api.ListJobAgentsParams{})
	if err != nil {
		result.Error = fmt.Errorf("failed to get existing job agents: %w", err)
		return result, result.Error
	}
	if agentsResp.JSON200 == nil {
		result.Error = fmt.Errorf("could not retrieve job agent list: %s", string(agentsResp.Body))
		return result, result.Error
	}

	var existingAgent *api.JobAgent
	for _, agent := range agentsResp.JSON200.Items {
		if agent.Name == d.Name {
			if existingAgent != nil {
				result.Error = fmt.Errorf("multiple job agents found with name '%s', unable to determine which to update", d.Name)
				return result, result.Error
			}

			result.ID = agent.Id
			result.Action = "updated"
			existingAgent = &agent
		}
	}

	jobAgentId := uuid.New().String()
	if existingAgent != nil {
		jobAgentId = existingAgent.Id
	}

	// Convert config map to JobAgentConfig
	configBytes, err := json.Marshal(config)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal job agent config: %w", err)
		return result, result.Error
	}
	var jobAgentConfig api.JobAgentConfig
	if err := jobAgentConfig.UnmarshalJSON(configBytes); err != nil {
		result.Error = fmt.Errorf("failed to parse job agent config: %w", err)
		return result, result.Error
	}

	opts := api.UpsertJobAgentJSONRequestBody{
		Name:   d.Name,
		Type:   d.AgentType(),
		Config: jobAgentConfig,
	}

	if d.Metadata != nil {
		opts.Metadata = &d.Metadata
	}

	upsertResp, err := ctx.Client.UpsertJobAgentWithResponse(ctx.Context, ctx.WorkspaceID, jobAgentId, opts)
	if err != nil {
		result.Error = fmt.Errorf("failed to upsert job agent: %w", err)
		return result, result.Error
	}
	if upsertResp.JSON202 == nil {
		result.Error = fmt.Errorf("failed to upsert job agent: %s", string(upsertResp.Body))
		return result, result.Error
	}
	result.ID = upsertResp.JSON202.Id
	if existingAgent != nil {
		result.Action = "updated"
	} else {
		result.Action = "created"
	}
	return result, nil
}

func (d *JobAgentDocument) Delete(ctx *DocContext) (DeleteResult, error) {
	result := DeleteResult{
		Type: TypeJobAgent,
		Name: d.Name,
	}
	result.Error = fmt.Errorf("failed to delete job agent")
	return result, result.Error
}
