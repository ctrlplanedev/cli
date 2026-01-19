package systems

import (
	"fmt"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewListSystemsCmd() *cobra.Command {
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "systems",
		Short: "List systems",
		Long:  `Commands for getting systems.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			params := &api.ListSystemsParams{}
			if limit > 0 {
				params.Limit = &limit
			}
			if offset > 0 {
				params.Offset = &offset
			}
			resp, err := client.ListSystems(cmd.Context(), workspaceID.String(), params)
			if err != nil {

				return fmt.Errorf("failed to get systems: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Offset the results")

	cmd.MarkFlagRequired("workspace")

	return cmd
}
