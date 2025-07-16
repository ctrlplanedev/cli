package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/viper"
)

// DynamicFieldHolder interface for structs that can hold custom fields
type DynamicFieldHolder interface {
	GetCustomFields() map[string]interface{}
}

// ExtractFieldsFromMetadataMappings extracts Salesforce field names from metadata mappings
func ExtractFieldsFromMetadataMappings(metadataMappings []string) []string {
	fieldMap := make(map[string]bool)

	for _, mapping := range metadataMappings {
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) == 2 {
			salesforceField := strings.TrimSpace(parts[1])
			if salesforceField != "" {
				fieldMap[salesforceField] = true
			}
		}
	}

	fields := make([]string, 0, len(fieldMap))
	for field := range fieldMap {
		fields = append(fields, field)
	}

	return fields
}

// ParseMappings applies custom field mappings to extract string values for metadata
func ParseMappings(data interface{}, mappings []string, defaultMappings map[string]string) map[string]string {
	result := map[string]string{}

	dataValue := reflect.ValueOf(data)
	dataType := reflect.TypeOf(data)

	if dataValue.Kind() == reflect.Ptr {
		dataValue = dataValue.Elem()
		dataType = dataType.Elem()
	}

	// Process custom mappings (format: ctrlplane/key=SalesforceField)
	for _, mapping := range mappings {
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) != 2 {
			log.Warn("Invalid mapping format, skipping", "mapping", mapping)
			continue
		}

		ctrlplaneKey, sfField := parts[0], parts[1]

		found := false
		for i := 0; i < dataType.NumField(); i++ {
			field := dataType.Field(i)
			jsonTag := field.Tag.Get("json")

			if jsonTag == sfField {
				fieldValue := dataValue.Field(i)

				strValue := fieldToString(fieldValue)

				if strValue != "" {
					result[ctrlplaneKey] = strValue
				}
				found = true
				break
			}
		}

		// If not found in struct fields, check CustomFields if the struct implements DynamicFieldHolder
		if !found {
			if holder, ok := data.(DynamicFieldHolder); ok {
				customFields := holder.GetCustomFields()
				if customFields != nil {
					if value, exists := customFields[sfField]; exists {
						strValue := fmt.Sprintf("%v", value)
						if strValue != "" && strValue != "<nil>" {
							result[ctrlplaneKey] = strValue
						}
					}
				}
			}
		}
	}

	// Add default mappings if they haven't been explicitly mapped
	existingKeys := make(map[string]bool)
	for key := range result {
		existingKeys[key] = true
	}

	for defaultKey, sfField := range defaultMappings {
		if !existingKeys[defaultKey] {
			// Find and add the default field
			for i := 0; i < dataType.NumField(); i++ {
				field := dataType.Field(i)
				if field.Tag.Get("json") == sfField {
					fieldValue := dataValue.Field(i)
					strValue := fieldToString(fieldValue)

					if strValue != "" {
						result[defaultKey] = strValue
					}
					break
				}
			}
		}
	}

	return result
}

// fieldToString converts a reflect.Value to string for metadata
func fieldToString(fieldValue reflect.Value) string {
	switch fieldValue.Kind() {
	case reflect.String:
		return fieldValue.String()
	case reflect.Int, reflect.Int64:
		if val := fieldValue.Int(); val != 0 {
			return fmt.Sprintf("%d", val)
		}
	case reflect.Float64, reflect.Float32:
		if val := fieldValue.Float(); val != 0 {
			return strconv.FormatFloat(val, 'f', -1, 64)
		}
	case reflect.Bool:
		if fieldValue.Bool() {
			return "true"
		}
		return "false"
	default:
		// For complex types, try to marshal to JSON
		if fieldValue.IsValid() && fieldValue.CanInterface() {
			if bytes, err := json.Marshal(fieldValue.Interface()); err == nil {
				return string(bytes)
			}
		}
	}

	return ""
}

