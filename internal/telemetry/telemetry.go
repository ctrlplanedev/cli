package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "ctrlplane-cli"
	serviceVersion = "1.0.0" // This could be made configurable
)

var tracer trace.Tracer

// InitTelemetry initializes OpenTelemetry with OTLP exporter
func InitTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// Check if telemetry is enabled
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("CTRLPLANE_TELEMETRY_DISABLED") == "true" {
		// Return no-op shutdown function if telemetry is disabled
		return func(context.Context) error { return nil }, nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
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
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP trace exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(), // Use secure connections in production
	)
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

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer for this package
	tracer = otel.Tracer(serviceName)

	// Return shutdown function
	return tp.Shutdown, nil
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
