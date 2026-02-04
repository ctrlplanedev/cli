package applyv2

import (
	"context"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/api/resolver"
)

type ProviderContext struct {
	ctx         context.Context
	workspaceID string
	client      *api.ClientWithResponses
	resolver    *resolver.APIResolver
}

func NewProviderContext(ctx context.Context, workspaceID string, client *api.ClientWithResponses, resolver *resolver.APIResolver) *ProviderContext {
	return &ProviderContext{
		ctx:         ctx,
		workspaceID: workspaceID,
		client:      client,
		resolver:    resolver,
	}
}

func (c *ProviderContext) Ctx() context.Context {
	return c.ctx
}

func (c *ProviderContext) WorkspaceIDValue() string {
	return c.workspaceID
}

func (c *ProviderContext) APIClient() *api.ClientWithResponses {
	return c.client
}

func (c *ProviderContext) ResolverProvider() *resolver.APIResolver {
	return c.resolver
}
