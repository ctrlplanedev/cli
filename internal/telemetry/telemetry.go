package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

const (
	serviceName    = "ctrlplane-cli"
	serviceVersion = "1.0.0" // This could be made configurable
)

var tracer trace.Tracer

// InitTelemetry initializes OpenTelemetry with OTLP exporter
func InitTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// Check if telemetry is disabled
	if os.Getenv("TELEMETRY_DISABLED") == "true" {
		// Return no-op shutdown function if telemetry is disabled
		return func(context.Context) error { return nil }, nil
	}

	// Check if any telemetry endpoint is configured
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		// Return no-op shutdown function if no endpoint is configured
		return func(context.Context) error { return nil }, nil
	}

	// Create resource with service information
	res, err := createResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP trace exporter with appropriate configuration
	exporter, err := createOTLPExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Configure sampling as needed
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator (includes Datadog propagation if enabled)
	otel.SetTextMapPropagator(createPropagator())

	// Get tracer for this package
	tracer = otel.Tracer(serviceName)

	// Return shutdown function
	return tp.Shutdown, nil
}

// createResource creates an OpenTelemetry resource with service information
func createResource(ctx context.Context) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
	}

	// Add Datadog-specific attributes if enabled
	if os.Getenv("DATADOG_ENABLED") == "true" {
		if env := os.Getenv("DD_ENV"); env != "" {
			attrs = append(attrs, attribute.String("deployment.environment", env))
		}
		if version := os.Getenv("DD_VERSION"); version != "" {
			attrs[1] = semconv.ServiceVersionKey.String(version)
		}
		if tags := os.Getenv("DD_TAGS"); tags != "" {
			// Parse DD_TAGS format: key1:value1,key2:value2
			for _, tag := range strings.Split(tags, ",") {
				parts := strings.SplitN(strings.TrimSpace(tag), ":", 2)
				if len(parts) == 2 {
					attrs = append(attrs, attribute.String(parts[0], parts[1]))
				}
			}
		}
	}

	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessOwner(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithHost(),
		resource.WithOS(),
	)
}

// createOTLPExporter creates an OTLP exporter with Datadog or generic configuration
func createOTLPExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	var opts []otlptracegrpc.Option

	// Check if Datadog is explicitly enabled
	if os.Getenv("DATADOG_ENABLED") == "true" {
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = "localhost:4317"
		}
		opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))

		// Add Datadog API key as header if provided
		// This is required when sending directly to Datadog intake (not via Agent)
		if apiKey := os.Getenv("DD_API_KEY"); apiKey != "" {
			opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{
				"dd-api-key": apiKey,
			}))
		}

		// Datadog Agent typically doesn't require TLS for localhost
		// But Datadog intake endpoints always require TLS
		if strings.HasPrefix(endpoint, "localhost") || strings.HasPrefix(endpoint, "127.0.0.1") {
			opts = append(opts, otlptracegrpc.WithInsecure())
		} else {
			// Use TLS for remote endpoints (Datadog intake or remote Agent)
			opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
		}
	} else {
		// Standard OTLP configuration
		// Check if insecure connection is requested
		if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true" {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	}

	return otlptracegrpc.New(ctx, opts...)
}

// createPropagator creates a composite propagator
func createPropagator() propagation.TextMapPropagator {
	propagators := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
	}

	// Datadog uses its own propagation format in addition to W3C
	// The Datadog Agent will handle conversion from W3C TraceContext to Datadog format
	// So we don't need a special propagator here - W3C TraceContext is sufficient

	return propagation.NewCompositeTextMapPropagator(propagators...)
}

// StartRootSpan creates a root span for the CLI invocation
func StartRootSpan(ctx context.Context, commandName string, args []string) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = otel.Tracer(serviceName)
	}

	spanName := fmt.Sprintf("ctrlc %s", commandName)

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("cli.command", commandName),
			attribute.StringSlice("cli.args", args),
			attribute.String("cli.version", serviceVersion),
		),
	)

	return ctx, span
}

// SetSpanError sets an error on the current span
func SetSpanError(span trace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}
}

// SetSpanSuccess marks the span as successful
func SetSpanSuccess(span trace.Span) {
	if span != nil {
		span.SetAttributes(attribute.Bool("success", true))
	}
}

// AddSpanAttribute adds an attribute to the current span
func AddSpanAttribute(span trace.Span, key string, value interface{}) {
	if span == nil {
		return
	}

	switch v := value.(type) {
	case string:
		span.SetAttributes(attribute.String(key, v))
	case int:
		span.SetAttributes(attribute.Int(key, v))
	case int64:
		span.SetAttributes(attribute.Int64(key, v))
	case bool:
		span.SetAttributes(attribute.Bool(key, v))
	case float64:
		span.SetAttributes(attribute.Float64(key, v))
	case []string:
		span.SetAttributes(attribute.StringSlice(key, v))
	default:
		span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

// GetTracer returns the global tracer instance
func GetTracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer(serviceName)
	}
	return tracer
}

// StartSpan is a convenience function to start a new span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = otel.Tracer(serviceName)
	}
	return tracer.Start(ctx, name, opts...)
}

// StartAPISpan creates a span for an external API call with common attributes
func StartAPISpan(ctx context.Context, service, operation string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("%s.%s", service, operation)
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attributes...),
	}
	return StartSpan(ctx, spanName, opts...)
}

// WithTelemetry wraps a function with telemetry, automatically handling success/error states
func WithTelemetry[T any](ctx context.Context, spanName string, fn func(context.Context) (T, error), attributes ...attribute.KeyValue) (T, error) {
	ctx, span := StartSpan(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attributes...),
	)
	defer span.End()

	result, err := fn(ctx)
	if err != nil {
		SetSpanError(span, err)
	} else {
		SetSpanSuccess(span)
	}

	return result, err
}
