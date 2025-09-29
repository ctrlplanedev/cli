# API Telemetry Implementation Guide

This guide shows how to add OpenTelemetry tracing to each external API integration in the Ctrlplane CLI.

## Summary of Current Implementation

✅ **Implemented**:
1. **Terraform Cloud API** - Complete tracing with retry logic and variable fetching
2. **GitHub API** - Complete tracing for pull requests and commits with pagination
3. **AWS SDK (EKS)** - Complete tracing for cluster listing and describing

🟡 **To be implemented** (patterns provided below):
4. Azure SDK
5. Google Cloud APIs
6. Salesforce API
7. Tailscale API
8. Kubernetes Client

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

## Specific Implementation Guides

### 4. Azure SDK Integration

**Files to modify:**
- `cmd/ctrlc/root/sync/azure/aks/aks.go`
- `cmd/ctrlc/root/sync/azure/networks/networks.go`

**Pattern:**
```go
// Add telemetry to Azure ARM calls
ctx, span := telemetry.StartAPISpan(ctx, "azure.aks", "list_clusters",
    attribute.String("azure.subscription_id", subscriptionID),
    attribute.String("azure.resource_group", resourceGroup),
)
defer span.End()

result, err := aksClient.List(ctx, resourceGroup)
if err != nil {
    telemetry.SetSpanError(span, err)
    return err
}

telemetry.AddSpanAttribute(span, "azure.clusters_found", len(result.Value))
telemetry.SetSpanSuccess(span)
```

### 5. Google Cloud API Integration

**Files to modify:**
- `cmd/ctrlc/root/sync/google/gke/gke.go`
- `cmd/ctrlc/root/sync/google/cloudsql/cloudsql.go`
- `cmd/ctrlc/root/sync/google/cloudrun/cloudrun.go`

**Pattern:**
```go
ctx, span := telemetry.StartAPISpan(ctx, "gcp.gke", "list_clusters",
    attribute.String("gcp.project_id", projectID),
    attribute.String("gcp.location", location),
)
defer span.End()

clusters, err := gkeClient.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
if err != nil {
    telemetry.SetSpanError(span, err)
    return err
}

telemetry.AddSpanAttribute(span, "gcp.clusters_found", len(clusters.Clusters))
telemetry.SetSpanSuccess(span)
```

### 6. Salesforce API Integration

**Files to modify:**
- `cmd/ctrlc/root/sync/salesforce/opportunities/opportunities.go`
- `cmd/ctrlc/root/sync/salesforce/accounts/accounts.go`

**Pattern:**
```go
ctx, span := telemetry.StartAPISpan(ctx, "salesforce", "soql_query",
    attribute.String("salesforce.object_type", "Opportunity"),
    attribute.String("salesforce.query", query),
)
defer span.End()

records, err := sf.Query(query).Context(ctx).Do()
if err != nil {
    telemetry.SetSpanError(span, err)
    return err
}

telemetry.AddSpanAttribute(span, "salesforce.records_found", len(records.Records))
telemetry.SetSpanSuccess(span)
```

### 7. Tailscale API Integration

**Files to modify:**
- `cmd/ctrlc/root/sync/tailscale/tailscale.go`

**Pattern:**
```go
ctx, span := telemetry.StartAPISpan(ctx, "tailscale", "list_devices",
    attribute.String("tailscale.tailnet", tailnet),
)
defer span.End()

devices, err := client.Devices(ctx, tailnet)
if err != nil {
    telemetry.SetSpanError(span, err)
    return err
}

telemetry.AddSpanAttribute(span, "tailscale.devices_found", len(devices))
telemetry.SetSpanSuccess(span)
```

### 8. Kubernetes Client Integration

**Files to modify:**
- `cmd/ctrlc/root/sync/kubernetes/kubernetes.go`
- `cmd/ctrlc/root/sync/kubernetes/vcluster.go`

**Pattern:**
```go
ctx, span := telemetry.StartAPISpan(ctx, "kubernetes", "list_pods",
    attribute.String("k8s.namespace", namespace),
    attribute.String("k8s.cluster", clusterName),
)
defer span.End()

pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
if err != nil {
    telemetry.SetSpanError(span, err)
    return err
}

telemetry.AddSpanAttribute(span, "k8s.pods_found", len(pods.Items))
telemetry.SetSpanSuccess(span)
```

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

- [ ] Add telemetry imports
- [ ] Wrap main operation function with root span
- [ ] Add spans for individual API calls
- [ ] Include relevant attributes (service, operation, identifiers)
- [ ] Handle pagination with child spans
- [ ] Set success/error states
- [ ] Add count attributes for results
- [ ] Test with `go build` to ensure no compilation errors

## Testing

After implementing telemetry for each API:

1. **Build test:**
   ```bash
   go build ./cmd/ctrlc
   ```

2. **Disabled telemetry test:**
   ```bash
   CTRLPLANE_TELEMETRY_DISABLED=true ./ctrlc sync [service] [command] --help
   ```

3. **Enabled telemetry test:**
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 ./ctrlc sync [service] [command]
   ```

## Benefits

With this implementation, every external API call will be traced with:

- **Visibility**: See exactly which APIs are being called and how long they take
- **Error tracking**: Automatic error recording with context
- **Performance insights**: Understand API call patterns and bottlenecks
- **Debugging**: Detailed trace information for troubleshooting
- **Monitoring**: Integration with observability platforms like Jaeger, DataDog, etc.

Each API call creates a new trace that can be correlated back to the root CLI command invocation, providing end-to-end visibility into the entire sync operation.