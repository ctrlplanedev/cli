package ipaddresses

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

func NewSyncIPAddressesCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "ip-addresses",
		Short: "Sync Netbox IP addresses into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox ip-addresses --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox IP addresses into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allIPs, err := fetchAllIPAddresses(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to list Netbox IP addresses: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allIPs))
			for _, ip := range allIPs {
				resources = append(resources, mapIPAddress(ip))
			}

			log.Info("Fetched Netbox IP addresses", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-ip-addresses"
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-ip-addresses)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllIPAddresses(ctx context.Context, client *netbox.APIClient) ([]netbox.IPAddress, error) {
	var all []netbox.IPAddress
	var offset int32

	for {
		res, _, err := client.IpamAPI.
			IpamIpAddressesList(ctx).
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

func mapIPAddress(ip netbox.IPAddress) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(ip.Id))
	metadata["netbox/family"] = strconv.Itoa(int(ip.Family.GetValue()))

	if ip.Status != nil {
		metadata["netbox/status"] = string(ip.Status.GetValue())
	}
	if ip.Role != nil {
		metadata["netbox/role"] = string(ip.Role.GetValue())
	}
	if vrf, ok := ip.GetVrfOk(); ok && vrf != nil {
		metadata["netbox/vrf"] = vrf.GetName()
	}
	if tenant, ok := ip.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}

	for _, tag := range ip.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	config := map[string]interface{}{
		"id":      ip.Id,
		"address": ip.Address,
	}
	if ip.DnsName != nil && *ip.DnsName != "" {
		config["dns_name"] = *ip.DnsName
	}
	if vrf, ok := ip.GetVrfOk(); ok && vrf != nil {
		config["vrf"] = vrf.GetName()
	}
	if tenant, ok := ip.GetTenantOk(); ok && tenant != nil {
		config["tenant"] = tenant.GetName()
	}

	return api.ResourceProviderResource{
		Version:    "netbox/ip-address/v1",
		Kind:       "IPAddress",
		Name:       ip.Address,
		Identifier: strconv.Itoa(int(ip.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
