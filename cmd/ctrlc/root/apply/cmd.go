package apply

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// NewApplyCmd creates a new apply command
func NewApplyCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration file to create systems and deployments",
		Long:  `Apply a YAML configuration file to create systems and deployments in Ctrlplane`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(filePath)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the YAML configuration file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runApply(filePath string) error {
	ctx := context.Background()

	// Read and parse the YAML file
	config, err := readConfigFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Create API client
	client, workspaceID, err := createAPIClient()
	if err != nil {
		return err
	}

	// Process systems and collect errors
	processAllSystems(ctx, client, workspaceID, config.Systems)
	processResourceProvider(ctx, client, workspaceID.String(), config.Providers)
	processResourceRelationships(ctx, client, workspaceID.String(), config.Relationships)

	return nil
}
