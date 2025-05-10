package networks

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewSyncNetworksCmd creates a new cobra command for syncing Google Networks
func NewSyncNetworksCmd() *cobra.Command {
	var region string
	var name string

	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Sync AWS VPC networks and subnets into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure AWS credentials are configured via environment variables or application default credentials
			
			# Sync all VPC networks and subnets from a project
			$ ctrlc sync aws networks --project my-project
		`),
		PreRunE: validateFlags(&region),
		RunE:    runSync(&region, &name),
	}

	// Add command flags
	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&region, "region", "r", "", "AWS Region")
	cmd.MarkFlagRequired("region")

	return cmd
}

// validateFlags ensures required flags are set
func validateFlags(region *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *region == "" {
			return fmt.Errorf("region is required")
		}
		return nil
	}
}

// runSync contains the main sync logic
func runSync(region *string, name *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Info("Syncing Google Network resources into Ctrlplane", "region", *region)

		ctx := context.Background()

		apiURL := viper.GetString("url")
		apiKey := viper.GetString("api-key")
		workspaceId := viper.GetString("workspace")

		// Initialize compute client
		ec2Client, err := initComputeClient(ctx, *region)
		if err != nil {
			return err
		}

		// List and process networks
		networkResources, err := processNetworks(ctx, ec2Client, *region)
		if err != nil {
			return err
		}

		// List and process subnets
		subnetResources, err := processSubnets(ctx, ec2Client, *region)
		if err != nil {
			return err
		}

		// Combine all resources
		resources := append(networkResources, subnetResources...)
		// Upsert resources to Ctrlplane
		return upsertToCtrlplane(ctx, resources, region, name)

	}
}

// initComputeClient creates a new Compute Engine client
func initComputeClient(ctx context.Context, region string) (*ec2.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	log.Info("AWS credentials loaded successfully",
		"provider", credentials.Source,
		"region", region,
		"access_key_id", credentials.AccessKeyID[:4]+"****",
		"expiration", credentials.Expires,
		"type", credentials.Source,
		"profile", os.Getenv("AWS_PROFILE"),
	)

	// Create EC2 client with retry options
	ec2Client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.RetryMaxAttempts = 3
		o.RetryMode = aws.RetryModeStandard
	})

	return ec2Client, nil
}

// processNetworks lists and processes all VPCs and subnets
func processNetworks(ctx context.Context, ec2Client *ec2.Client, project string) ([]api.AgentResource, error) {
	var nextToken *string
	vpcs := make([]types.Vpc, 0)

	for {
		output, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list VPCs: %w", err)
		}

		vpcs = append(vpcs, output.Vpcs...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	log.Info("Found vpcOutput", "count", len(vpcs))

	resources := []api.AgentResource{}
	for _, vpc := range vpcs {
		subnets, err := getSubnetsForVpc(ctx, ec2Client, *vpc.VpcId)
		if err != nil {
			log.Error("Failed to get subnets for VPC", "vpcId", *vpc.VpcId)
			continue
		}
		subnetResources := processSubnets(ctx, ec2Client, *vpc.VpcId)
		resource, err := processNetwork(vpc, project)
		if err != nil {
			log.Error("Failed to process vpc", "name", vpc.Name, "error", err)
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// processNetwork handles processing of a single VPC network
func processNetwork(vpc *types.Vpc, region string) (api.AgentResource, error) {
	metadata := initNetworkMetadata(vpc, region, subnetCount)

	// Build console URL
	consoleUrl := fmt.Sprintf(
		"https://%s.console.aws.amazon.com/vpcconsole/home?region=%s#VpcDetails:VpcId=%s",
		region, *vpc.VpcId, *vpc.VpcId)
	metadata["ctrlplane/links"] = fmt.Sprintf("{ \"AWS Console\": \"%s\" }", consoleUrl)

	// Determine subnet mode
	subnetMode := "CUSTOM"
	if vpc.AutoCreateSubnetworks {
		subnetMode = "AUTO"
	}

	// Create peering info for metadata
	if vpc.Peerings != nil {
		for i, peering := range vpc.Peerings {
			metadata[fmt.Sprintf("vpc/peering/%d/name", i)] = peering.Name
			metadata[fmt.Sprintf("vpc/peering/%d/vpc", i)] = getResourceName(peering.Network)
			metadata[fmt.Sprintf("vpc/peering/%d/state", i)] = peering.State
			metadata[fmt.Sprintf("vpc/peering/%d/auto-create-routes", i)] = strconv.FormatBool(peering.AutoCreateRoutes)
		}
		metadata["vpc/peering-count"] = strconv.Itoa(len(vpc.Peerings))
	}

	return api.AgentResource{
		Version:    "ctrlplane.dev/vpc/v1",
		Kind:       "GoogleNetwork",
		Name:       vpc.Name,
		Identifier: vpc.SelfLink,
		Config: map[string]any{
			// Common cross-provider options
			"name": vpc.Name,
			"type": "vpc",
			"id":   strconv.FormatUint(vpc.Id, 10),
			"mtu":  vpc.Mtu,

			// Provider-specific implementation details
			"googleNetwork": map[string]any{
				"project":           project,
				"selfLink":          vpc.SelfLink,
				"subnetMode":        subnetMode,
				"autoCreateSubnets": vpc.AutoCreateSubnetworks,
				"subnetCount":       subnetCount,
				"routingMode":       vpc.RoutingConfig.RoutingMode,
			},
		},
		Metadata: metadata,
	}, nil
}

// initNetworkMetadata initializes the base metadata for a network
func initNetworkMetadata(vpc *types.Vpc, project string, subnetCount int) map[string]string {
	var vpcName = vpc.VpcId // default to VPC ID

	metadata := map[string]string{
		"vpc/type":         "vpc",
		"vpc/name":         *vpc.VpcId,
		"vpc/subnet-mode":  subnetMode,
		"vpc/subnet-count": strconv.Itoa(subnetCount),
		"vpc/id":           strconv.FormatUint(vpc.Id, 10),
		"vpc/mtu":          strconv.FormatInt(vpc.Mtu, 10),

		"google/self-link":     vpc.SelfLink,
		"google/project":       project,
		"google/resource-type": "compute.googleapis.com/Network",
		"google/console-url":   consoleUrl,
		"google/id":            strconv.FormatUint(vpc.Id, 10),
	}

	// Add creation timestamp
	if vpc.CreationTimestamp != "" {
		creationTime, err := time.Parse(time.RFC3339, vpc.CreationTimestamp)
		if err == nil {
			metadata["vpc/created"] = creationTime.Format(time.RFC3339)
		} else {
			metadata["vpc/created"] = vpc.CreationTimestamp
		}
	}

	// Add routing configuration
	if vpc.RoutingConfig != nil && vpc.RoutingConfig.RoutingMode != "" {
		metadata["vpc/routing-mode"] = vpc.RoutingConfig.RoutingMode
	}

	return metadata
}

// getSubnetsForVpc retrieves subnets as AWS SDK objects
// these objects are processed differently for VPC and subnet resources
func getRawAwsSubnets(ctx context.Context, ec2Client *ec2.Client, region string) ([]types.Subnet, error) {
	var subnets []types.Subnet
	var nextToken *string

	for {
		subnetInput := &ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("region"),
					Values: []string{region},
				},
			},
			NextToken: nextToken,
		}

		subnetsOutput, err := ec2Client.DescribeSubnets(ctx, subnetInput)
		if err != nil {
			return nil, fmt.Errorf("failed to list subnets at region %s: %w", region, err)
		}

		subnets = append(subnets, subnetsOutput.Subnets...)
		if subnetsOutput.NextToken == nil {
			break
		}
		nextToken = subnetsOutput.NextToken
	}

	return subnets, nil
}

// processSubnets lists and processes all subnetworks
func processSubnets(_ context.Context, subnets []types.Subnet, region string) ([]api.AgentResource, error) {
	resources := []api.AgentResource{}
	subnetCount := 0

	// Process subnets from all regions
	for subnet := range subnets {
		resource, err := processSubnet(subnet, region)
		if err != nil {
			log.Error("Failed to process subnet", "name", subnet.Name, "error", err)
			continue
		}
		resources = append(resources, resource)
		subnetCount++
	}

	log.Info("Processed subnets", "count", subnetCount)
	return resources, nil
}

// processSubnet handles processing of a single subnet
func processSubnet(subnet types.Subnet, region string) (api.AgentResource, error) {
	metadata := initSubnetMetadata(subnet, region)

	// Build console URL
	consoleUrl := fmt.Sprintf("https://console.cloud.google.com/networking/subnetworks/details/%s/%s?project=%s",
		region, subnet.Name, project)
	metadata["ctrlplane/links"] = fmt.Sprintf("{ \"Google Cloud Console\": \"%s\" }", consoleUrl)

	// Extract network name from self link
	networkName := getResourceName(subnet.Network)

	return api.AgentResource{
		Version:    "ctrlplane.dev/network/subnet/v1",
		Kind:       "GoogleSubnet",
		Name:       subnet.Name,
		Identifier: subnet.SelfLink,
		Config: map[string]any{
			// Common cross-provider options
			"name":        subnet.Name,
			"provider":    "google",
			"type":        "subnet",
			"cidr":        subnet.IpCidrRange,
			"region":      region,
			"id":          strconv.FormatUint(subnet.Id, 10),
			"gateway":     subnet.GatewayAddress,
			"networkName": networkName,

			// Provider-specific implementation details
			"googleSubnet": map[string]any{
				"project":               project,
				"purpose":               subnet.Purpose,
				"role":                  subnet.Role,
				"privateIpGoogleAccess": subnet.PrivateIpGoogleAccess,
				"network":               subnet.Network,
				"stackType":             subnet.StackType,
				"ipv6AccessType":        subnet.Ipv6AccessType,
				"enableFlowLogs":        subnet.EnableFlowLogs,
				"secondaryIpRanges":     subnet.SecondaryIpRanges,
			},
		},
		Metadata: metadata,
	}, nil
}

// initSubnetMetadata initializes the base metadata for a subnet
func initSubnetMetadata(subnet *compute.Subnetwork, project string, region string) map[string]string {
	consoleUrl := fmt.Sprintf("https://console.cloud.google.com/networking/subnetworks/details/%s/%s?project=%s",
		region, subnet.Name, project)

	// Extract network name from self link
	networkName := getResourceName(subnet.Network)

	metadata := map[string]string{
		"network/type":           "subnet",
		"network/name":           subnet.Name,
		"network/vpc":            networkName,
		"network/region":         region,
		"network/cidr":           subnet.IpCidrRange,
		"network/gateway":        subnet.GatewayAddress,
		"network/private-access": strconv.FormatBool(subnet.PrivateIpGoogleAccess),

		"google/project":       project,
		"google/resource-type": "compute.googleapis.com/Subnetwork",
		"google/console-url":   consoleUrl,
		"google/region":        region,
		"google/id":            strconv.FormatUint(subnet.Id, 10),
	}

	// Add creation timestamp
	if subnet.CreationTimestamp != "" {
		creationTime, err := time.Parse(time.RFC3339, subnet.CreationTimestamp)
		if err == nil {
			metadata["network/created"] = creationTime.Format(time.RFC3339)
		} else {
			metadata["network/created"] = subnet.CreationTimestamp
		}
	}

	// Add purpose and role if set
	if subnet.Purpose != "" {
		metadata["network/purpose"] = subnet.Purpose
		if subnet.Role != "" {
			metadata["network/role"] = subnet.Role
		}
	}

	// Add secondary IP ranges if present
	if subnet.SecondaryIpRanges != nil {
		for i, secondaryRange := range subnet.SecondaryIpRanges {
			metadata[fmt.Sprintf("network/secondary-range/%d/name", i)] = secondaryRange.RangeName
			metadata[fmt.Sprintf("network/secondary-range/%d/cidr", i)] = secondaryRange.IpCidrRange
		}
		metadata["network/secondary-range-count"] = strconv.Itoa(len(subnet.SecondaryIpRanges))
	}

	// Add IP version details
	if subnet.StackType != "" {
		metadata["network/stack-type"] = subnet.StackType
	}
	if subnet.Ipv6AccessType != "" {
		metadata["network/ipv6-access-type"] = subnet.Ipv6AccessType
	}
	if subnet.InternalIpv6Prefix != "" {
		metadata["network/internal-ipv6-prefix"] = subnet.InternalIpv6Prefix
	}
	if subnet.ExternalIpv6Prefix != "" {
		metadata["network/external-ipv6-prefix"] = subnet.ExternalIpv6Prefix
	}

	// Add flow logs status
	if subnet.EnableFlowLogs {
		metadata["network/flow-logs"] = "enabled"
	} else {
		metadata["network/flow-logs"] = "disabled"
	}

	return metadata
}

// getRegionFromURL extracts the region name from a URL like "regions/us-central1"
func getRegionFromURL(regionURL string) string {
	parts := strings.Split(regionURL, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return regionURL
}

// getResourceName extracts the resource name from its full path
func getResourceName(fullPath string) string {
	if fullPath == "" {
		return ""
	}
	parts := strings.Split(fullPath, "/")
	return parts[len(parts)-1]
}

// upsertToCtrlplane handles upserting resources to Ctrlplane
func upsertToCtrlplane(ctx context.Context, resources []api.AgentResource, project, name *string) error {
	if *name == "" {
		*name = fmt.Sprintf("google-networks-project-%s", *project)
	}

	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	rp, err := api.NewResourceProvider(ctrlplaneClient, workspaceId, *name)
	if err != nil {
		return fmt.Errorf("failed to create resource provider: %w", err)
	}

	upsertResp, err := rp.UpsertResource(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to upsert resources: %w", err)
	}

	log.Info("Response from upserting resources", "status", upsertResp.Status)
	return nil
}
