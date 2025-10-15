package resources

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewGetResourcesCmd() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:   "resource [flags]",
		Short: "Get a resource",
		Long:  `Get a resource by specifying either an ID or both a workspace and an identifier.`,
		Example: heredoc.Doc(`
            # Get a resource by workspace and identifier
            $ ctrlc get resource --workspace 00000000-0000-0000-0000-000000000000
        `),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			resp, err := client.ListResources(cmd.Context(), workspace)
			if err != nil {
				return fmt.Errorf("failed to get resource by workspace and identifier: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace of the target resource")

	return cmd
}
