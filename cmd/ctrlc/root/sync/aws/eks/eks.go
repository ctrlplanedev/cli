package eks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/aws/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/kinds"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewSyncEKSCmd creates a new cobra command for syncing EKS clusters
func NewSyncEKSCmd() *cobra.Command {
	var regions []string
	var name string

	cmd := &cobra.Command{
		Use:   "eks",
		Short: "Sync Amazon Elastic Kubernetes Service clusters into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure AWS credentials are configured via environment variables or AWS CLI
			
			# Sync all EKS clusters from a region
			$ ctrlc sync aws eks --region us-west-2
			
			# Sync all EKS clusters from multiple regions
			$ ctrlc sync aws eks --region us-west-2 --region us-east-1
			
			# Sync all EKS clusters from all regions
			$ ctrlc sync aws eks
		`),
		RunE: runSync(&regions, &name),
	}

	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringSliceVarP(&regions, "region", "r", []string{}, "AWS Region(s)")

	return cmd
}

func runSync(regions *[]string, name *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Get the regions to sync from using common package
		regionsToSync, err := common.GetRegions(ctx, *regions)
		if err != nil {
			return err
		}

		log.Info("Syncing EKS clusters", "regions", regionsToSync)

		// Process each region
		var allResources []api.CreateResource
		var mu sync.Mutex
		var wg sync.WaitGroup
		var syncErrors []error

		for _, r := range regionsToSync {
			wg.Add(1)
			go func(regionName string) {
				defer wg.Done()

				// Initialize AWS client for this region
				eksClient, cfg, err := initEKSClient(ctx, regionName)
				if err != nil {
					log.Error("Failed to initialize EKS client", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				// List and process clusters for this region
				resources, err := processClusters(ctx, eksClient, regionName, cfg)
				if err != nil {
					log.Error("Failed to process clusters", "region", regionName, "error", err)
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Errorf("region %s: %w", regionName, err))
					mu.Unlock()
					return
				}

				if len(resources) > 0 {
					mu.Lock()
					allResources = append(allResources, resources...)
					mu.Unlock()
				}
			}(r)
		}

		wg.Wait()

		if len(syncErrors) > 0 {
			log.Warn("Some regions failed to sync", "errors", len(syncErrors))
			// Continue with the regions that succeeded
		}

		if len(allResources) == 0 {
			log.Info("No EKS clusters found in the specified regions")
			return nil
		}

		common.EnsureProviderDetails(ctx, "aws-eks", regionsToSync, name)

		// Upsert resources to Ctrlplane
		return upsertToCtrlplane(ctx, allResources, name)
	}
}

func initEKSClient(ctx context.Context, region string) (*eks.Client, aws.Config, error) {
	// Use common package to initialize AWS config
	cfg, err := common.InitAWSConfig(ctx, region)
	if err != nil {
		return nil, aws.Config{}, err
	}

	return eks.NewFromConfig(cfg), cfg, nil
}

func processClusters(ctx context.Context, eksClient *eks.Client, region string, cfg aws.Config) ([]api.CreateResource, error) {
	var resources []api.CreateResource
	var nextToken *string

	accountID, err := common.GetAccountID(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS account ID: %w", err)
	}

	for {
		resp, err := eksClient.ListClusters(ctx, &eks.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
		}

		for _, clusterName := range resp.Clusters {
			cluster, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: &clusterName,
			})
			if err != nil {
				log.Error("Failed to describe cluster", "name", clusterName, "error", err)
				continue
			}

			resource, err := processCluster(ctx, cluster.Cluster, region, accountID)
			if err != nil {
				log.Error("Failed to process EKS cluster", "name", clusterName, "error", err)
				continue
			}
			resources = append(resources, resource)
		}

		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}

	log.Info("Found EKS clusters", "region", region, "count", len(resources))
	return resources, nil
}

func processCluster(_ context.Context, cluster *types.Cluster, region string, accountID string) (api.CreateResource, error) {
	metadata := initClusterMetadata(cluster, region)

	metadata["aws/account"] = accountID

	consoleUrl := fmt.Sprintf("https://%s.console.aws.amazon.com/eks/home?region=%s#/clusters/%s",
		region, region, *cluster.Name)
	metadata["ctrlplane/links"] = fmt.Sprintf("{ \"AWS Console\": \"%s\" }", consoleUrl)

	return api.CreateResource{
		Version:    "ctrlplane.dev/kubernetes/cluster/v1",
		Kind:       "AmazonElasticKubernetesService",
		Name:       *cluster.Name,
		Identifier: *cluster.Arn,
		Config: map[string]any{
			"name":    *cluster.Name,
			"version": *cluster.Version,
			"server": map[string]any{
				"endpoint":                 *cluster.Endpoint,
				"certificateAuthorityData": *cluster.CertificateAuthority.Data,
			},

			// Provider-specific implementation details
			"amazonElasticKubernetesService": map[string]any{
				"accountId":       accountID,
				"arn":             *cluster.Arn,
				"region":          region,
				"status":          string(cluster.Status),
				"platformVersion": *cluster.PlatformVersion,
				"vpc":             *cluster.ResourcesVpcConfig.VpcId,
			},
		},
		Metadata: metadata,
	}, nil
}

func initClusterMetadata(cluster *types.Cluster, region string) map[string]string {
	version, err := semver.NewVersion(*cluster.Version)
	if err != nil {
		log.Error("Failed to parse Kubernetes version", "version", *cluster.Version, "error", err)
	}

	normalizedStatus := "unknown"
	switch cluster.Status {
	case types.ClusterStatusActive:
		normalizedStatus = "running"
	case types.ClusterStatusUpdating:
		normalizedStatus = "updating"
	case types.ClusterStatusCreating:
		normalizedStatus = "creating"
	case types.ClusterStatusDeleting:
		normalizedStatus = "deleting"
	case types.ClusterStatusFailed:
		normalizedStatus = "failed"
	}

	metadata := map[string]string{
		"network/type": "vpc",
		"network/name": *cluster.ResourcesVpcConfig.VpcId,

		kinds.K8SMetadataType:              "eks",
		kinds.K8SMetadataName:              *cluster.Name,
		kinds.K8SMetadataVersion:           version.String(),
		kinds.K8SMetadataVersionMajor:      strconv.FormatUint(uint64(version.Major()), 10),
		kinds.K8SMetadataVersionMinor:      strconv.FormatUint(uint64(version.Minor()), 10),
		kinds.K8SMetadataVersionPatch:      strconv.FormatUint(uint64(version.Patch()), 10),
		kinds.K8SMetadataVersionPrerelease: version.Prerelease(),
		kinds.K8SMetadataStatus:            normalizedStatus,

		"aws/region":           region,
		"aws/resource-type":    "eks:cluster",
		"aws/status":           string(cluster.Status),
		"aws/platform-version": *cluster.PlatformVersion,
		"aws/arn":              *cluster.Arn,
	}

	if cluster.CreatedAt != nil {
		metadata[kinds.K8SMetadataCreated] = cluster.CreatedAt.Format(time.RFC3339)
	}

	// Network configuration
	if cluster.ResourcesVpcConfig != nil {
		metadata["network/vpc"] = *cluster.ResourcesVpcConfig.VpcId
		if len(cluster.ResourcesVpcConfig.SubnetIds) > 0 {
			metadata["network/subnet-count"] = strconv.Itoa(len(cluster.ResourcesVpcConfig.SubnetIds))
			metadata["network/subnets"] = strings.Join(cluster.ResourcesVpcConfig.SubnetIds, ",")
		}
	}

	// Logging configuration
	if cluster.Logging != nil && cluster.Logging.ClusterLogging != nil {
		for _, logging := range cluster.Logging.ClusterLogging {
			if logging.Enabled != nil && *logging.Enabled {
				for _, logType := range logging.Types {
					metadata[fmt.Sprintf("logging/%s", strings.ToLower(string(logType)))] = "enabled"
				}
			}
		}
	}

	for key, value := range cluster.Tags {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}

	return metadata
}

var relationshipRules = []api.CreateResourceRelationshipRule{
	{
		Reference:      "network",
		Name:           "AWS Cluster Network",
		DependencyType: api.ProvisionedIn,

		SourceVersion: "ctrlplane.dev/kubernetes/cluster/v1",
		SourceKind:    "AmazonElasticKubernetesService",

		TargetVersion: "ctrlplane.dev/network/v1",
		TargetKind:    "AmazonNetwork",

		MetadataKeysMatches: &[]string{"aws/region", "network/name"},
	},
}

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

	err = rp.AddResourceRelationshipRule(ctx, relationshipRules)
	if err != nil {
		log.Error("Failed to add resource relationship rule", "name", *name, "error", err)
	}

	upsertResp, err := rp.UpsertResource(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to upsert resources: %w", err)
	}

	log.Info("Response from upserting resources", "status", upsertResp.Status)
	return nil
}
