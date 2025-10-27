package resources

import (
	"fmt"
	"net/url"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourcesCmd() *cobra.Command {
	var query string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Get resources",
		Long:  `Commands for getting resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			params := &api.GetAllResourcesParams{}
			if limit > 0 {
				params.Limit = &limit
			}
			if offset > 0 {
				params.Offset = &offset
			}
			if query != "" {
				q := url.QueryEscape(query)
				params.Cel = &q
			}
			
			resp, err := client.GetAllResources(cmd.Context(), workspaceID.String(), params)
			if err != nil {
				return fmt.Errorf("failed to get resources: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "CEL filter")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Offset the results")
	
	cmd.MarkFlagRequired("workspace")

	return cmd
}