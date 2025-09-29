package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/telemetry"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func GetSalesforceSubdomain(domain string) string {
	subdomain := "salesforce"
	if strings.HasPrefix(domain, "https://") || strings.HasPrefix(domain, "http://") {
		parts := strings.Split(domain, "//")
		if len(parts) > 1 {
			hostParts := strings.Split(parts[1], ".")
			if len(hostParts) > 0 {
				subdomain = hostParts[0]
			}
		}
	}
	return subdomain
}

// QuerySalesforceObject performs a generic query on any Salesforce object with pagination support
func QuerySalesforceObject(ctx context.Context, sf *salesforce.Salesforce, objectName string, limit int, listAllFields bool, target interface{}, additionalFields []string, whereClause string) error {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.query_object",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("salesforce.object", objectName),
			attribute.Int("salesforce.limit", limit),
			attribute.Bool("salesforce.list_all_fields", listAllFields),
		),
	)
	defer span.End()

	if whereClause != "" {
		telemetry.AddSpanAttribute(span, "salesforce.where_clause", whereClause)
	}

	targetValue := reflect.ValueOf(target).Elem()
	if targetValue.Kind() != reflect.Slice {
		err := fmt.Errorf("target must be a pointer to a slice")
		telemetry.SetSpanError(span, err)
		return err
	}

	fieldMap := make(map[string]bool)

	var standardFields []string
	switch objectName {
	case "Account":
		standardFields = []string{
			"Id", "Name", "Type", "Industry", "Website", "Phone",
			"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry",
			"BillingLatitude", "BillingLongitude",
			"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry",
			"ShippingLatitude", "ShippingLongitude",
			"NumberOfEmployees", "Description", "OwnerId", "ParentId", "AccountSource",
			"CreatedDate", "LastModifiedDate", "IsDeleted", "PhotoUrl",
		}
	case "Opportunity":
		standardFields = []string{
			"Id", "Name", "Amount", "StageName", "CloseDate", "AccountId",
			"Probability", "Type", "NextStep", "LeadSource", "IsClosed", "IsWon",
			"ForecastCategory", "Description", "OwnerId", "ContactId", "CampaignId",
			"HasOpenActivity", "CreatedDate", "LastModifiedDate", "LastActivityDate",
		}
	default:
		standardFields = []string{"Id", "Name", "CreatedDate", "LastModifiedDate"}
	}

	for _, field := range standardFields {
		fieldMap[field] = true
	}

	for _, field := range additionalFields {
		fieldMap[field] = true
	}

	fieldNames := make([]string, 0, len(fieldMap))
	for field := range fieldMap {
		fieldNames = append(fieldNames, field)
	}

	telemetry.AddSpanAttribute(span, "salesforce.field_count", len(fieldNames))

	if listAllFields {
		if err := logAvailableFields(ctx, sf, objectName); err != nil {
			telemetry.SetSpanError(span, err)
			return err
		}
	}

	err := paginateQuery(ctx, sf, objectName, fieldNames, whereClause, limit, targetValue)
	if err != nil {
		telemetry.SetSpanError(span, err)
		return err
	}

	telemetry.AddSpanAttribute(span, "salesforce.records_retrieved", targetValue.Len())
	telemetry.SetSpanSuccess(span)
	return nil
}

func logAvailableFields(ctx context.Context, sf *salesforce.Salesforce, objectName string) error {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.describe_object",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("salesforce.object", objectName),
		),
	)
	defer span.End()

	resp, err := sf.DoRequest("GET", fmt.Sprintf("/sobjects/%s/describe", objectName), nil)
	if err != nil {
		telemetry.SetSpanError(span, err)
		return fmt.Errorf("failed to describe %s object: %w", objectName, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		telemetry.SetSpanError(span, err)
		return fmt.Errorf("failed to decode describe response: %w", err)
	}

	fields, ok := result["fields"].([]interface{})
	if !ok {
		err := fmt.Errorf("unexpected describe response format")
		telemetry.SetSpanError(span, err)
		return err
	}

	fieldNames := make([]string, 0, len(fields))
	for _, field := range fields {
		if fieldMap, ok := field.(map[string]interface{}); ok {
			if name, ok := fieldMap["name"].(string); ok {
				fieldNames = append(fieldNames, name)
			}
		}
	}

	log.Info("Available fields", "object", objectName, "count", len(fieldNames), "fields", fieldNames)
	telemetry.AddSpanAttribute(span, "salesforce.available_fields_count", len(fieldNames))
	telemetry.SetSpanSuccess(span)
	return nil
}

// buildSOQL constructs a SOQL query with pagination
func buildSOQL(objectName string, fields []string, whereClause string, lastId string, limit int) string {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(fields, ", "), objectName)

	conditions := []string{}
	if whereClause != "" {
		conditions = append(conditions, whereClause)
	}
	if lastId != "" {
		conditions = append(conditions, fmt.Sprintf("Id > '%s'", lastId))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY Id"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	return query
}

