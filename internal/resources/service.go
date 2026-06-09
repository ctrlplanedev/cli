package resources

import (
	"context"
	"fmt"
	"time"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

const pageSize = 200

// APIResourceService implements ResourceService using the Connect API client.
type APIResourceService struct {
	Client      *api.Client
	WorkspaceID string
}

// NewAPIResourceService creates an APIResourceService by initializing the API
// client and resolving the workspace ID from a slug or UUID.
func NewAPIResourceService(ctx context.Context, apiURL, apiKey, workspace string) (*APIResourceService, error) {
	client, err := api.NewConnectClient(apiURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID, err := client.GetWorkspaceID(ctx, workspace)
	if err != nil {
		return nil, err
	}
	log.Debug("resolved workspace", "input", workspace, "workspaceID", workspaceID.String())

	return &APIResourceService{
		Client:      client,
		WorkspaceID: workspaceID.String(),
	}, nil
}

func (s *APIResourceService) GetByIdentifier(ctx context.Context, identifier string) (*apiv1.Resource, error) {
	log.Debug("GetByIdentifier", "workspaceID", s.WorkspaceID, "identifier", identifier)
	start := time.Now()
	resp, err := s.Client.Resource.GetResourceByIdentifier(ctx, connect.NewRequest(&apiv1.GetResourceByIdentifierRequest{
		WorkspaceId: s.WorkspaceID,
		Identifier:  identifier,
	}))
	elapsed := time.Since(start)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, fmt.Errorf("resource %q not found", identifier)
		}
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	log.Debug("GetByIdentifier response", "duration", elapsed)
	return resp.Msg, nil
}

func (s *APIResourceService) Search(ctx context.Context, filters Filters) ([]*apiv1.Resource, error) {
	searchStart := time.Now()
	var allItems []*apiv1.Resource
	var offset int32
	const limit int32 = pageSize

	log.Debug("Search", "workspaceID", s.WorkspaceID, "filters", filters, "pageSize", limit)

	for {
		req := &apiv1.SearchResourcesRequest{
			WorkspaceId: s.WorkspaceID,
			Kinds:       filters.Kinds,
			Versions:    filters.Versions,
			ProviderIds: filters.ProviderIDs,
			Metadata:    filters.Metadata,
			Limit:       limit,
			Offset:      offset,
		}

		log.Debug("Search request", "offset", offset, "limit", limit)
		start := time.Now()
		resp, err := s.Client.Resource.SearchResources(ctx, connect.NewRequest(req))
		elapsed := time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("failed to search resources: %w", err)
		}

		msg := resp.Msg
		log.Debug("Search response", "items", len(msg.GetItems()), "total", msg.GetTotal(), "offset", msg.GetOffset(), "duration", elapsed)
		allItems = append(allItems, msg.GetItems()...)

		if offset+limit >= msg.GetTotal() {
			break
		}
		offset += limit
	}

	log.Debug("Search complete", "totalFetched", len(allItems), "duration", time.Since(searchStart))
	return allItems, nil
}

func (s *APIResourceService) DeleteByIdentifier(ctx context.Context, identifier string) (*apiv1.ResourceMutationResponse, error) {
	log.Debug("DeleteByIdentifier", "workspaceID", s.WorkspaceID, "identifier", identifier)
	start := time.Now()
	resp, err := s.Client.Resource.DeleteResourceByIdentifier(ctx, connect.NewRequest(&apiv1.DeleteResourceByIdentifierRequest{
		WorkspaceId: s.WorkspaceID,
		Identifier:  identifier,
	}))
	elapsed := time.Since(start)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, fmt.Errorf("resource %q not found", identifier)
		}
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}
	log.Debug("DeleteByIdentifier response", "duration", elapsed)
	return resp.Msg, nil
}
