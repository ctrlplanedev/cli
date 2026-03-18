package kubernetes

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	ctrlp "github.com/ctrlplanedev/cli/internal/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSyncKubernetesCmd() *cobra.Command {
	var clusterIdentifier string
	var providerName string
	var clusterName string
	var selectors ResourceTypes

	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Sync Kubernetes resources on a cluster",
		Example: heredoc.Doc(`
			$ ctrlc sync kubernetes --cluster-identifier 1234567890 --cluster-name my-cluster
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log.Info("Syncing Kubernetes resources on a cluster")
			if clusterIdentifier == "" {
				clusterIdentifier = viper.GetString("cluster-identifier")
			}

			config, configClusterName, err := getKubeConfig()
			if err != nil {
				return err
			}

			if clusterName == "" {
				clusterName = configClusterName
			}

			log.Info("Connected to cluster", "name", clusterName)

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")

			ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			sync := newSync(clusterIdentifier, workspaceId, ctrlplaneClient, config, clusterName)
			resources, err := sync.process(ctx, selectors)
			if err != nil {
				return err
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&clusterIdentifier, "cluster-identifier", "c", "", "The identifier of the parent cluster in ctrlplane (if not provided, will use the CLUSTER_IDENTIFIER environment variable)")
	cmd.Flags().StringVarP(&clusterName, "cluster-name", "n", "", "The name of the cluster")
	cmd.Flags().VarP(&selectors, "selector", "s", "Select resources to sync [nodes|deployments|namespaces]. Repeat the flag to select multiple resources; omit it to sync all resources.")

	return cmd
}
