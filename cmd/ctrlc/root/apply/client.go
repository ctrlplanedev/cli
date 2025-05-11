package apply

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

func createAPIClient() (*api.ClientWithResponses, uuid.UUID, error) {
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	workspaceID, err := uuid.Parse(workspace)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid workspace ID: %w", err)
	}

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return client, workspaceID, nil
}
