package apply

import (
	"context"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
)

// getSlug returns the provided slug if set, otherwise generates one from the name
// func getSlug(providedSlug *string, name string) string {
// 	if providedSlug != nil && *providedSlug != "" {
// 		return *providedSlug
// 	}
// 	return slug.Make(name)
// }

// Resolver helps resolve document references (e.g., system names) to IDs
type Resolver struct {
	client      *api.ClientWithResponses
	workspaceID string
	// Cache for resolved systems by name
	systemCache map[string]string
}

// NewResolver creates a new resolver for document references
func NewResolver(client *api.ClientWithResponses, workspaceID string) *Resolver {
	return &Resolver{
		client:      client,
		workspaceID: workspaceID,
		systemCache: make(map[string]string),
	}
}

// ResolveSystemID resolves a system name or ID to an actual ID
func (r *Resolver) ResolveSystemID(ctx context.Context, nameOrID string) (string, error) {
	// Check if it's already a valid UUID
	if _, err := uuid.Parse(nameOrID); err == nil {
		return nameOrID, nil
	}

	// Check cache
	if id, ok := r.systemCache[nameOrID]; ok {
		return id, nil
	}

	// Look up by name
	resp, err := r.client.ListSystemsWithResponse(ctx, r.workspaceID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list systems: %w", err)
	}

	if resp.JSON200 == nil {
		return "", fmt.Errorf("failed to list systems: %s", string(resp.Body))
	}

	for _, sys := range resp.JSON200.Items {
		r.systemCache[sys.Name] = sys.Id
		if sys.Name == nameOrID || sys.Slug == nameOrID {
			return sys.Id, nil
		}
	}

	return "", fmt.Errorf("system not found: %s", nameOrID)
}

func fromCelSelector(cel string) *api.Selector {
	var selector api.Selector
	selector.FromCelSelector(api.CelSelector{Cel: cel})
	return &selector
}

// buildSelector converts a SelectorConfig to an API Selector
func buildSelector(cfg *SelectorConfig) (*api.Selector, error) {
	if cfg == nil {
		return nil, nil
	}

	var selector api.Selector

	if cfg.Cel != nil {
		if err := selector.FromCelSelector(api.CelSelector{Cel: *cfg.Cel}); err != nil {
			return nil, fmt.Errorf("failed to create CEL selector: %w", err)
		}
		return &selector, nil
	}

	if cfg.Json != nil {
		if err := selector.FromJsonSelector(api.JsonSelector{Json: cfg.Json}); err != nil {
			return nil, fmt.Errorf("failed to create JSON selector: %w", err)
		}
		return &selector, nil
	}

	return nil, nil
}
