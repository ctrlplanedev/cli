package vms

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

func NewSyncVMsCmd() *cobra.Command {
	var netboxURL string
	var netboxToken string
	var providerName string

	cmd := &cobra.Command{
		Use:   "vms",
		Short: "Sync Netbox virtual machines into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync netbox vms --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Info("Syncing Netbox virtual machines into Ctrlplane")

			client := netbox.NewAPIClientFor(netboxURL, netboxToken)

			allVMs, err := fetchAllVMs(ctx, client)
			if err != nil {
				return fmt.Errorf("failed to list Netbox VMs: %w", err)
			}

			resources := make([]api.ResourceProviderResource, 0, len(allVMs))
			for _, vm := range allVMs {
				resources = append(resources, mapVM(vm))
			}

			log.Info("Fetched Netbox virtual machines", "count", len(resources))

			if providerName == "" {
				providerName = "netbox-vms"
			}

			return ctrlp.UpsertResources(ctx, resources, &providerName)
		},
	}

	cmd.Flags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "Netbox instance URL")
	cmd.Flags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "Netbox API token")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name (default: netbox-vms)")

	cmd.MarkFlagRequired("netbox-url")
	cmd.MarkFlagRequired("netbox-token")

	return cmd
}

func fetchAllVMs(ctx context.Context, client *netbox.APIClient) ([]netbox.VirtualMachineWithConfigContext, error) {
	var all []netbox.VirtualMachineWithConfigContext
	var offset int32

	for {
		res, _, err := client.VirtualizationAPI.
			VirtualizationVirtualMachinesList(ctx).
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

func mapVM(vm netbox.VirtualMachineWithConfigContext) api.ResourceProviderResource {
	metadata := map[string]string{}

	metadata["netbox/id"] = strconv.Itoa(int(vm.Id))

	if vm.Status != nil {
		metadata["netbox/status"] = string(vm.Status.GetValue())
	}
	if cluster, ok := vm.GetClusterOk(); ok && cluster != nil {
		metadata["netbox/cluster"] = cluster.GetName()
	}
	if site, ok := vm.GetSiteOk(); ok && site != nil {
		metadata["netbox/site"] = site.GetName()
	}
	if tenant, ok := vm.GetTenantOk(); ok && tenant != nil {
		metadata["netbox/tenant"] = tenant.GetName()
	}
	if role, ok := vm.GetRoleOk(); ok && role != nil {
		metadata["netbox/role"] = role.GetName()
	}

	for _, tag := range vm.Tags {
		metadata[fmt.Sprintf("netbox/tag/%s", tag.Slug)] = "true"
	}

	config := map[string]interface{}{
		"id":   vm.Id,
		"name": vm.Name,
	}
	if cluster, ok := vm.GetClusterOk(); ok && cluster != nil {
		config["cluster"] = cluster.GetName()
	}
	if vcpus, ok := vm.GetVcpusOk(); ok && vcpus != nil {
		config["vcpus"] = *vcpus
	}
	if memory, ok := vm.GetMemoryOk(); ok && memory != nil {
		config["memory"] = *memory
	}
	if disk, ok := vm.GetDiskOk(); ok && disk != nil {
		config["disk"] = *disk
	}
	if pip, ok := vm.GetPrimaryIpOk(); ok && pip != nil {
		config["primary_ip"] = pip.GetAddress()
	}

	return api.ResourceProviderResource{
		Version:    "netbox/vm/v1",
		Kind:       "VirtualMachine",
		Name:       vm.Name,
		Identifier: strconv.Itoa(int(vm.Id)),
		Config:     config,
		Metadata:   metadata,
	}
}
