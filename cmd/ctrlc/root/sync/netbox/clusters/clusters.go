package clusters

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	ctrlp "github.com/ctrlplanedev/cli/internal/common"
	netbox "github.com/netbox-community/go-netbox/v4"
	"github.com/spf13/cobra"
)

const pageSize int32 = 100

func NewSyncClustersCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "clusters",
		Short: "Sync Netbox clusters into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox clusters --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox clusters into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allClusters, err := fetchAllClusters(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to list Netbox clusters: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allClusters))
			for _, cluster := range allClusters {
				resources = append(resources, mapCluster(cluster))
			}

			log.Info("Fetched Netbox clusters", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-clusters"
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-clusters)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllClusters(ctx context.Context, client *netbox.APIClient) ([]netbox.Cluster, error) {
	var all []netbox.Cluster
	var offset int32

	for {
		res, _, err := client.VirtualizationAPI.
			VirtualizationClustersList(ctx).
			Limit(pageSize).
			Offset(offset).
			Execute()
		if err != nil {
			return nil, err
		}

		all = append(all, res.Results...)

		if int32(len(all)) >= res.Count {
			break
		}
		offset += pageSize
	}

	return all, nil
}

func mapCluster(cluster netbox.Cluster) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(cluster.Id))
	metadata["netbox/type"] = cluster.Type.GetName()

	if cluster.Status != nil {
		metadata["netbox/status"] = string(cluster.Status.GetValue())
	}
	if group, ok := cluster.GetGroupOk(); ok && group != nil {
		metadata["netbox/group"] = group.GetName()
	}
	if tenant, ok := cluster.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}

	for _, tag := range cluster.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	config := map[string]interface{}{
		"id":   cluster.Id,
		"name": cluster.Name,
		"type": cluster.Type.GetName(),
	}
	if group, ok := cluster.GetGroupOk(); ok && group != nil {
		config["group"] = group.GetName()
	}
	if tenant, ok := cluster.GetTenantOk(); ok && tenant != nil {
		config["tenant"] = tenant.GetName()
	}

	return api.ResourceProviderResource{
		Version:    "netbox/cluster/v1",
		Kind:       "Cluster",
		Name:       cluster.Name,
		Identifier: strconv.Itoa(int(cluster.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
