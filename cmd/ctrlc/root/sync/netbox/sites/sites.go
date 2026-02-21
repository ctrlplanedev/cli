package sites

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

func NewSyncSitesCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Sync Netbox sites into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox sites --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox sites into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allSites, err := fetchAllSites(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to list Netbox sites: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allSites))
			for _, site := range allSites {
				resources = append(resources, mapSite(site))
			}

			log.Info("Fetched Netbox sites", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-sites"
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-sites)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllSites(ctx context.Context, client *netbox.APIClient) ([]netbox.Site, error) {
	var all []netbox.Site
	var offset int32

	for {
		res, _, err := client.DcimAPI.
			DcimSitesList(ctx).
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

func mapSite(site netbox.Site) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(site.Id))
	metadata["netbox/slug"] = site.Slug

	if site.Status != nil {
		metadata["netbox/status"] = string(site.Status.GetValue())
	}
	if region, ok := site.GetRegionOk(); ok && region != nil {
		metadata["netbox/region"] = region.GetName()
	}
	if tenant, ok := site.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}
	if group, ok := site.GetGroupOk(); ok && group != nil {
		metadata["netbox/group"] = group.GetName()
	}

	for _, tag := range site.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	config := map[string]interface{}{
		"id":   site.Id,
		"name": site.Name,
		"slug": site.Slug,
	}
	if site.Facility != nil && *site.Facility != "" {
		config["facility"] = *site.Facility
	}
	if site.PhysicalAddress != nil && *site.PhysicalAddress != "" {
		config["physical_address"] = *site.PhysicalAddress
	}
	if region, ok := site.GetRegionOk(); ok && region != nil {
		config["region"] = region.GetName()
	}
	if tenant, ok := site.GetTenantOk(); ok && tenant != nil {
		config["tenant"] = tenant.GetName()
	}

	return api.ResourceProviderResource{
		Version:    "netbox/site/v1",
		Kind:       "Site",
		Name:       site.Name,
		Identifier: strconv.Itoa(int(site.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