// QuerySalesforceObject performs a generic query on any Salesforce object with pagination support
func QuerySalesforceObject(ctx context.Context, sf *salesforce.Salesforce, objectName string, limit int, listAllFields bool, target interface{}, additionalFields []string, whereClause string) error {
	targetValue := reflect.ValueOf(target).Elem()
	if targetValue.Kind() != reflect.Slice {
		return fmt.Errorf("target must be a pointer to a slice")
	}
	elementType := targetValue.Type().Elem()

	// Get field names from the struct
	fieldNames := []string{}
	for i := 0; i < elementType.NumField(); i++ {
		field := elementType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			fieldName := strings.Split(jsonTag, ",")[0]
			if fieldName != "" {
				fieldNames = append(fieldNames, fieldName)
			}
		}
	}

	// Include additional fields passed in (e.g., from metadata mappings)
	for _, field := range additionalFields {
		found := false
		for _, existing := range fieldNames {
			if existing == field {
				found = true
				break
			}
		}
		if !found {
			fieldNames = append(fieldNames, field)
		}
	}

	// If listAllFields is true, describe the object to show what's available
	if listAllFields {
		describeResp, err := sf.DoRequest("GET", fmt.Sprintf("/sobjects/%s/describe", objectName), nil)
		if err != nil {
			return fmt.Errorf("failed to describe %s object: %w", objectName, err)
		}
		defer describeResp.Body.Close()

		var describeResult map[string]interface{}
		if err := json.NewDecoder(describeResp.Body).Decode(&describeResult); err != nil {
			return fmt.Errorf("failed to decode describe response: %w", err)
		}

		// Extract all available field names for logging
		fields, ok := describeResult["fields"].([]interface{})
		if !ok {
			return fmt.Errorf("unexpected describe response format")
		}

		allFieldNames := []string{}
		for _, field := range fields {
			fieldMap, ok := field.(map[string]interface{})
			if !ok {
				continue
			}
			if name, ok := fieldMap["name"].(string); ok {
				allFieldNames = append(allFieldNames, name)
			}
		}
		log.Info("Available fields", "object", objectName, "count", len(allFieldNames), "fields", allFieldNames)
	}

	// Build query with pagination support
	totalRetrieved := 0
	lastId := ""
	batchSize := 2000

	// Query in batches using ID-based pagination to avoid OFFSET limits
	for {
		fieldsClause := strings.Join(fieldNames, ", ")
		baseQuery := fmt.Sprintf("SELECT %s FROM %s", fieldsClause, objectName)

		paginatedQuery := baseQuery
		whereClauses := []string{}

		if whereClause != "" {
			whereClauses = append(whereClauses, whereClause)
		}

		if lastId != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("Id > '%s'", lastId))
		}

		if len(whereClauses) > 0 {
			paginatedQuery += " WHERE " + strings.Join(whereClauses, " AND ")
		}
		paginatedQuery += " ORDER BY Id"

		if limit > 0 && limit-totalRetrieved < batchSize {
			paginatedQuery += fmt.Sprintf(" LIMIT %d", limit-totalRetrieved)
		} else {
			paginatedQuery += fmt.Sprintf(" LIMIT %d", batchSize)
		}

		encodedQuery := url.QueryEscape(paginatedQuery)
		queryURL := fmt.Sprintf("/query?q=%s", encodedQuery)

		queryResp, err := sf.DoRequest("GET", queryURL, nil)
		if err != nil {
			return fmt.Errorf("failed to query %s: %w", objectName, err)
		}

		body, err := io.ReadAll(queryResp.Body)
		if err != nil {
			queryResp.Body.Close()
			return fmt.Errorf("failed to read response body: %w", err)
		}
		queryResp.Body.Close()

		var queryResult struct {
			TotalSize      int             `json:"totalSize"`
			Done           bool            `json:"done"`
			Records        json.RawMessage `json:"records"`
			NextRecordsUrl string          `json:"nextRecordsUrl"`
		}

		if err := json.Unmarshal(body, &queryResult); err != nil {
			return fmt.Errorf("failed to unmarshal query response: %w", err)
		}

		batchSlice := reflect.New(targetValue.Type()).Elem()

		// Unmarshal records into the batch slice - this will trigger our custom UnmarshalJSON
		if err := json.Unmarshal(queryResult.Records, batchSlice.Addr().Interface()); err != nil {
			return fmt.Errorf("failed to unmarshal records: %w", err)
		}

		if batchSlice.Len() == 0 {
			break
		}

		for i := 0; i < batchSlice.Len(); i++ {
			targetValue.Set(reflect.Append(targetValue, batchSlice.Index(i)))
		}

		recordCount := batchSlice.Len()
		totalRetrieved += recordCount

		if recordCount > 0 {
			lastRecord := batchSlice.Index(recordCount - 1)
			if lastRecord.Kind() == reflect.Struct {
				idField := lastRecord.FieldByName("ID")
				if !idField.IsValid() {
					idField = lastRecord.FieldByName("Id")
				}
				if idField.IsValid() && idField.Kind() == reflect.String {
					lastId = idField.String()
				}
			}
		}

		log.Debug("Retrieved batch", "object", objectName, "batch_size", recordCount, "total", totalRetrieved)

		if limit > 0 && totalRetrieved >= limit {
			break
		}

		if recordCount == 0 {
			break
		}
	}

	if limit > 0 && targetValue.Len() > limit {
		targetValue.Set(targetValue.Slice(0, limit))
	}

	return nil
}

// ExtractCustomFields extracts fields from a JSON response that aren't in the struct
func ExtractCustomFields(data []byte, knownFields map[string]bool) (map[string]interface{}, error) {
	var allFields map[string]interface{}
	if err := json.Unmarshal(data, &allFields); err != nil {
		return nil, err
	}

	customFields := make(map[string]interface{})
	for key, value := range allFields {
		if !knownFields[key] {
			customFields[key] = value
		}
	}

	return customFields, nil
}

// GetKnownFieldsFromStruct extracts all JSON field names from a struct's tags
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

// UpsertToCtrlplane creates or updates a resource provider and sets its resources
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

// UnmarshalWithCustomFields unmarshals JSON data into the target struct and returns any unknown fields
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
