package prefixes

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

func NewSyncPrefixesCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "prefixes",
		Short: "Sync Netbox prefixes into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox prefixes --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox prefixes into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allPrefixes, err := fetchAllPrefixes(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to list Netbox prefixes: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allPrefixes))
			for _, prefix := range allPrefixes {
				resources = append(resources, mapPrefix(prefix))
			}

			log.Info("Fetched Netbox prefixes", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-prefixes"
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-prefixes)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllPrefixes(ctx context.Context, client *netbox.APIClient) ([]netbox.Prefix, error) {
	var all []netbox.Prefix
	var offset int32

	for {
		res, _, err := client.IpamAPI.
			IpamPrefixesList(ctx).
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

func mapPrefix(prefix netbox.Prefix) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(prefix.Id))
	metadata["netbox/family"] = strconv.Itoa(int(prefix.Family.GetValue()))

	if prefix.Status != nil {
		metadata["netbox/status"] = string(prefix.Status.GetValue())
	}
	if vrf, ok := prefix.GetVrfOk(); ok && vrf != nil {
		metadata["netbox/vrf"] = vrf.GetName()
	}
	if tenant, ok := prefix.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}
	if role, ok := prefix.GetRoleOk(); ok && role != nil {
		metadata["netbox/role"] = role.GetName()
	}
	if vlan, ok := prefix.GetVlanOk(); ok && vlan != nil {
		metadata["netbox/vlan"] = vlan.GetDisplay()
	}

	for _, tag := range prefix.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	config := map[string]interface{}{
		"id":     prefix.Id,
		"prefix": prefix.Prefix,
	}
	if vrf, ok := prefix.GetVrfOk(); ok && vrf != nil {
		config["vrf"] = vrf.GetName()
	}
	if tenant, ok := prefix.GetTenantOk(); ok && tenant != nil {
		config["tenant"] = tenant.GetName()
	}
	if vlan, ok := prefix.GetVlanOk(); ok && vlan != nil {
		config["vlan"] = vlan.GetDisplay()
	}

	return api.ResourceProviderResource{
		Version:    "netbox/prefix/v1",
		Kind:       "Prefix",
		Name:       prefix.Prefix,
		Identifier: strconv.Itoa(int(prefix.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
