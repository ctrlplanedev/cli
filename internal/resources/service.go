package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

const pageSize = 200

// APIResourceService implements ResourceService using the generated API client.
type APIResourceService struct {
	Client      api.ClientWithResponsesInterface
	WorkspaceID string
}

// NewAPIResourceService creates an APIResourceService by initializing the API
// client and resolving the workspace ID from a slug or UUID.
func NewAPIResourceService(ctx context.Context, apiURL, apiKey, workspace string) (*APIResourceService, error) {
	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)
	log.Debug("resolved workspace", "input", workspace, "workspaceID", workspaceID.String())

	return &APIResourceService{
		Client:      client,
		WorkspaceID: workspaceID.String(),
	}, nil
}

func (s *APIResourceService) GetByIdentifier(ctx context.Context, identifier string) (*api.Resource, error) {
	log.Debug("GetByIdentifier", "workspaceID", s.WorkspaceID, "identifier", identifier)
	start := time.Now()
	resp, err := s.Client.GetResourceByIdentifierWithResponse(ctx, s.WorkspaceID, identifier)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	if resp.JSON200 == nil {
		log.Debug("GetByIdentifier response", "status", resp.Status(), "body", string(resp.Body), "duration", elapsed)
		return nil, fmt.Errorf("unexpected response status: %s", resp.Status())
	}
	log.Debug("GetByIdentifier response", "status", resp.Status(), "duration", elapsed)
	return resp.JSON200, nil
}

func (s *APIResourceService) List(ctx context.Context, cel *string) ([]api.Resource, error) {
	listStart := time.Now()
	var allItems []api.Resource
	offset := 0
	limit := pageSize

	celStr := "<nil>"
	if cel != nil {
		celStr = *cel
	}
	log.Debug("List", "workspaceID", s.WorkspaceID, "cel", celStr, "pageSize", limit)

	for {
		params := &api.GetAllResourcesParams{
			Limit:  &limit,
			Offset: &offset,
			Cel:    cel,
		}

		log.Debug("List request", "offset", offset, "limit", limit)
		start := time.Now()
		resp, err := s.Client.GetAllResourcesWithResponse(ctx, s.WorkspaceID, params)
		elapsed := time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}
		if resp.JSON200 == nil {
			log.Debug("List response error", "status", resp.Status(), "body", string(resp.Body), "duration", elapsed)
			return nil, fmt.Errorf("unexpected response status: %s", resp.Status())
		}

		log.Debug("List response", "items", len(resp.JSON200.Items), "total", resp.JSON200.Total, "offset", resp.JSON200.Offset, "duration", elapsed)
		allItems = append(allItems, resp.JSON200.Items...)

		if offset+limit >= resp.JSON200.Total {
			break
		}
		offset += limit
	}

	log.Debug("List complete", "totalFetched", len(allItems), "duration", time.Since(listStart))
	return allItems, nil
}

func (s *APIResourceService) GetTotal(ctx context.Context) (int, error) {
	log.Debug("GetTotal", "workspaceID", s.WorkspaceID)
	limit := 1
	params := &api.GetAllResourcesParams{
		Limit: &limit,
	}

	start := time.Now()
	resp, err := s.Client.GetAllResourcesWithResponse(ctx, s.WorkspaceID, params)
	elapsed := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("failed to get resource count: %w", err)
	}
	if resp.JSON200 == nil {
		log.Debug("GetTotal response error", "status", resp.Status(), "body", string(resp.Body), "duration", elapsed)
		return 0, fmt.Errorf("unexpected response status: %s", resp.Status())
	}

	log.Debug("GetTotal result", "total", resp.JSON200.Total, "duration", elapsed)
	return resp.JSON200.Total, nil
}

func (s *APIResourceService) DeleteByIdentifier(ctx context.Context, identifier string) (*api.ResourceRequestAccepted, error) {
	log.Debug("DeleteByIdentifier", "workspaceID", s.WorkspaceID, "identifier", identifier)
	start := time.Now()
	resp, err := s.Client.RequestResourceDeletionByIdentifierWithResponse(ctx, s.WorkspaceID, identifier)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	// The generated client expects 202 but the API actually returns 200.
	// Handle both cases.
	if resp.JSON202 != nil {
		log.Debug("DeleteByIdentifier response", "status", resp.Status(), "duration", elapsed)
		return resp.JSON202, nil
	}

	statusCode := resp.HTTPResponse.StatusCode
	if statusCode == 200 {
		var result api.ResourceRequestAccepted
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse delete response: %w", err)
		}
		log.Debug("DeleteByIdentifier response", "status", resp.Status(), "duration", elapsed)
		return &result, nil
	}

	log.Debug("DeleteByIdentifier response error", "status", resp.Status(), "body", string(resp.Body), "duration", elapsed)
	return nil, fmt.Errorf("unexpected response status: %s", resp.Status())
}
