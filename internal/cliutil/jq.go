package cliutil

import (
	"fmt"

	"github.com/itchyny/gojq"
)

// ApplyJQ applies a jq expression to input data and returns the results.
func ApplyJQ(expression string, input interface{}) ([]interface{}, error) {
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}

	iter := query.Run(input)
	var results []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return nil, fmt.Errorf("jq evaluation error: %w", err)
		}
		results = append(results, v)
	}
	return results, nil
}
