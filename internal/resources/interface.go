package resources

import (
	"context"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
)

// Filters describes the optional server-side filters for a resource search.
// It decouples commands from the proto request shape.
type Filters struct {
	Kinds       []string
	Versions    []string
	ProviderIDs []string
	Metadata    map[string]string
}

// ResourceService abstracts resource retrieval operations.
// This interface decouples commands from the generated API client,
// enabling easy swapping when API changes happen or (more) straightforward test mocking.
type ResourceService interface {
	GetByIdentifier(ctx context.Context, identifier string) (*apiv1.Resource, error)
	Search(ctx context.Context, filters Filters) ([]*apiv1.Resource, error)
	DeleteByIdentifier(ctx context.Context, identifier string) (*apiv1.ResourceMutationResponse, error)
}
