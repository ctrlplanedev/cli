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

func ParseMetadataMappings(mappings []string) ([]string, map[string]string) {
	fieldMap := make(map[string]bool)
	lookupMap := make(map[string]string) // fieldName -> metadataKey

	for _, mapping := range mappings {
		parts := strings.Split(mapping, "=")
		if len(parts) == 2 {
			metadataKey := parts[0]
			fieldName := parts[1]
			fieldMap[fieldName] = true
			lookupMap[fieldName] = metadataKey
		}
	}

	fields := make([]string, 0, len(fieldMap))
	for field := range fieldMap {
		fields = append(fields, field)
	}

	return fields, lookupMap
}

func GetCustomFieldValue(obj interface{}, fieldName string) (string, bool) {
	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	customFields := objValue.FieldByName("CustomFields")
	if customFields.IsValid() && customFields.Kind() == reflect.Map {
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
	elementType := targetValue.Type().Elem()

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

	totalRetrieved := 0
	lastId := ""
	batchSize := 2000

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