func getRecordId(record reflect.Value) string {
	if record.Kind() == reflect.Map && record.Type().Key().Kind() == reflect.String {
		for _, key := range record.MapKeys() {
			if key.String() == "Id" {
				idValue := record.MapIndex(key)
				if idValue.IsValid() && idValue.CanInterface() {
					return fmt.Sprintf("%v", idValue.Interface())
				}
			}
		}
	}
	return ""
}

func paginateQuery(ctx context.Context, sf *salesforce.Salesforce, objectName string, fields []string, whereClause string, limit int, targetValue reflect.Value) error {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.paginate_query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("salesforce.object", objectName),
			attribute.Int("salesforce.limit", limit),
		),
	)
	defer span.End()

	const batchSize = 200
	totalRetrieved := 0
	lastId := ""
	totalApiCalls := 0

	for {
		batchLimit := batchSize
		if limit > 0 && limit-totalRetrieved < batchSize {
			batchLimit = limit - totalRetrieved
		}

		query := buildSOQL(objectName, fields, whereClause, lastId, batchLimit)

		// Create child span for each API call/batch
		batchCtx, batchSpan := telemetry.StartSpan(ctx, "salesforce.execute_query_batch",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("salesforce.object", objectName),
				attribute.Int("salesforce.batch_limit", batchLimit),
				attribute.Int("salesforce.batch_number", totalApiCalls+1),
			),
		)

		batch, err := executeQuery(batchCtx, sf, query, targetValue.Type())
		totalApiCalls++

		if err != nil {
			telemetry.SetSpanError(batchSpan, err)
			batchSpan.End()
			telemetry.SetSpanError(span, err)
			return fmt.Errorf("failed to query %s: %w", objectName, err)
		}

		if batch.Len() == 0 {
			telemetry.AddSpanAttribute(batchSpan, "salesforce.records_fetched", 0)
			telemetry.SetSpanSuccess(batchSpan)
			batchSpan.End()
			break
		}

		for i := 0; i < batch.Len(); i++ {
			targetValue.Set(reflect.Append(targetValue, batch.Index(i)))
		}

		recordCount := batch.Len()
		totalRetrieved += recordCount

		if recordCount > 0 {
			lastId = getRecordId(batch.Index(recordCount - 1))
		}

		log.Debug("Retrieved batch", "object", objectName, "batch_size", recordCount, "total", totalRetrieved)
		telemetry.AddSpanAttribute(batchSpan, "salesforce.records_fetched", recordCount)
		telemetry.SetSpanSuccess(batchSpan)
		batchSpan.End()

		if (limit > 0 && totalRetrieved >= limit) || recordCount < batchLimit {
			break
		}
	}

	if limit > 0 && targetValue.Len() > limit {
		targetValue.Set(targetValue.Slice(0, limit))
	}

	telemetry.AddSpanAttribute(span, "salesforce.total_records", totalRetrieved)
	telemetry.AddSpanAttribute(span, "salesforce.total_api_calls", totalApiCalls)
	telemetry.SetSpanSuccess(span)
	return nil
}

// executeQuery executes a SOQL query and returns the unmarshaled records
func executeQuery(ctx context.Context, sf *salesforce.Salesforce, query string, targetType reflect.Type) (reflect.Value, error) {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.execute_query",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	encodedQuery := url.QueryEscape(query)
	resp, err := sf.DoRequest("GET", fmt.Sprintf("/query?q=%s", encodedQuery), nil)
	if err != nil {
		telemetry.SetSpanError(span, err)
		return reflect.Value{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		telemetry.SetSpanError(span, err)
		return reflect.Value{}, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Records json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		telemetry.SetSpanError(span, err)
		return reflect.Value{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	batch := reflect.New(targetType).Elem()
	if err := json.Unmarshal(result.Records, batch.Addr().Interface()); err != nil {
		telemetry.SetSpanError(span, err)
		return reflect.Value{}, fmt.Errorf("failed to unmarshal records: %w", err)
	}

	telemetry.SetSpanSuccess(span)
	return batch, nil
}

func AddToMetadata(metadata map[string]string, key string, value any) {
	if value != nil {
		strVal := fmt.Sprintf("%v", value)
		if strVal != "" && strVal != "<nil>" {
			metadata[key] = strVal
		}
	}
}

func UpsertToCtrlplane(ctx context.Context, resources []api.CreateResource, providerName string) error {
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	providerResp, err := ctrlplaneClient.UpsertResourceProviderWithResponse(ctx, workspaceId, providerName)
	if err != nil {
		return fmt.Errorf("failed to upsert resource provider: %w", err)
	}

	if providerResp.JSON200 == nil {
		return fmt.Errorf("failed to upsert resource provider: %s", providerResp.Body)
	}

	providerId := providerResp.JSON200.Id
	log.Info("Upserting resources", "provider", providerName, "count", len(resources))

	setResp, err := ctrlplaneClient.SetResourceProvidersResourcesWithResponse(ctx, providerId, api.SetResourceProvidersResourcesJSONRequestBody{
		Resources: resources,
	})
	if err != nil {
		return fmt.Errorf("failed to set resources: %w", err)
	}

	if setResp.JSON200 == nil {
		return fmt.Errorf("failed to set resources: %s", setResp.Body)
	}

	log.Info("Successfully synced resources", "count", len(resources))
	return nil
}
