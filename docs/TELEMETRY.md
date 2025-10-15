# OpenTelemetry Integration

The Ctrlplane CLI now includes OpenTelemetry support for observability and distributed tracing.

## Features

- **Root span creation**: Every CLI invocation creates a root span with command details
- **Automatic span management**: Spans are properly started and ended with success/error status
- **Configurable telemetry**: Can be enabled/disabled via environment variables
- **OTLP export**: Traces are exported using the OpenTelemetry Protocol (OTLP)

## Configuration

### Environment Variables

#### Core Configuration

- `TELEMETRY_DISABLED`: Set to `"true"` to disable telemetry completely
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint URL (default: `http://localhost:4317`)
- `OTEL_EXPORTER_OTLP_INSECURE`: Set to `"true"` to use insecure (non-TLS) connection
- `OTEL_SERVICE_NAME`: Override service name (default: `ctrlplane-cli`)
- `OTEL_RESOURCE_ATTRIBUTES`: Additional resource attributes

#### Datadog Configuration

To send traces to Datadog (via Agent or directly):

- `DATADOG_ENABLED`: Set to `"true"` to enable Datadog-specific configuration
- `DD_API_KEY`: Datadog API key (required for direct intake, optional for Agent)
- `DD_OTLP_GRPC_ENDPOINT`: Datadog endpoint (default: `localhost:4317`)
  - For Agent: `localhost:4317`
  - For direct intake: `api.datadoghq.com:4317` (US) or `api.datadoghq.eu:4317` (EU)
- `DD_SERVICE`: Service name for Datadog (overrides `OTEL_SERVICE_NAME`)
- `DD_ENV`: Environment name (e.g., `production`, `staging`, `development`)
- `DD_VERSION`: Service version for Datadog
- `DD_TAGS`: Additional tags in format `key1:value1,key2:value2`

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

### Enable Telemetry with Datadog

#### Direct to Datadog Intake (Agentless)

```bash
# Send traces directly to Datadog without local Agent
export DATADOG_ENABLED=true
export DD_API_KEY=your_datadog_api_key_here
export OTLP_EXPORTER_GRPC_ENDPOINT=api.datadoghq.com:4317  # US region
export OTEL_SERVICE_NAME=ctrlplane-cli
export DD_ENV=production

# Run CLI commands
ctrlc sync aws eks --region us-west-2
```

**Note**: Direct intake requires a Datadog API key and uses TLS automatically.

### Disable Telemetry

```bash
# Disable telemetry completely
TELEMETRY_DISABLED=true ctrlc version
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
- **AWS SDK** ✅ - EKS, EC2, RDS, VPC operations
- **Azure SDK** ✅ - AKS and networking operations
- **Google Cloud APIs** ✅ - GKE, Cloud SQL, Cloud Run, and more
- **Salesforce API** ✅ - SOQL queries for opportunities and accounts
- **Tailscale API** ✅ - Device listing and management
- **Kubernetes Client** ✅ - Namespace and deployment operations
- **Ctrlplane API** ✅ - Resource provider operations with trace context propagation

Each API call creates detailed spans with:
- Service-specific attributes (region, project ID, organization, etc.)
- Operation details (list, describe, query, etc.)
- Success/error states
- Result counts and pagination info

### Distributed Tracing

All API calls to the Ctrlplane backend automatically include trace context via the `traceparent` HTTP header (W3C Trace Context standard). This enables end-to-end distributed tracing from the CLI through to the backend services.

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

This means the OTLP collector/agent is not running. Options:

1. **For Datadog**: Ensure Datadog Agent is running and OTLP is enabled
2. **For Jaeger**: Start a Jaeger instance (see example above)
3. **For other collectors**: Configure endpoint with `OTEL_EXPORTER_OTLP_ENDPOINT`
4. **To disable**: Set `TELEMETRY_DISABLED=true`

### Datadog-Specific Troubleshooting

**Traces not appearing in Datadog:**

1. Verify Datadog Agent is running:
   ```bash
   datadog-agent status
   ```

2. Check OTLP receiver is enabled in `datadog.yaml`:
   ```yaml
   otlp_config:
     receiver:
       protocols:
         grpc:
           endpoint: 0.0.0.0:4317
   ```

3. Verify connectivity:
   ```bash
   telnet localhost 4317
   ```

4. Check Agent logs for OTLP errors:
   ```bash
   tail -f /var/log/datadog/agent.log | grep -i otlp
   ```

**Service not showing correct name in Datadog:**

Ensure you've set `DD_SERVICE` or it will default to `ctrlplane-cli`:
```bash
export DD_SERVICE=my-custom-service-name
```

**Authentication errors when sending directly to Datadog intake:**

Ensure `DD_API_KEY` is set when using direct intake (not needed for Agent):
```bash
export DD_API_KEY=your_datadog_api_key
export DD_OTLP_GRPC_ENDPOINT=api.datadoghq.com:4317
```

**Connection timeouts or TLS errors with direct intake:**

Verify your API key is valid and you're using the correct regional endpoint:
- US: `api.datadoghq.com:4317`
- EU: `api.datadoghq.eu:4317`
- US1-FED: `api.ddog-gov.com:4317`

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