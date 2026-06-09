package common

import (
	"context"
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/resourceprovider"
	"github.com/spf13/viper"
)

func UpsertResources(ctx context.Context, resources []*apiv1.ResourceInput, name *string) error {
	if name == nil || *name == "" {
		return fmt.Errorf("name is unset, invalid usage")
	}

	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewConnectClient(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	rp, err := resourceprovider.New(ctrlplaneClient, workspaceId, *name)
	if err != nil {
		return fmt.Errorf("failed to create resource provider: %w", err)
	}

	if _, err := rp.UpsertResource(ctx, resources); err != nil {
		return fmt.Errorf("failed to upsert resources: %w", err)
	}

	log.Info("Successfully upserted resources", "count", len(resources))
	return nil
}
