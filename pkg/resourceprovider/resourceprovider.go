package resourceprovider

import (
	"context"
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func New(client *api.Client, workspace string, name string) (*ResourceProvider, error) {
	ctx := context.Background()
	workspaceUUID, err := client.GetWorkspaceID(ctx, workspace)
	if err != nil {
		return nil, err
	}
	workspaceId := workspaceUUID.String()

	log.Debug("Upserting resource provider", "workspaceId", workspaceId, "name", name)

	resp, err := client.Resource.UpsertResourceProvider(ctx, connect.NewRequest(&apiv1.UpsertResourceProviderRequest{
		WorkspaceId: workspaceId,
		Name:        name,
	}))
	if err != nil {
		log.Error("Failed to upsert resource provider",
			"error", err,
			"workspaceId", workspaceId,
			"name", name)
		return nil, fmt.Errorf("failed to upsert resource provider: %w", err)
	}

	provider := resp.Msg
	log.Debug("Successfully created resource provider",
		"id", provider.GetId(),
		"name", name)

	return &ResourceProvider{
		Name:        name,
		ID:          provider.GetId(),
		client:      client,
		workspaceId: workspaceId,
	}, nil
}

type ResourceProvider struct {
	ID          string
	Name        string
	client      *api.Client
	workspaceId string
}

func (r *ResourceProvider) UpsertResource(ctx context.Context, resources []*apiv1.ResourceInput) (*apiv1.SetResourceProviderResourcesResponse, error) {
	upsertResp, err := r.client.Resource.SetResourceProviderResources(
		ctx,
		connect.NewRequest(&apiv1.SetResourceProviderResourcesRequest{
			WorkspaceId: r.workspaceId,
			ProviderId:  r.ID,
			Resources:   resources,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert resources: %w", err)
	}
	return upsertResp.Msg, nil
}

// func (r *ResourceProvider) AddResourceRelationshipRule(ctx context.Context, rules []api.ResourceProviderResourceRelationshipRule) error {
// 	for _, rule := range rules {
// 		rule.WorkspaceId = r.workspaceId
// 		resp, err := r.client.CreateResourceRelationshipRuleWithResponse(ctx, rule)
// 		if resp.StatusCode() == http.StatusConflict {
// 			log.Info("Resource relationship rule already exists, skipped creation")
// 			return nil
// 		}
// 		if err != nil {
// 			return err
// 		}
// 		if resp.StatusCode() != http.StatusOK {
// 			return fmt.Errorf("failed to upsert resource relationship rule: %s", string(resp.Body))
// 		}
// 	}
// 	log.Info("Successfully created resource relationship rules")
// 	return nil
// }
