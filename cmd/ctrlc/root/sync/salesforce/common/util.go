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
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/viper"
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

func GetCustomFieldValue(obj interface{}, fieldName string) (string, bool) {
	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	if customFields := objValue.FieldByName("CustomFields"); customFields.IsValid() && customFields.Kind() == reflect.Map {
		if value := customFields.MapIndex(reflect.ValueOf(fieldName)); value.IsValid() {
			return fmt.Sprintf("%v", value.Interface()), true
		}
	}

	objType := objValue.Type()
	for i := 0; i < objType.NumField(); i++ {
		field := objType.Field(i)
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName == fieldName {
				fieldValue := objValue.Field(i)
				if fieldValue.IsValid() && fieldValue.CanInterface() {
					return fmt.Sprintf("%v", fieldValue.Interface()), true
				}
			}
		}
	}

	return "", false
}

// QuerySalesforceObject performs a generic query on any Salesforce object with pagination support
func QuerySalesforceObject(ctx context.Context, sf *salesforce.Salesforce, objectName string, limit int, listAllFields bool, target interface{}, additionalFields []string, whereClause string) error {
	targetValue := reflect.ValueOf(target).Elem()
	if targetValue.Kind() != reflect.Slice {
		return fmt.Errorf("target must be a pointer to a slice")
	}

	fieldNames := getFieldsToQuery(targetValue.Type().Elem(), additionalFields)

	if listAllFields {
		if err := logAvailableFields(sf, objectName); err != nil {
			return err
		}
	}

	return paginateQuery(ctx, sf, objectName, fieldNames, whereClause, limit, targetValue)
}

// getFieldsToQuery extracts field names from struct tags and merges with additional fields
func getFieldsToQuery(elementType reflect.Type, additionalFields []string) []string {
	// Use a map to automatically handle deduplication
	fieldMap := make(map[string]bool)

	for i := 0; i < elementType.NumField(); i++ {
		field := elementType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if fieldName := strings.Split(jsonTag, ",")[0]; fieldName != "" {
				fieldMap[fieldName] = true
			}
		}
	}

	for _, field := range additionalFields {
		fieldMap[field] = true
	}

	fields := make([]string, 0, len(fieldMap))
	for field := range fieldMap {
		fields = append(fields, field)
	}
	return fields
}

func logAvailableFields(sf *salesforce.Salesforce, objectName string) error {
	resp, err := sf.DoRequest("GET", fmt.Sprintf("/sobjects/%s/describe", objectName), nil)
	if err != nil {
		return fmt.Errorf("failed to describe %s object: %w", objectName, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode describe response: %w", err)
	}

	fields, ok := result["fields"].([]interface{})
	if !ok {
		return fmt.Errorf("unexpected describe response format")
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
	return nil
}

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
	} else {
		query += " LIMIT 2000" // Default batch size
	}

	return query
}

func getRecordId(record reflect.Value) string {
	if record.Kind() != reflect.Struct {
		return ""
	}
	if idField := record.FieldByName("ID"); idField.IsValid() && idField.Kind() == reflect.String {
		return idField.String()
	}
	if idField := record.FieldByName("Id"); idField.IsValid() && idField.Kind() == reflect.String {
		return idField.String()
	}
	return ""
}

func paginateQuery(ctx context.Context, sf *salesforce.Salesforce, objectName string, fields []string, whereClause string, limit int, targetValue reflect.Value) error {
	const batchSize = 2000
	totalRetrieved := 0
	lastId := ""

	for {
		batchLimit := batchSize
		if limit > 0 && limit-totalRetrieved < batchSize {
			batchLimit = limit - totalRetrieved
		}

		query := buildSOQL(objectName, fields, whereClause, lastId, batchLimit)
		batch, err := executeQuery(sf, query, targetValue.Type())
		if err != nil {
			return fmt.Errorf("failed to query %s: %w", objectName, err)
		}

		if batch.Len() == 0 {
			break
		}

		for i := 0; i < batch.Len(); i++ {
			targetValue.Set(reflect.Append(targetValue, batch.Index(i)))
		}

		recordCount := batch.Len()
		totalRetrieved += recordCount

		// Get last ID for next page
		if recordCount > 0 {
			lastId = getRecordId(batch.Index(recordCount - 1))
		}

		log.Debug("Retrieved batch", "object", objectName, "batch_size", recordCount, "total", totalRetrieved)

		if (limit > 0 && totalRetrieved >= limit) || recordCount < batchLimit {
			break
		}
	}

	// Trim to exact limit if needed
	if limit > 0 && targetValue.Len() > limit {
		targetValue.Set(targetValue.Slice(0, limit))
	}

	return nil
}

// executeQuery executes a SOQL query and returns the unmarshaled records
func executeQuery(sf *salesforce.Salesforce, query string, targetType reflect.Type) (reflect.Value, error) {
	encodedQuery := url.QueryEscape(query)
	resp, err := sf.DoRequest("GET", fmt.Sprintf("/query?q=%s", encodedQuery), nil)
	if err != nil {
		return reflect.Value{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Records json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Create a new slice of the target type to unmarshal into
	batch := reflect.New(targetType).Elem()
	if err := json.Unmarshal(result.Records, batch.Addr().Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to unmarshal records: %w", err)
	}

	return batch, nil
}

func GetKnownFieldsFromStruct(structType reflect.Type) map[string]bool {
	knownFields := make(map[string]bool)

	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		jsonTag := field.Tag.Get("json")

		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]
		if fieldName != "" {
			knownFields[fieldName] = true
		}
	}

	return knownFields
}

func UnmarshalWithCustomFields(data []byte, target interface{}, knownFields map[string]bool) (map[string]interface{}, error) {
	if err := json.Unmarshal(data, target); err != nil {
		return nil, err
	}

	var allFields map[string]interface{}
	if err := json.Unmarshal(data, &allFields); err != nil {
		return nil, err
	}

	customFields := make(map[string]interface{})
	for fieldName, value := range allFields {
		if !knownFields[fieldName] {
			customFields[fieldName] = value
		}
	}

	return customFields, nil
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
