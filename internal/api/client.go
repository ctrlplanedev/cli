package api

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func NewAPIKeyClientWithResponses(server string, apiKey string) (*ClientWithResponses, error) {
	server = strings.TrimSuffix(server, "/")
	server = strings.TrimSuffix(server, "/api")
	return NewClientWithResponses(server+"/api",
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			// Set API key
			req.Header.Set("X-API-Key", apiKey)

			// Inject trace context into HTTP headers (traceparent, tracestate)
			// This propagates the current span context to the downstream service
			propagator := otel.GetTextMapPropagator()
			propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

			return nil
		}),
	)
}
