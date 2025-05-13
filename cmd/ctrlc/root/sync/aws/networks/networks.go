package networks

import (
	"context"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/aws/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strconv"
	"sync"
)

// NewSyncNetworksCmd creates a new cobra command for syncing AWS Networks
func NewSyncNetworksCmd() *cobra.Command {
	var name string
	var regions []string

	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Sync AWS VPC networks and subnets into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure AWS credentials are configured via environment variables or application default credentials
			
			# Sync all VPC networks and subnets from a region
			$ ctrlc sync aws networks --region my-region
		`),
		RunE: runSync(&regions, &name),
	}

	// Add command flags
	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringSliceVarP(&regions, "region", "r", []string{}, "AWS Region(s)")

	return cmd
}

// runSync contains the main sync logic
func runSync(regions *[]string, name *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Get the regions to sync from using common package
		regionsToSync, err := common.GetRegions(ctx, *regions)
		if err != nil {
			return err
		}

		allResources := make([]api.CreateResource, 0)

		var mu sync.Mutex
		var wg sync.WaitGroup
		var syncErrors []error

		for _, region := range regionsToSync {
			wg.Add(1)
			go func(regionName string) {
				defer wg.Done()
				log.Info("Syncing AWS Network resources into Ctrlplane", "region", regionName)

				ctx := context.Background()

				//apiURL := viper.GetString("url")
				//apiKey := viper.GetString("api-key")
				//workspaceId := viper.GetString("workspace")

				// Initialize compute client
				ec2Client, cfg, err := initComputeClient(ctx, regionName)
				if err != nil {
					log.Error("Failed to initialize EC2 client", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				accountId, err := common.GetAccountID(ctx, cfg)
				if err != nil {
					log.Error("Failed get accountId", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				awsSubnets, err := getAwsSubnets(ctx, ec2Client, regionName)
				if err != nil {
					log.Error("Failed to get subnets", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				// List and process networks
				vpcResources, err := processNetworks(ctx, ec2Client, awsSubnets, regionName, accountId)
				if err != nil {
					log.Error("Failed to process VPCs", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				// List and process subnets
				subnetResources, err := processSubnets(ctx, awsSubnets, regionName)
				if err != nil {
					log.Error("Failed to process subnets", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				if len(vpcResources) > 0 {
					mu.Lock()
					allResources = append(allResources, vpcResources...)
					mu.Unlock()
				}
				if len(subnetResources) > 0 {
					mu.Lock()
					allResources = append(allResources, subnetResources...)
					mu.Unlock()
				}
			}(region)
		}
		wg.Wait()

		common.EnsureProviderDetails(ctx, "aws-networks", regionsToSync, name)

		// Upsert resources to Ctrlplane
		return upsertToCtrlplane(ctx, allResources, name)
	}
}

// initComputeClient creates a new Compute Engine client
func initComputeClient(ctx context.Context, region string) (*ec2.Client, aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, cfg, fmt.Errorf("failed to load AWS config: %w", err)
	}

	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, cfg, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
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

	return ec2Client, cfg, nil
}

// processNetworks lists and processes all VPCs and subnets
func processNetworks(
	ctx context.Context, ec2Client *ec2.Client, awsSubnets []types.Subnet, region string, accountId string,
) ([]api.CreateResource, error) {
	var nextToken *string
	vpcs := make([]types.Vpc, 0)
	subnetsByVpc := make(map[string][]types.Subnet)
	var awsVpcSubnets []types.Subnet
	var exists bool

	for _, sn := range awsSubnets {
		vpcId := *sn.VpcId
		if _, exists = subnetsByVpc[vpcId]; !exists {
			subnetsByVpc[vpcId] = []types.Subnet{}
		}
		subnetsByVpc[vpcId] = append(subnetsByVpc[vpcId], sn)
	}

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

	resources := make([]api.CreateResource, 0)
	for _, vpc := range vpcs {
		if awsVpcSubnets, exists = subnetsByVpc[*vpc.VpcId]; !exists {
			awsVpcSubnets = []types.Subnet{}
		}
		resource, err := processNetwork(vpc, awsVpcSubnets, region, accountId)
		if err != nil {
			log.Error("Failed to process vpc", "vpcId", vpc.VpcId, "error", err)
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// processNetwork handles processing of a single VPC network
func processNetwork(
	vpc types.Vpc, subnets []types.Subnet, region string, accountId string,
) (api.CreateResource, error) {
	metadata := initNetworkMetadata(vpc, region, len(subnets))
	vpcName := getVpcName(vpc)

	// Build console URL
	consoleUrl := getVpcConsoleUrl(vpc, region)
	metadata["ctrlplane/links"] = fmt.Sprintf("{ \"AWS Console\": \"%s\" }", consoleUrl)

	return api.CreateResource{
		Version:    "ctrlplane.dev/network/v1",
		Kind:       "AmazonNetwork",
		Name:       vpcName,
		Identifier: *vpc.VpcId,
		Config: map[string]any{
			// Common cross-provider options
			"name": vpcName,
			"type": "vpc",
			"id":   *vpc.VpcId,

			// Provider-specific implementation details
			"awsVpc": map[string]any{
				"accountId":   accountId,
				"region":      region,
				"state":       string(vpc.State),
				"subnetCount": len(subnets),
			},
		},
		Metadata: metadata,
	}, nil
}

// initNetworkMetadata initializes the base metadata for a network
func initNetworkMetadata(vpc types.Vpc, region string, subnetCount int) map[string]string {
	var vpcName = getVpcName(vpc)
	var consoleUrl = getVpcConsoleUrl(vpc, region)

	metadata := map[string]string{
		"vpc/type":         "vpc",
		"vpc/name":         vpcName,
		"vpc/subnet-count": strconv.Itoa(subnetCount),
		"vpc/id":           *vpc.VpcId,
		"vpc/tenancy":      string(vpc.InstanceTenancy),

		"aws/region":        region,
		"aws/resource-type": "vpc",
		"aws/status":        string(vpc.State),
		"aws/console-url":   consoleUrl,
		"aws/id":            *vpc.VpcId,
	}

	// Tags
	if vpc.Tags != nil {
		for _, tag := range vpc.Tags {
			metadata[fmt.Sprintf("tags/%s", *tag.Key)] = *tag.Value
		}
	}

	return metadata
}

// getSubnetsForVpc retrieves subnets as AWS SDK objects
// these objects are processed differently for VPC and subnet resources
func getAwsSubnets(ctx context.Context, ec2Client *ec2.Client, region string) ([]types.Subnet, error) {
	var subnets []types.Subnet
	var nextToken *string

	for {
		subnetInput := &ec2.DescribeSubnetsInput{
			Filters:   []types.Filter{},
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
func processSubnets(_ context.Context, subnets []types.Subnet, region string) ([]api.CreateResource, error) {
	resources := make([]api.CreateResource, 0)
	subnetCount := 0

	// Process subnets from all regions
	for _, subnet := range subnets {
		resource, err := processSubnet(subnet, region)
		if err != nil {
			log.Error("Failed to process subnet", "subnetId", subnet.SubnetId, "error", err)
			continue
		}
		resources = append(resources, resource)
		subnetCount++
	}

	log.Info("Processed subnets", "count", subnetCount)
	return resources, nil
}

// processSubnet handles processing of a single subnet
func processSubnet(subnet types.Subnet, region string) (api.CreateResource, error) {
	metadata := initSubnetMetadata(subnet, region)
	subnetName := getSubnetName(subnet)
	consoleUrl := getSubnetConsoleUrl(subnet, region)
	metadata["ctrlplane/links"] = fmt.Sprintf("{ \"AWS Console\": \"%s\" }", consoleUrl)

	return api.CreateResource{
		Version:    "ctrlplane.dev/network/subnet/v1",
		Kind:       "AmazonSubnet",
		Name:       subnetName,
		Identifier: *subnet.SubnetArn,
		Config: map[string]any{
			// Common cross-provider options
			"name":     subnetName,
			"provider": "aws",
			"type":     "subnet",
			"cidr":     subnet.CidrBlock,
			"region":   region,
			"id":       *subnet.SubnetId,
			"vpcId":    *subnet.VpcId,
		},
		Metadata: metadata,
	}, nil
}

// initSubnetMetadata initializes the base metadata for a subnet
func initSubnetMetadata(subnet types.Subnet, region string) map[string]string {
	consoleUrl := getSubnetConsoleUrl(subnet, region)
	subnetName := getSubnetName(subnet)

	metadata := map[string]string{
		"network/type":                "subnet",
		"network/name":                subnetName,
		"network/vpc":                 *subnet.VpcId,
		"network/region":              region,
		"network/cidr":                *subnet.CidrBlock,
		"network/block-public-access": string(subnet.BlockPublicAccessStates.InternetGatewayBlockMode),

		"aws/resource-type": "aws/Subnet",
		"aws/console-url":   consoleUrl,
		"aws/region":        region,
		"aws/id":            *subnet.SubnetId,
	}

	// Tags
	if subnet.Tags != nil {
		for _, tag := range subnet.Tags {
			metadata[fmt.Sprintf("tags/%s", *tag.Key)] = *tag.Value
		}
	}

	return metadata
}

func getVpcConsoleUrl(vpc types.Vpc, region string) string {
	return fmt.Sprintf(
		"https://%s.console.aws.amazon.com/vpcconsole/home?region=%s#VpcDetails:VpcId=%s",
		region, region, *vpc.VpcId)
}

func getVpcName(vpc types.Vpc) string {
	vpcName := *vpc.VpcId
	for _, tag := range vpc.Tags {
		if *tag.Key == "Name" {
			vpcName = *tag.Value
			break
		}
	}
	return vpcName
}

func getSubnetConsoleUrl(subnet types.Subnet, region string) string {
	return fmt.Sprintf(
		"https://%s.console.aws.amazon.com/vpcconsole/home?region=%s#SubnetDetails:subnetId=%s",
		region, region, *subnet.VpcId)
}

func getSubnetName(subnet types.Subnet) string {
	subnetName := *subnet.VpcId
	for _, tag := range subnet.Tags {
		if *tag.Key == "Name" {
			subnetName = *tag.Value
			break
		}
	}
	return subnetName
}

// upsertToCtrlplane handles upserting resources to Ctrlplane
func upsertToCtrlplane(ctx context.Context, resources []api.CreateResource, name *string) error {
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
