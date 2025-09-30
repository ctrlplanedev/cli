# API Telemetry Implementation Guide

This guide shows how to add OpenTelemetry tracing to each external API integration in the Ctrlplane CLI.

## Summary of Current Implementation

✅ **Fully Implemented**:
1. **Terraform Cloud API** - Complete tracing with retry logic and variable fetching
2. **GitHub API** - Complete tracing for pull requests and commits with pagination
3. **AWS SDK** - Complete tracing for EKS, EC2, RDS, and VPC operations
4. **Azure SDK** - Complete tracing for AKS clusters and virtual networks
5. **Google Cloud APIs** - Complete tracing for GKE, Cloud SQL, Cloud Run, Storage, Redis, BigTable, VMs, Secrets, Projects, and Networks
6. **Salesforce API** - Complete tracing for SOQL queries and pagination
7. **Tailscale API** - Complete tracing for device listing and management
8. **Kubernetes Client** - Complete tracing for namespace and deployment operations
9. **Ctrlplane API** - Automatic trace context propagation via `traceparent` header

## Datadog Integration

Two ways to send traces to Datadog:

### Option 1: Via Datadog Agent (Recommended)

```bash
# Enable Datadog integration with local Agent
export DATADOG_ENABLED=true
export DD_SERVICE=ctrlplane-cli
export DD_ENV=production
export DD_VERSION=1.0.0
export DD_TAGS="team:platform,component:cli"

# Run any CLI command
ctrlc sync aws eks --region us-west-2
```

**Requirements:**
- Datadog Agent must be running with OTLP enabled
- Agent configuration should include:
  ```yaml
  otlp_config:
    receiver:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
  ```

### Option 2: Direct to Datadog Intake (Agentless)

```bash
# Send traces directly to Datadog without Agent
export DATADOG_ENABLED=true
export DD_API_KEY=your_datadog_api_key
export DD_OTLP_GRPC_ENDPOINT=api.datadoghq.com:4317  # US
# export DD_OTLP_GRPC_ENDPOINT=api.datadoghq.eu:4317  # EU
export DD_SERVICE=ctrlplane-cli
export DD_ENV=production

# Run any CLI command
ctrlc sync aws eks --region us-west-2
```

The CLI will automatically:
- Connect to the specified Datadog endpoint
- Include Datadog API key header for authentication (when provided)
- Apply unified service tags (env, service, version)
- Parse and apply custom tags from `DD_TAGS`
- Use W3C Trace Context for distributed tracing
- Enable TLS automatically for remote endpoints

## Ctrlplane API Trace Propagation

All API calls to the Ctrlplane backend automatically propagate trace context via the `traceparent` HTTP header (W3C Trace Context standard). This is handled transparently by the API client in `internal/api/client.go`.

When you make API calls like:
```go
ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
rp.UpsertResource(ctx, resources)
```

The client automatically:
1. Extracts the current span context from `ctx`
2. Injects it into the HTTP request headers as `traceparent`
3. Enables end-to-end distributed tracing from CLI → Ctrlplane API

This means every API call is automatically part of the same distributed trace as the CLI command that initiated it.

## Implementation Patterns

### 1. Basic API Call Tracing

```go
// Add to imports
import (
    "github.com/ctrlplanedev/cli/internal/telemetry"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// Wrap API calls with telemetry
func myAPICall(ctx context.Context, client APIClient, param string) (Result, error) {
    ctx, span := telemetry.StartAPISpan(ctx, "service_name", "operation_name",
        attribute.String("service.param", param),
    )
    defer span.End()

    result, err := client.CallAPI(ctx, param)
    if err != nil {
        telemetry.SetSpanError(span, err)
        return nil, err
    }

    telemetry.AddSpanAttribute(span, "service.results_count", len(result))
    telemetry.SetSpanSuccess(span)
    return result, nil
}
```

### 2. Paginated API Calls

```go
func fetchWithPagination(ctx context.Context, client APIClient) ([]Item, error) {
    ctx, span := telemetry.StartSpan(ctx, "service.fetch_paginated",
        trace.WithSpanKind(trace.SpanKindClient),
    )
    defer span.End()

    var allItems []Item
    page := 1
    totalApiCalls := 0

    for {
        // Child span for each page
        pageCtx, pageSpan := telemetry.StartSpan(ctx, "service.fetch_page",
            trace.WithSpanKind(trace.SpanKindClient),
            trace.WithAttributes(
                attribute.Int("service.page", page),
            ),
        )

        items, hasNext, err := client.FetchPage(pageCtx, page)
        totalApiCalls++

        if err != nil {
            telemetry.SetSpanError(pageSpan, err)
            pageSpan.End()
            telemetry.SetSpanError(span, err)
            return nil, err
        }

        telemetry.AddSpanAttribute(pageSpan, "service.items_fetched", len(items))
        telemetry.SetSpanSuccess(pageSpan)
        pageSpan.End()

        allItems = append(allItems, items...)
        if !hasNext {
            break
        }
        page++
    }

    telemetry.AddSpanAttribute(span, "service.total_items", len(allItems))
    telemetry.AddSpanAttribute(span, "service.total_api_calls", totalApiCalls)
    telemetry.SetSpanSuccess(span)

    return allItems, nil
}
```

