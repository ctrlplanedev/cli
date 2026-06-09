package resolver

import (
	"context"
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
)

type APIResolver struct {
	client      *api.Client
	workspaceID uuid.UUID
	systemCache map[string]uuid.UUID
	jobCache    map[string]uuid.UUID
}

func NewAPIResolver(client *api.Client, workspaceID uuid.UUID) *APIResolver {
	return &APIResolver{
		client:      client,
		workspaceID: workspaceID,
		systemCache: make(map[string]uuid.UUID),
		jobCache:    make(map[string]uuid.UUID),
	}
}

func NewAPIResolverFromWorkspace(ctx context.Context, client *api.Client, workspace string) (*APIResolver, error) {
	workspaceID, err := client.GetWorkspaceID(ctx, workspace)
	if err != nil {
		return nil, err
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

	resp, err := r.client.System.ListSystems(ctx, connect.NewRequest(&apiv1.ListSystemsRequest{
		WorkspaceId: r.workspaceID.String(),
	}))
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list systems: %w", err)
	}

	for _, sys := range resp.Msg.GetItems() {
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

	resp, err := r.client.Job.ListJobAgents(ctx, connect.NewRequest(&apiv1.ListJobAgentsRequest{
		WorkspaceId: r.workspaceID.String(),
	}))
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list job agents: %w", err)
	}

	for _, agent := range resp.Msg.GetItems() {
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
