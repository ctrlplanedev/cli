package steampipe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSyncSteampipeCmd() *cobra.Command {
	var providerName string
	var connection string
	var table string

	cmd := &cobra.Command{
		Use:   "steampipe",
		Short: "Subcommands for integrating steampipe with Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync steampipe -r resource-provider -c steampipe-connection -t steampipe-table
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var provider *api.ResourceProvider
			var providerResp *http.Response
			var resources []api.AgentResource

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")

			ctx := context.Background()

			steampipe, err := NewSteampipeClient(connection)
			if err != nil {
				return fmt.Errorf("failed to create steampipe connection: %w", err)
			}
			defer func(steampipe *Client) {
				err := steampipe.Close()
				if err != nil {
					log.Errorf("failed to close steampipe connection: %v", err)
				}
			}(steampipe)

			apiClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			if resources, err = steampipe.DoSync(workspaceId, table); err != nil {
				return err
			}

			log.Infof("Resource count  %d", len(resources))
			if len(resources) > 0 {
				provider, err = api.NewResourceProvider(apiClient, workspaceId, providerName)
				if err != nil {
					return fmt.Errorf("failed to create resource provider: %w", err)
				}
				providerResp, err = provider.UpsertResource(ctx, resources)
				if providerResp != nil {
					log.Info("upsert resources response ", "status", providerResp.Status)
					bodyContent, err := io.ReadAll(providerResp.Body)
					if err != nil {
						log.Errorf("failed to read response body: %v", err)
					}
					log.Info("upsert resources response ", "body", string(bodyContent))
					providerResp.Body.Close()
				}
				if err != nil {
					return fmt.Errorf("failed to upsert resources: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&providerName, "resource-provider-name", "r", os.Getenv("RESOURCE_PROVIDER"), "The resource group name")
	cmd.Flags().StringVarP(&connection, "steampipe-connection", "c", os.Getenv("STEAMPIPE_CONNECTION"), "The steampipe postgresql connection string to use")
	cmd.Flags().StringVarP(&table, "steampipe-table", "t", os.Getenv("STEAMPIPE_TABLE"), "The steampipe postgresql table to select from")

	cmd.MarkFlagRequired("resource-provider-name")
	cmd.MarkFlagRequired("steampipe-connection")
	cmd.MarkFlagRequired("steampipe-table")

	return cmd
}
