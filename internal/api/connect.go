package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"buf.build/gen/go/ctrlplane/ctrlplane/connectrpc/go/ctrlplane/api/v1/apiv1connect"
	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/version"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

// Client bundles the ctrlplane workspace-engine Connect service clients behind a
// single shared HTTP client and credential interceptor. It replaces the
// generated OpenAPI/REST client; the engine's Connect API is authoritative.
type Client struct {
	Resource   apiv1connect.ResourceServiceClient
	Deployment apiv1connect.DeploymentServiceClient
	System     apiv1connect.SystemServiceClient
	Job        apiv1connect.JobServiceClient
	Workspace  apiv1connect.WorkspaceServiceClient
	Workflow   apiv1connect.WorkflowServiceClient
	Release    apiv1connect.ReleaseServiceClient
}

// authInterceptor injects the same credentials the OpenAPI client used: the
// X-API-Key header plus a ctrlc User-Agent. Applied to every RPC.
func authInterceptor(apiKey string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("X-API-Key", apiKey)
			req.Header().Set("User-Agent", fmt.Sprintf("ctrlc/%s", version.Version))
			return next(ctx, req)
		}
	}
}

// NewConnectClient builds a Client pointed at the engine base URL using the
// existing CLI url/api-key config. Connect mounts services at the root with
// fully-qualified procedure paths, so the base URL is the server with any
// trailing "/api" (used by the retired REST surface) trimmed off.
func NewConnectClient(server string, apiKey string) (*Client, error) {
	server = strings.TrimSuffix(server, "/")
	server = strings.TrimSuffix(server, "/api")
	server = strings.TrimSuffix(server, "/")

	httpClient := http.DefaultClient
	opts := connect.WithInterceptors(authInterceptor(apiKey))

	return &Client{
		Resource:   apiv1connect.NewResourceServiceClient(httpClient, server, opts),
		Deployment: apiv1connect.NewDeploymentServiceClient(httpClient, server, opts),
		System:     apiv1connect.NewSystemServiceClient(httpClient, server, opts),
		Job:        apiv1connect.NewJobServiceClient(httpClient, server, opts),
		Workspace:  apiv1connect.NewWorkspaceServiceClient(httpClient, server, opts),
		Workflow:   apiv1connect.NewWorkflowServiceClient(httpClient, server, opts),
		Release:    apiv1connect.NewReleaseServiceClient(httpClient, server, opts),
	}, nil
}

// GetWorkspaceID resolves a workspace slug or UUID to its UUID. A UUID input is
// returned verbatim; otherwise the slug is resolved via GetWorkspaceBySlug.
func (c *Client) GetWorkspaceID(ctx context.Context, workspace string) (uuid.UUID, error) {
	if id, err := uuid.Parse(workspace); err == nil {
		return id, nil
	}

	resp, err := c.Workspace.GetWorkspaceBySlug(ctx, connect.NewRequest(&apiv1.GetWorkspaceBySlugRequest{
		Slug: workspace,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return uuid.Nil, fmt.Errorf("workspace %q not found", workspace)
		}
		return uuid.Nil, fmt.Errorf("failed to look up workspace %q: %w", workspace, err)
	}

	id, err := uuid.Parse(resp.Msg.GetId())
	if err != nil {
		return uuid.Nil, fmt.Errorf("workspace %q returned invalid id %q: %w", workspace, resp.Msg.GetId(), err)
	}
	return id, nil
}

// NewStruct converts a plain map into a *structpb.Struct for proto Config-style
// fields. A nil or empty map yields nil (matching how the REST client omitted
// absent bodies).
//
// The map is normalised through a JSON round-trip first. This mirrors exactly
// how the REST client serialised config (json.Marshal), so named scalar types
// (e.g. Kubernetes PodPhase), time.Time, and other JSON-marshalable values are
// preserved rather than being rejected by structpb.NewStruct, which only
// accepts the bare types string/bool/float64/[]any/map[string]any.
func NewStruct(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var normalized map[string]any
	if err := json.Unmarshal(b, &normalized); err != nil {
		return nil
	}
	s, err := structpb.NewStruct(normalized)
	if err != nil {
		return nil
	}
	return s
}
