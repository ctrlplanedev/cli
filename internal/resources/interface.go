package resources

import (
	"context"

	"github.com/ctrlplanedev/cli/internal/api"
)

// ResourceService abstracts resource retrieval operations.
// This interface decouples commands from the generated API client,
// enabling easy swapping when API changes happen or (more) straightforward test mocking.
type ResourceService interface {
	GetByIdentifier(ctx context.Context, identifier string) (*api.Resource, error)
	List(ctx context.Context, cel *string) ([]api.Resource, error)
	GetTotal(ctx context.Context) (int, error)
	DeleteByIdentifier(ctx context.Context, identifier string) (*api.ResourceRequestAccepted, error)
}
