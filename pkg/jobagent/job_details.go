package jobagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/viper"
)

func fetchJobDetails(ctx context.Context, jobID string) (map[string]interface{}, error) {
	client, err := api.NewAPIKeyClientWithResponses(viper.GetString("url"), viper.GetString("api-key"))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client for job details: %w", err)
	}

	resp, err := client.GetJobWithResponse(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job details: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("received empty response from job details API")
	}

	var details map[string]interface{}
	detailsBytes, err := json.Marshal(resp.JSON200)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job response: %w", err)
	}
	if err := json.Unmarshal(detailsBytes, &details); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job details: %w", err)
	}
	return details, nil
}
