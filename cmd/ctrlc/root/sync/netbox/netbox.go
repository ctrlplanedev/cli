package netbox

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/clusters"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/devices"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/ipaddresses"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/prefixes"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/sites"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/netbox/vms"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
)

func NewNetboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "netbox",
		Short: "Sync Netbox resources into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync all Netbox devices
			$ ctrlc sync netbox devices --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN

			# Sync VMs every 5 minutes
			$ ctrlc sync netbox vms --netbox-url https://netbox.example.com --netbox-token $NETBOX_TOKEN --interval 5m
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(cliutil.AddIntervalSupport(devices.NewSyncDevicesCmd(), ""))
	cmd.AddCommand(cliutil.AddIntervalSupport(vms.NewSyncVMsCmd(), ""))
	cmd.AddCommand(cliutil.AddIntervalSupport(ipaddresses.NewSyncIPAddressesCmd(), ""))
	cmd.AddCommand(cliutil.AddIntervalSupport(prefixes.NewSyncPrefixesCmd(), ""))
	cmd.AddCommand(cliutil.AddIntervalSupport(sites.NewSyncSitesCmd(), ""))
	cmd.AddCommand(cliutil.AddIntervalSupport(clusters.NewSyncClustersCmd(), ""))

	return cmd
}