### 3. Using WithTelemetry Helper

```go
func simpleAPICall(ctx context.Context, client APIClient, param string) (Result, error) {
    return telemetry.WithTelemetry(ctx, "service.simple_call",
        func(ctx context.Context) (Result, error) {
            return client.SimpleCall(ctx, param)
        },
        attribute.String("service.param", param),
    )
}
```

## Reference Implementation Examples

All external API integrations now have complete telemetry tracing. Here are the key files to reference:

### Azure SDK
- **AKS**: `cmd/ctrlc/root/sync/azure/aks/aks.go` (lines 163-243)
- **Networks**: `cmd/ctrlc/root/sync/azure/networks/networks.go` (lines 160-280)

### Google Cloud APIs
- **GKE**: `cmd/ctrlc/root/sync/google/gke/gke.go`
- **Cloud SQL**: `cmd/ctrlc/root/sync/google/cloudsql/cloudsql.go`
- **Cloud Run**: `cmd/ctrlc/root/sync/google/cloudrun/cloudrun.go`
- **Storage**: `cmd/ctrlc/root/sync/google/buckets/buckets.go`
- **And 6+ more Google Cloud services**

### Salesforce API
- **Accounts**: `cmd/ctrlc/root/sync/salesforce/accounts/accounts.go`
- **Opportunities**: `cmd/ctrlc/root/sync/salesforce/opportunities/opportunities.go`
- **Common**: `cmd/ctrlc/root/sync/salesforce/common/util.go` (comprehensive pagination tracing)

### Tailscale API
- **Devices**: `cmd/ctrlc/root/sync/tailscale/tailscale.go` (lines 96-198)

### Kubernetes Client
- **Resources**: `cmd/ctrlc/root/sync/kubernetes/kubernetes.go` (lines 120-161)

## Standard Attributes

### Common Attributes for All APIs
- `service.operation` - The operation being performed
- `service.success` - Boolean indicating success
- `error` - Boolean indicating if an error occurred

### Service-Specific Attributes

**Terraform:**
- `terraform.organization`
- `terraform.workspace_id`
- `terraform.page_number`

**GitHub:**
- `github.owner`
- `github.repo`
- `github.pr_number`
- `github.state`

**AWS:**
- `aws.region`
- `aws.service` (eks, rds, ec2)
- `aws.resource_id`

**Azure:**
- `azure.subscription_id`
- `azure.resource_group`
- `azure.location`

**GCP:**
- `gcp.project_id`
- `gcp.location`
- `gcp.zone`

**Salesforce:**
- `salesforce.object_type`
- `salesforce.query`
- `salesforce.domain`

**Tailscale:**
- `tailscale.tailnet`
- `tailscale.device_id`

**Kubernetes:**
- `k8s.cluster`
- `k8s.namespace`
- `k8s.resource_type`

## Implementation Checklist

For each API integration:

- [x] Add telemetry imports
- [x] Wrap main operation function with root span
- [x] Add spans for individual API calls
- [x] Include relevant attributes (service, operation, identifiers)
- [x] Handle pagination with child spans
- [x] Set success/error states
- [x] Add count attributes for results
- [x] Test with `go build` to ensure no compilation errors
- [x] Add trace context propagation to Ctrlplane API calls

## Testing

After implementing telemetry for each API:

1. **Build test:**
   ```bash
   go build ./cmd/ctrlc
   ```

2. **Disabled telemetry test:**
   ```bash
   TELEMETRY_DISABLED=true ./ctrlc sync [service] [command] --help
   ```

3. **Enabled telemetry test:**
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 ./ctrlc sync [service] [command]
   ```

## Benefits

With this implementation, every external API call is traced with:

- **Visibility**: See exactly which APIs are being called and how long they take
- **Error tracking**: Automatic error recording with context
- **Performance insights**: Understand API call patterns and bottlenecks
- **Debugging**: Detailed trace information for troubleshooting
- **Monitoring**: Integration with observability platforms like Jaeger, Datadog, etc.
- **Distributed tracing**: Full end-to-end traces from CLI → External APIs → Ctrlplane API

Each API call creates a span that's part of the root trace from the CLI command invocation, providing complete visibility into the entire operation including downstream calls to the Ctrlplane backend.

## Datadog-Specific Features

When using Datadog:

- **Unified Service Tagging**: Automatic inclusion of `env`, `service`, and `version` tags
- **Custom Tags**: Support for `DD_TAGS` environment variable
- **APM Integration**: Traces appear in Datadog APM with service map
- **Resource Names**: Automatic resource naming based on operation
- **Infrastructure Correlation**: Links traces to host metrics via Datadog Agent

Example trace in Datadog APM:
```
ctrlc sync aws eks
├─ aws.eks.process_clusters (200ms)
│  ├─ aws.eks.list_clusters (150ms)
│  └─ aws.eks.describe_cluster (50ms)
└─ POST /api/v1/workspaces/{id}/resource-providers/{id}/resources (300ms)
   └─ [Ctrlplane backend spans continue the trace...]
```