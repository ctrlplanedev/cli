package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/common"
	"github.com/spf13/cobra"
)

func NewSyncDevicesCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string
	var query string
	var siteFilter []string
	var roleFilter []string
	var statusFilter []string
	var statusExcludeFilter []string
	var tagFilter []string
	var tenantFilter []string

	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Sync Netbox devices into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox devices --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox devices into Ctrlplane")

			filters := deviceFilters{
				Query:         query,
				Site:          siteFilter,
				Role:          roleFilter,
				Status:        statusFilter,
				StatusExclude: statusExcludeFilter,
				Tag:           tagFilter,
				Tenant:        tenantFilter,
			}

			allDevices, err := fetchAllDevicesDirect(ctx, netboxURL, netboxToken, filters)
			if err != nil {
				return fmt.Errorf("failed to list Netbox devices: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allDevices))
			for _, device := range allDevices {
				resources = append(resources, mapDevice(device))
			}

			log.Info("Fetched Netbox devices", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-devices"
			}

			for _, resource := range resources {
				b, err := json.MarshalIndent(resource, "", "  ")
				if err != nil {
					fmt.Printf("error marshaling resource: %v\n", err)
					continue
				}
				fmt.Println(string(b))
			}

			return common.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-devices)")
	cmd.Flags().StringVar(&query, "q", "", "Search query for Netbox devices")
	cmd.Flags().StringSliceVar(&siteFilter, "site", nil, "Filter by Netbox site slug/name (repeatable)")
	cmd.Flags().StringSliceVar(&roleFilter, "role", nil, "Filter by Netbox device role slug/name (repeatable)")
	cmd.Flags().StringSliceVar(&statusFilter, "status", nil, "Filter by Netbox status (repeatable)")
	cmd.Flags().StringSliceVar(&statusExcludeFilter, "status-n", nil, "Exclude Netbox status values (repeatable)")
	cmd.Flags().StringSliceVar(&tagFilter, "tag", nil, "Filter by Netbox tag slug (repeatable)")
	cmd.Flags().StringSliceVar(&tenantFilter, "tenant", nil, "Filter by Netbox tenant slug/name (repeatable)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}
