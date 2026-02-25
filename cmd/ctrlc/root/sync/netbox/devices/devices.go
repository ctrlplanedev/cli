package devices

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

func NewSyncDevicesCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Sync Netbox devices into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox devices --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox devices into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allDevices, err := fetchAllDevices(ctx, client)
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

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-devices)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllDevices(ctx context.Context, client *netbox.APIClient) ([]netbox.DeviceWithConfigContext, error) {
	var all []netbox.DeviceWithConfigContext
	var offset int32
	page := 1

	for {
		log.Info("Fetching Netbox devices page", "page", page, "offset", offset, "limit", pageSize)
		res, _, err := client.DcimAPI.
			DcimDevicesList(ctx).
			Limit(pageSize).
			Offset(offset).
			Execute()
		if err != nil {
			log.Error(err, "Failed to fetch Netbox devices page", "page", page, "offset", offset)
			return nil, err
		}

		log.Info("Fetched devices from Netbox page", "page", page, "count", len(res.Results), "total", res.Count)
		all = append(all, res.Results...)

		if int32(len(all)) >= res.Count {
			log.Info("All Netbox devices fetched", "total_count", len(all))
			break
		}
		offset += pageSize
		page++
	}

	return all, nil
}

func mapDevice(device netbox.DeviceWithConfigContext) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(device.Id))
	metadata["netbox/site"] = device.GetSite().Name

	if device.Status != nil {
		metadata["netbox/status"] = string(device.Status.GetValue())
	}
	if rack, ok := device.GetRackOk(); ok && rack != nil {
		metadata["netbox/rack"] = rack.GetName()
	}
	if tenant, ok := device.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}
	if role := device.Role; role.Name != "" {
		metadata["netbox/role"] = role.Name
	}
	if platform, ok := device.GetPlatformOk(); ok && platform != nil {
		metadata["netbox/platform"] = platform.GetName()
	}

	for _, tag := range device.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	site := device.GetSite()
	config := map[string]any{
		"id":          device.Id,
		"name":        device.GetName(),
		"device_type": device.DeviceType.GetDisplay(),
		"role":        device.Role.GetName(),
		"site": map[string]any{
			"id":   site.Id,
			"url":  site.Url,
			"slug": site.Slug,
			"name": site.Name,
		},
	}
	if device.Serial != nil {
		config["serial"] = *device.Serial
	}
	if pip, ok := device.GetPrimaryIpOk(); ok && pip != nil {
		config["primaryIp"] = pip.GetAddress()
	}
	if platform, ok := device.GetPlatformOk(); ok && platform != nil {
		config["platform"] = platform.GetName()
	}

	name := device.GetName()
	if name == "" {
		name = device.Display
	}

	return api.ResourceProviderResource{
		Version:    "netbox/device/v1",
		Kind:       "Device",
		Name:       name,
		Identifier: strconv.Itoa(int(device.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
