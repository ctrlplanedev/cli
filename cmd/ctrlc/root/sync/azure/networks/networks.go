package networks

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/azure/common"
	"github.com/ctrlplanedev/cli/internal/api"
	ctrlp "github.com/ctrlplanedev/cli/internal/common"
	"github.com/ctrlplanedev/cli/internal/kinds"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSyncNetworksCmd() *cobra.Command {
	var subscriptionID string
	var name string

	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Sync Azure Virtual Networks into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure Azure credentials are configured via environment variables or Azure CLI
			
			# Sync all AKS VPCs and subnets from the subscription
			$ ctrlc sync azure networks
			
			# Sync all AKS VPCs and subnets from a specific subscription
			$ ctrlc sync azure networks --subscription-id 00000000-0000-0000-0000-000000000000
			
			# Sync all AKS VPCs and subnets every 5 minutes
			$ ctrlc sync azure networks --interval 5m
		`),
		RunE: runSync(&subscriptionID, &name),
	}

	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&subscriptionID, "subscription-id", "s", "", "Azure Subscription ID")

	return cmd
}

func runSync(subscriptionID, name *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Initialize Azure credential from environment or CLI
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return fmt.Errorf("failed to obtain Azure credential: %w", err)
		}

		// If subscription ID is not provided, get the default one
		if *subscriptionID == "" {
			defaultSubscriptionID, err := getDefaultSubscriptionID(ctx, cred)
			if err != nil {
				return fmt.Errorf("failed to get default subscription ID: %w", err)
			}
			*subscriptionID = defaultSubscriptionID
			log.Info("Using default subscription ID", "subscriptionID", *subscriptionID)
		}

		// Get tenant ID from the subscription
		tenantID, err := getTenantIDFromSubscription(ctx, cred, *subscriptionID)
		if err != nil {
			log.Warn("Failed to get tenant ID from subscription, falling back to environment variables", "error", err)
			tenantID = getTenantIDFromEnv()
		}

		log.Info("Syncing all Networks", "subscriptionID", *subscriptionID, "tenantID", tenantID)

		resources, err := processNetworks(ctx, cred, *subscriptionID, tenantID)
		if err != nil {
			return err
		}

		if len(resources) == 0 {
			log.Info("No Networks found")
			return nil
		}

		// If name is not provided, use subscription ID
		if *name == "" {
			*name = fmt.Sprintf("azure-networks-%s", *subscriptionID)
		}

		// Upsert resources to Ctrlplane
		return ctrlp.UpsertResources(ctx, resources, name)
	}
}

func getTenantIDFromSubscription(ctx context.Context, cred azcore.TokenCredential, subscriptionID string) (string, error) {
	// Create a subscriptions client
	subsClient, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	// Get the subscription details
	resp, err := subsClient.Get(ctx, subscriptionID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get subscription details: %w", err)
	}

	// Extract tenant ID from subscription
	if resp.TenantID == nil || *resp.TenantID == "" {
		return "", fmt.Errorf("subscription doesn't have a tenant ID")
	}

	return *resp.TenantID, nil
}

func getTenantIDFromEnv() string {
	// Check environment variables
	if tenantID := os.Getenv("AZURE_TENANT_ID"); tenantID != "" {
		return tenantID
	}

	// Check viper config
	if tenantID := viper.GetString("azure.tenant-id"); tenantID != "" {
		return tenantID
	}

	return ""
}

func getDefaultSubscriptionID(ctx context.Context, cred azcore.TokenCredential) (string, error) {
	subClient, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create subscription client: %w", err)
	}

	pager := subClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list subscriptions: %w", err)
		}

		// Return the first subscription as default
		if len(page.Value) > 0 && page.Value[0].SubscriptionID != nil {
			return *page.Value[0].SubscriptionID, nil
		}
	}

	return "", fmt.Errorf("no subscriptions found")
}

func processNetworks(
	ctx context.Context, cred azcore.TokenCredential, subscriptionID string, tenantID string,
) ([]api.ResourceProviderResource, error) {
	var allResources []api.ResourceProviderResource
	var resourceGroups []common.ResourceGroupInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	var err error
	var syncErrors []error

	if resourceGroups, err = common.GetResourceGroupInfo(ctx, cred, subscriptionID); err != nil {
		return nil, err
	}

	// Create virtual network client
	client, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Virtual Network client: %w", err)
	}

	for _, rg := range resourceGroups {
		wg.Add(1)
		rgName := rg.Name
		go func(resourceGroup string) {
			defer wg.Done()

			pager := client.NewListPager(resourceGroup, nil)
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("failed to list networks: %w", err))
					mu.Unlock()
					return
				}
				for _, network := range page.Value {
					resources, err := processNetwork(ctx, network, resourceGroup, subscriptionID, tenantID)
					if err != nil {
						log.Error("Failed to process network", "name", *network.Name, "error", err)
						mu.Lock()
						syncErrors = append(syncErrors, fmt.Errorf("network %s: %w", *network.Name, err))
						mu.Unlock()
						return
					}
					mu.Lock()
					allResources = append(allResources, resources...)
					mu.Unlock()
				}
			}
		}(rgName)
	}

	wg.Wait()

	if len(syncErrors) > 0 {
		log.Warn("Some clusters failed to sync", "errors", len(syncErrors))
		// Continue with the clusters that succeeded
	}

	log.Info("Found network resources", "count", len(allResources))
	return allResources, nil
}

func processNetwork(
	_ context.Context, network *armnetwork.VirtualNetwork, resourceGroup string, subscriptionID string, tenantID string,
) ([]api.ResourceProviderResource, error) {
	resources := make([]api.ResourceProviderResource, 0)
	networkName := network.Name
	metadata := initNetworkMetadata(network, resourceGroup, subscriptionID, tenantID)

	// Build console URL
	consoleUrl := getNetworkConsoleUrl(resourceGroup, subscriptionID, *networkName)
	metadata[kinds.CtrlplaneMetadataLinks] = fmt.Sprintf("{ \"Azure Portal\": \"%s\" }", consoleUrl)

	resources = append(resources, api.ResourceProviderResource{
		Version:    "ctrlplane.dev/network/v1",
		Kind:       "AzureNetwork",
		Name:       *networkName,
		Identifier: *network.ID,
		Config: map[string]any{
			// Common cross-provider options
			"name": networkName,
			"type": "vpc",
			"id":   network.ID,

			// Provider-specific implementation details
			"azureVirtualNetwork": map[string]any{
				"type":        network.Type,
				"region":      network.Location,
				"state":       getNetworkState(network),
				"subnetCount": getNetworkSubnetCount(network),
			},
		},
		Metadata: metadata,
	})
	if network.Properties != nil && network.Properties.Subnets != nil {
		for _, subnet := range network.Properties.Subnets {
			if res, err := processSubnet(network, subnet, resourceGroup, subscriptionID, tenantID); err != nil {
				return nil, err
			} else {
				resources = append(resources, res)
			}
		}
	}
	return resources, nil
}

func processSubnet(
	network *armnetwork.VirtualNetwork, subnet *armnetwork.Subnet, resourceGroup string, subscriptionID string, tenantID string,
) (api.ResourceProviderResource, error) {
	metadata := initSubnetMetadata(network, subnet, resourceGroup, subscriptionID, tenantID)
	networkName := network.Name
	subnetName := subnet.Name

	// Build console URL
	consoleUrl := getSubnetConsoleUrl(resourceGroup, subscriptionID, *networkName)
	metadata[kinds.CtrlplaneMetadataLinks] = fmt.Sprintf("{ \"Azure Portal\": \"%s\" }", consoleUrl)

	return api.ResourceProviderResource{
		Version:    "ctrlplane.dev/network/subnet/v1",
		Kind:       "AzureSubnet",
		Name:       *subnetName,
		Identifier: *subnet.ID,
		Config: map[string]any{
			// Common cross-provider options
			"name": subnetName,
			"type": "subnet",
			"id":   subnet.ID,

			// Provider-specific implementation details
			"azureSubnet": map[string]any{
				"type":    subnet.Type,
				"purpose": getSubnetPurpose(subnet),
				"state":   getSubnetState(subnet),
			},
		},
		Metadata: metadata,
	}, nil
}

func initNetworkMetadata(network *armnetwork.VirtualNetwork, resourceGroup, subscriptionID, tenantID string) map[string]string {
	subnetCount := 0
	if network.Properties != nil && network.Properties.Subnets != nil {
		subnetCount = len(network.Properties.Subnets)
	}

	metadata := map[string]string{
		"network/type":         "vpc",
		"network/name":         *network.Name,
		"network/subnet-count": strconv.Itoa(subnetCount),
		"network/id":           *network.ID,
		"azure/subscription":   subscriptionID,
		"azure/tenant":         tenantID,
		"azure/resource-group": resourceGroup,
		"azure/resource-type":  "Microsoft.Network/virtualNetworks/subnets",
		"azure/location":       *network.Location,
		"azure/status":         getNetworkState(network),
		"azure/id":             *network.ID,
		"azure/console-url":    getNetworkConsoleUrl(resourceGroup, subscriptionID, *network.Name),
	}

	// Tags
	if network.Tags != nil {
		for key, value := range network.Tags {
			if value != nil {
				metadata[fmt.Sprintf("tags/%s", key)] = *value
			}
		}
	}

	return metadata
}

func initSubnetMetadata(network *armnetwork.VirtualNetwork, subnet *armnetwork.Subnet, resourceGroup, subscriptionID, tenantID string) map[string]string {

	privateAccess := false
	if subnet.Properties != nil {
		if len(subnet.Properties.PrivateEndpoints) > 0 {
			privateAccess = true
		}
	}

	metadata := map[string]string{
		"network/type":           "subnet",
		"network/name":           *subnet.Name,
		"network/vpc":            *network.Name,
		"network/region":         *network.Location,
		"network/private-access": strconv.FormatBool(privateAccess),
		"azure/subscription":     subscriptionID,
		"azure/tenant":           tenantID,
		"azure/resource-group":   resourceGroup,
		"azure/resource-type":    "Microsoft.Network/virtualNetworks/subnets",
		"azure/location":         *network.Location,
		"azure/status":           getSubnetState(subnet),
		"azure/id":               *subnet.ID,
		"azure/console-url":      getSubnetConsoleUrl(resourceGroup, subscriptionID, *network.Name),
	}

	return metadata
}

func getNetworkConsoleUrl(resourceGroup, subscriptionID, networkName string) string {
	return fmt.Sprintf(
		"https://portal.azure.com/#@/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		subscriptionID,
		resourceGroup,
		networkName,
	)
}

func getNetworkState(network *armnetwork.VirtualNetwork) string {
	return func() string {
		if network.Properties != nil && network.Properties.ProvisioningState != nil {
			return string(*network.Properties.ProvisioningState)
		}
		return ""
	}()
}

func getNetworkSubnetCount(network *armnetwork.VirtualNetwork) int {
	if network.Properties != nil && network.Properties.Subnets != nil {
		return len(network.Properties.Subnets)
	}
	return 0
}

func getSubnetConsoleUrl(resourceGroup, subscriptionID, networkName string) string {
	return fmt.Sprintf(
		"https://portal.azure.com/#@/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets",
		subscriptionID,
		resourceGroup,
		networkName,
	)
}

func getSubnetPurpose(subnet *armnetwork.Subnet) *string {
	if subnet.Properties != nil && subnet.Properties.Purpose != nil {
		return subnet.Properties.Purpose
	}
	return nil
}

func getSubnetState(subnet *armnetwork.Subnet) string {
	if subnet.Properties != nil && subnet.Properties.ProvisioningState != nil {
		return string(*subnet.Properties.ProvisioningState)
	}
	return ""
}
