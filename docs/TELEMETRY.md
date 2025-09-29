# OpenTelemetry Integration

The Ctrlplane CLI now includes OpenTelemetry support for observability and distributed tracing.

## Features

- **Root span creation**: Every CLI invocation creates a root span with command details
- **Automatic span management**: Spans are properly started and ended with success/error status
- **Configurable telemetry**: Can be enabled/disabled via environment variables
- **OTLP export**: Traces are exported using the OpenTelemetry Protocol (OTLP)

## Configuration

### Environment Variables

- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint URL (default: `http://localhost:4317`)
- `CTRLPLANE_TELEMETRY_DISABLED`: Set to `"true"` to disable telemetry completely
- `OTEL_SERVICE_NAME`: Override service name (default: `ctrlplane-cli`)
- `OTEL_RESOURCE_ATTRIBUTES`: Additional resource attributes

### Standard OpenTelemetry Variables

All standard OpenTelemetry environment variables are supported:

- `OTEL_EXPORTER_OTLP_HEADERS`: Custom headers for OTLP exporter
- `OTEL_EXPORTER_OTLP_TIMEOUT`: Timeout for OTLP export
- `OTEL_TRACES_SAMPLER`: Sampling strategy
- `OTEL_TRACES_SAMPLER_ARG`: Sampler arguments

## Usage Examples

### Enable Telemetry with Jaeger

```bash
# Start Jaeger (example using Docker)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 14250:14250 \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Run CLI with telemetry
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 ctrlc version
```

### Enable Telemetry with Custom OTLP Collector

```bash
# Configure OTLP endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT=https://your-collector-endpoint:4317
export OTEL_EXPORTER_OTLP_HEADERS="api-key=your-api-key"

# Run CLI commands
ctrlc sync github pull-requests --repo owner/repo
```

### Disable Telemetry

```bash
# Disable telemetry completely
CTRLPLANE_TELEMETRY_DISABLED=true ctrlc version
```

## Span Attributes

Each root span includes the following attributes:

- `cli.command`: The command being executed (e.g., "sync github pull-requests")
- `cli.args`: Command arguments as a string array
- `cli.version`: CLI version
- `success`: Boolean indicating if the command succeeded
- `error`: Boolean indicating if an error occurred

Additional resource attributes are automatically added:

- `service.name`: Service name (ctrlplane-cli)
- `service.version`: Service version
- `process.pid`: Process ID
- `host.*`: Host information
- `os.*`: Operating system information

## External API Tracing

The CLI automatically traces all external API calls to:

- **Terraform Cloud API** ✅ - Workspace listing, variable fetching
- **GitHub API** ✅ - Pull request sync, commit fetching
- **AWS SDK** ✅ - EKS cluster operations
- **Azure SDK** - AKS and networking operations
- **Google Cloud APIs** - GKE, Cloud SQL, Cloud Run operations
- **Salesforce API** - SOQL queries for opportunities and accounts
- **Tailscale API** - Device listing and management
- **Kubernetes Client** - Pod and resource operations

Each API call creates detailed spans with:
- Service-specific attributes (region, project ID, organization, etc.)
- Operation details (list, describe, query, etc.)
- Success/error states
- Result counts and pagination info

See [API_TELEMETRY_GUIDE.md](./API_TELEMETRY_GUIDE.md) for implementation details.

## Integration in Subcommands

Subcommands can access the telemetry context and create child spans:

```go
import (
    "github.com/ctrlplanedev/cli/internal/telemetry"
)

func myCommandHandler(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    // Create a child span
    ctx, span := telemetry.StartSpan(ctx, "my-operation")
    defer span.End()

    // Add custom attributes
    telemetry.AddSpanAttribute(span, "custom.attribute", "value")

    // Your command logic here
    if err := doSomething(ctx); err != nil {
        telemetry.SetSpanError(span, err)
        return err
    }

    telemetry.SetSpanSuccess(span)
    return nil
}
```

### API Call Tracing Helpers

For external API integrations, use these helper functions:

```go
// Simple API call with automatic error handling
result, err := telemetry.WithTelemetry(ctx, "service.operation",
    func(ctx context.Context) (Result, error) {
        return apiClient.CallAPI(ctx, params)
    },
    attribute.String("service.param", value),
)

// Manual span creation for complex operations
ctx, span := telemetry.StartAPISpan(ctx, "service", "operation",
    attribute.String("service.resource_id", resourceID),
)
defer span.End()
// ... API call logic
```

## Troubleshooting

### Connection Issues

If you see connection refused errors like:

```
grpc: addrConn.createTransport failed to connect to {...}: connection refused
```

This means the OTLP collector is not running. Either:

1. Start an OTLP-compatible collector (like Jaeger)
2. Configure a different endpoint with `OTEL_EXPORTER_OTLP_ENDPOINT`
3. Disable telemetry with `CTRLPLANE_TELEMETRY_DISABLED=true`

### Debugging

Enable debug logging to see telemetry-related messages:

```bash
ctrlc --log-level debug version
```

## Performance

- Telemetry initialization happens early in the CLI lifecycle
- Spans are batched and exported asynchronously
- Failed telemetry initialization does not prevent CLI operation
- Graceful shutdown ensures pending spans are exported