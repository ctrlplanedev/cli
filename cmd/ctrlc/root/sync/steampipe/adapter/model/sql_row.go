package model

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/xeipuuv/gojsonschema"
	"strings"
)

type SqlRow struct {
	EntityName string
	Json       string
}

// ValidateAndUnmarshal ValidateAndMarshal is a convenience function to validate and unmarshal JSON to T.
// It assumes the schemaLoader and T are compatible.
func ValidateAndUnmarshal[T any](schemaLoader gojsonschema.JSONLoader, jsonStr string) (T, bool) {
	var zero T
	rowJsonLoader := gojsonschema.NewStringLoader(jsonStr)

	// Perform validation
	result, err := gojsonschema.Validate(schemaLoader, rowJsonLoader)
	if err != nil {
		fmt.Printf("Error validating JSON: %s\n", err)
		return zero, false
	}

	// Check validation result
	if !result.Valid() {
		var errors []string
		for _, desc := range result.Errors() {
			errors = append(errors, desc.String())
		}
		log.Errorf("Invalid JSON:\n%s", strings.Join(errors, "\n"))
		return zero, false
	}

	var row T
	if err := json.Unmarshal([]byte(jsonStr), &row); err != nil {
		log.Errorf("Failed to unmarshal JSON: %s", err)
		return zero, false
	}

	return row, true
}
