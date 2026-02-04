package resolver

import (
	"context"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
)

type APIResolver struct {
	client      *api.ClientWithResponses
	workspaceID uuid.UUID
	systemCache map[string]uuid.UUID
	jobCache    map[string]uuid.UUID
}

func NewAPIResolver(client *api.ClientWithResponses, workspaceID uuid.UUID) *APIResolver {
	return &APIResolver{
		client:      client,
		workspaceID: workspaceID,
		systemCache: make(map[string]uuid.UUID),
		jobCache:    make(map[string]uuid.UUID),
	}
}

func NewAPIResolverFromWorkspace(ctx context.Context, client *api.ClientWithResponses, workspace string) (*APIResolver, error) {
	workspaceID := client.GetWorkspaceID(ctx, workspace)
	if workspaceID == uuid.Nil {
		return nil, fmt.Errorf("workspace not found: %s", workspace)
	}
	return NewAPIResolver(client, workspaceID), nil
}

func (r *APIResolver) ResolveSystemID(ctx context.Context, nameOrID string) (uuid.UUID, error) {
	if parsed, err := uuid.Parse(nameOrID); err == nil {
		return parsed, nil
	}

	if id, ok := r.systemCache[nameOrID]; ok {
		return id, nil
	}

	resp, err := r.client.ListSystemsWithResponse(ctx, r.workspaceID.String(), nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list systems: %w", err)
	}
	if resp.JSON200 == nil {
		return uuid.Nil, fmt.Errorf("failed to list systems: %s", string(resp.Body))
	}

	for _, sys := range resp.JSON200.Items {
		systemID, err := uuid.Parse(sys.Id)
		if err != nil {
			continue
		}
		r.systemCache[sys.Name] = systemID
		if sys.Name == nameOrID || sys.Slug == nameOrID {
			return systemID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("system not found: %s", nameOrID)
}

func (r *APIResolver) ResolveJobAgentID(ctx context.Context, nameOrID string) (uuid.UUID, error) {
	if parsed, err := uuid.Parse(nameOrID); err == nil {
		return parsed, nil
	}

	if id, ok := r.jobCache[nameOrID]; ok {
		return id, nil
	}

	resp, err := r.client.ListJobAgentsWithResponse(ctx, r.workspaceID.String(), &api.ListJobAgentsParams{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list job agents: %w", err)
	}
	if resp.JSON200 == nil {
		return uuid.Nil, fmt.Errorf("failed to list job agents: %s", string(resp.Body))
	}

	for _, agent := range resp.JSON200.Items {
		agentID, err := uuid.Parse(agent.Id)
		if err != nil {
			continue
		}
		r.jobCache[agent.Name] = agentID
		if agent.Name == nameOrID {
			return agentID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("job agent not found: %s", nameOrID)
}
