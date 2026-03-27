//go:generate go tool oapi-codegen -config openapi.client.yaml https://raw.githubusercontent.com/ctrlplanedev/ctrlplane/refs/heads/main/apps/api/openapi/openapi.json
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// UpsertResourceByIdentifierJSONBody is the request body for upserting a resource directly by identifier.
type UpsertResourceByIdentifierJSONBody struct {
	Config   map[string]interface{} `json:"config,omitempty"`
	Kind     string                 `json:"kind"`
	Metadata map[string]string      `json:"metadata,omitempty"`
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
}

// UpsertResourceByIdentifierResponse is the response from upserting a resource by identifier.
type UpsertResourceByIdentifierResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON202      *ResourceRequestAccepted
}

func (r UpsertResourceByIdentifierResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// UpsertResourceByIdentifierWithResponse calls PATCH /v1/workspaces/{workspaceId}/resources/identifier/{identifier}
// to upsert a resource directly without requiring a resource provider.
func (c *ClientWithResponses) UpsertResourceByIdentifierWithResponse(ctx context.Context, workspaceId string, identifier string, body UpsertResourceByIdentifierJSONBody) (*UpsertResourceByIdentifierResponse, error) {
	baseClient, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("unexpected client type")
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/v1/workspaces/%s/resources/identifier/%s", baseClient.Server, workspaceId, identifier)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := baseClient.applyEditors(ctx, req, nil); err != nil {
		return nil, fmt.Errorf("failed to apply request editors: %w", err)
	}

	httpResp, err := baseClient.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	defer func() { _ = httpResp.Body.Close() }()
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := &UpsertResourceByIdentifierResponse{
		Body:         respBody,
		HTTPResponse: httpResp,
	}

	if strings.Contains(httpResp.Header.Get("Content-Type"), "json") && httpResp.StatusCode == 202 {
		var dest ResourceRequestAccepted
		if err := json.Unmarshal(respBody, &dest); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		response.JSON202 = &dest
	}

	return response, nil
}

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
