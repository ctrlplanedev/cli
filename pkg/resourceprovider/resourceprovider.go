package resourceprovider

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func New(client *api.ClientWithResponses, workspace string, name string) (*ResourceProvider, error) {
	ctx := context.Background()
	workspaceId := client.GetWorkspaceID(ctx, workspace).String()

	log.Debug("Upserting resource provider", "workspaceId", workspaceId, "name", name)

	resp, err := client.RequestResourceProviderUpsertWithResponse(ctx, workspaceId, api.RequestResourceProviderUpsertJSONRequestBody{
		Name: name,
	})
	if err != nil {
		log.Error("Failed to upsert resource provider",
			"error", err,
			"workspaceId", workspaceId,
			"name", name,
			"status", resp.StatusCode,
			"body", string(resp.Body))
		return nil, fmt.Errorf("failed to upsert resource provider: %w", err)
	}

	log.Debug("Got response from upserting resource provider",
		"status", resp.StatusCode,
		"body", string(resp.Body))

	if resp.JSON200 == nil {
		log.Error("Invalid response from upserting resource provider",
			"status", resp.StatusCode(),
			"body", string(resp.Body))
		return nil, fmt.Errorf("failed to upsert resource provider: %s", string(resp.Body))
	}

	provider := resp.JSON200
	log.Debug("Successfully created resource provider",
		"id", provider.Id,
		"name", name)

	return &ResourceProvider{
		Name:        name,
		ID:          provider.Id,
		client:      client,
		workspaceId: workspaceId,
	}, nil
}

type ResourceProvider struct {
	ID          string
	Name        string
	client      *api.ClientWithResponses
	workspaceId string
}

func (r *ResourceProvider) UpsertResource(ctx context.Context, resources []api.ResourceProviderResource) (*http.Response, error) {
	upsertResp, err := r.client.SetResourceProviderResources(
		ctx,
		r.workspaceId,
		r.ID,
		api.SetResourceProviderResourcesJSONRequestBody{
			Resources: resources,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert resource: %w", err)
	}
	if upsertResp.StatusCode >= 400 {
		defer upsertResp.Body.Close()
		body, _ := io.ReadAll(upsertResp.Body)
		return upsertResp, fmt.Errorf("failed to upsert resources (HTTP %d): %s", upsertResp.StatusCode, string(body))
	}
	return upsertResp, nil
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
