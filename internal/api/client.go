//go:generate go tool oapi-codegen -config ../../openapi.client.yaml https://raw.githubusercontent.com/ctrlplanedev/ctrlplane/refs/heads/main/apps/api/openapi/openapi.json
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func NewAPIKeyClientWithResponses(server string, apiKey string) (*ClientWithResponses, error) {
	server = strings.TrimSuffix(server, "/")
	server = strings.TrimSuffix(server, "/api")
	return NewClientWithResponses(server+"/api",
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-API-Key", apiKey)
			return nil
		}),
	)
}

func (c *ClientWithResponses) GetWorkspaceID(ctx context.Context, workspace string) uuid.UUID {
	id, err := uuid.Parse(workspace)
	if err == nil {
		return id
	}

	resp, err := c.GetWorkspaceBySlugWithResponse(ctx, workspace)
	if err != nil {
		return uuid.Nil
	}

	if resp.JSON200 == nil {
		return uuid.Nil
	}

	return resp.JSON200.Id
}
