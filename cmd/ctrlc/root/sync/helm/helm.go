package helm

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/resourceprovider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/rest"
)

// clusterConfig holds the resolved cluster information needed for syncing
type clusterConfig struct {
	name       string // Human-readable cluster name (e.g., "production-us-east")
	identifier string // Optional Ctrlplane resource identifier for the cluster
}

// helmRelease converts a Helm release into a Ctrlplane resource.
// The clusterName is used to:
// - Create unique identifiers across clusters (cluster/namespace/release)
// - Tag resources with their source cluster for filtering and relationships
func helmReleaseToResource(release *release.Release, clusterName string) api.ResourceProviderResource {
	metadata := buildHelmMetadata(release, clusterName)
	config := buildHelmConfig(release)
	identifier := fmt.Sprintf("%s/%s/%s", clusterName, release.Namespace, release.Name)

	return api.ResourceProviderResource{
		Version:    "ctrlplane.dev/helm/release/v1",
		Kind:       "HelmRelease",
		Name:       identifier, // Use cluster/namespace/release for uniqueness
		Identifier: identifier,
		Config:     config,
		Metadata:   metadata,
	}
}

// buildHelmMetadata creates the metadata map for a Helm release resource
func buildHelmMetadata(release *release.Release, clusterName string) map[string]string {
	metadata := map[string]string{}

	// Copy user-defined labels from the Helm release
	for key, value := range release.Labels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}

	// Add standard Helm metadata for filtering and querying
	// Create an insightful version string that combines chart version, app version, revision, and status
	metadata["ctrlplane/version"] = fmt.Sprintf(
		"chart:%s app:%s rev:%d status:%s",
		release.Chart.Metadata.Version,
		release.Chart.Metadata.AppVersion,
		release.Version,
		release.Info.Status.String(),
	)
	metadata["helm/name"] = release.Name
	metadata["helm/namespace"] = release.Namespace
	metadata["helm/chart-name"] = release.Chart.Metadata.Name
	metadata["helm/chart-version"] = release.Chart.Metadata.Version
	metadata["helm/app-version"] = release.Chart.Metadata.AppVersion
	metadata["helm/status"] = release.Info.Status.String()
	metadata["helm/revision"] = fmt.Sprintf("%d", release.Version)

	// Add Kubernetes context - this links the release to its cluster
	metadata["kubernetes/name"] = clusterName
	metadata["kubernetes/namespace"] = release.Namespace

	return metadata
}

// buildHelmConfig creates the config object for a Helm release resource
func buildHelmConfig(release *release.Release) map[string]any {
	return map[string]any{
		"name":         release.Name,
		"namespace":    release.Namespace,
		"chart":        release.Chart.Metadata.Name,
		"chartVersion": release.Chart.Metadata.Version,
		"appVersion":   release.Chart.Metadata.AppVersion,
		"status":       release.Info.Status.String(),
		"revision":     release.Version,
	}
}

func NewSyncHelmCmd() *cobra.Command {
	var providerName string
	var clusterIdentifier string
	var clusterName string
	var namespace string

	cmd := &cobra.Command{
		Use:   "helm",
		Short: "Sync Helm resources into Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync helm --cluster-identifier 1234567890 --cluster-name my-cluster
			$ ctrlc sync helm --namespace my-namespace
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Step 1: Initialize Ctrlplane API client
			ctrlplaneClient, workspaceId, err := initializeCtrlplaneClient()
			if err != nil {
				return err
			}

			// Step 2: Resolve cluster information (name, identifier, kubeconfig)
			cluster, kubeConfig, err := resolveClusterConfig(ctx, ctrlplaneClient, workspaceId, clusterIdentifier, clusterName)
			if err != nil {
				return err
			}

			log.Info("Syncing Helm releases", "cluster", cluster.name, "namespace", namespaceOrAll(namespace))

			// Step 3: Fetch Helm releases from the Kubernetes cluster
			releases, err := fetchHelmReleases(kubeConfig, namespace)
			if err != nil {
				return err
			}

			log.Info("Found Helm releases", "count", len(releases))

			// Step 4: Convert Helm releases to Ctrlplane resources
			resources := convertReleasesToResources(releases, cluster.name)

			// Step 5: Optionally inherit metadata from parent cluster resource
			if cluster.identifier != "" {
				inheritClusterMetadata(ctx, ctrlplaneClient, workspaceId, cluster.identifier, resources)
			}

			// Step 6: Upsert resources to Ctrlplane
			return upsertResourcesToCtrlplane(ctx, ctrlplaneClient, workspaceId, resources, cluster.name, providerName)
		},
	}

	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&clusterIdentifier, "cluster-identifier", "c", "", "The identifier of the parent cluster in ctrlplane (if not provided, will use the CLUSTER_IDENTIFIER environment variable)")
	cmd.Flags().StringVarP(&clusterName, "cluster-name", "n", "", "The name of the cluster")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace to sync Helm releases from (if not provided, syncs from all namespaces)")

	return cmd
}

// --- Helper Functions ---
// These functions break down the sync operation into clear, testable steps

// initializeCtrlplaneClient creates an authenticated API client
func initializeCtrlplaneClient() (*api.ClientWithResponses, string, error) {
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API client: %w", err)
	}

	return ctrlplaneClient, workspaceId, nil
}

// resolveClusterConfig determines the cluster name and identifier from multiple sources:
// 1. Explicit --cluster-identifier flag (takes precedence)
// 2. Environment variable CLUSTER_IDENTIFIER
// 3. Kubeconfig current-context (fallback)
//
// If a cluster identifier is provided, we try to fetch the cluster resource from Ctrlplane
// to get the canonical cluster name. Otherwise, we use the name from kubeconfig.
func resolveClusterConfig(ctx context.Context, client *api.ClientWithResponses, workspaceId, flagIdentifier, flagName string) (clusterConfig, *rest.Config, error) {
	// Get kubeconfig first since we need it regardless
	kubeConfig, kubeconfigClusterName, err := getKubeConfig()
	if err != nil {
		return clusterConfig{}, nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Resolve cluster identifier (flag takes precedence over environment)
	clusterIdentifier := flagIdentifier
	if clusterIdentifier == "" {
		clusterIdentifier = viper.GetString("cluster-identifier")
	}

	// Determine final cluster name with priority:
	// 1. Name from Ctrlplane API (if identifier provided and resource exists)
	// 2. Explicit --cluster-name flag
	// 3. Name from kubeconfig
	resolvedClusterName := flagName
	if clusterIdentifier != "" {
		// Try to get the canonical cluster name from Ctrlplane
		clusterResource, err := client.GetResourceByIdentifierWithResponse(ctx, workspaceId, clusterIdentifier)
		if err == nil && clusterResource.JSON200 != nil {
			resolvedClusterName = clusterResource.JSON200.Name
			log.Info("Using cluster name from Ctrlplane", "name", resolvedClusterName)
		}
	}

	// Fall back to kubeconfig cluster name if still not set
	if resolvedClusterName == "" {
		resolvedClusterName = kubeconfigClusterName
		log.Info("Using cluster name from kubeconfig", "name", resolvedClusterName)
	}

	return clusterConfig{
		name:       resolvedClusterName,
		identifier: clusterIdentifier,
	}, kubeConfig, nil
}

// fetchHelmReleases queries the Kubernetes cluster for all Helm releases
func fetchHelmReleases(kubeConfig *rest.Config, namespace string) ([]*release.Release, error) {
	// Initialize Helm action configuration
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(getConfigFlags(kubeConfig, namespace), namespace, "secret", log.Debugf); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	// Configure list operation
	listClient := action.NewList(actionConfig)
	listClient.All = true                        // Include all releases (not just deployed)
	listClient.AllNamespaces = (namespace == "") // Scan all namespaces if none specified

	releases, err := listClient.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm releases: %w", err)
	}

	return releases, nil
}

// convertReleasesToResources transforms Helm releases into Ctrlplane resource format
func convertReleasesToResources(releases []*release.Release, clusterName string) []api.ResourceProviderResource {
	resources := make([]api.ResourceProviderResource, 0, len(releases))
	for _, release := range releases {
		resource := helmReleaseToResource(release, clusterName)
		resources = append(resources, resource)
	}
	return resources
}

// inheritClusterMetadata copies non-tag metadata from the parent cluster resource to all Helm releases.
// This is useful for propagating environment labels, region info, etc. from the cluster to its workloads.
func inheritClusterMetadata(ctx context.Context, client *api.ClientWithResponses, workspaceId, clusterIdentifier string, resources []api.ResourceProviderResource) {
	clusterResource, err := client.GetResourceByIdentifierWithResponse(ctx, workspaceId, clusterIdentifier)
	if err != nil || clusterResource.JSON200 == nil {
		log.Debug("Could not fetch cluster resource for metadata inheritance", "identifier", clusterIdentifier)
		return
	}

	// Copy metadata from cluster to each Helm release (skip tags to avoid conflicts)
	for i := range resources {
		for key, value := range clusterResource.JSON200.Metadata {
			// Skip user-defined tags to let each release keep its own tags
			if strings.HasPrefix(key, "tags/") {
				continue
			}
			// Only set if the release doesn't already have this metadata key
			if _, exists := resources[i].Metadata[key]; !exists {
				resources[i].Metadata[key] = value
			}
		}
		// Ensure kubernetes/name is set to the cluster's canonical name
		resources[i].Metadata["kubernetes/name"] = clusterResource.JSON200.Name
	}

	log.Debug("Inherited metadata from cluster resource", "keys", len(clusterResource.JSON200.Metadata))
}

// upsertResourcesToCtrlplane sends the resources to Ctrlplane via the resource provider API
func upsertResourcesToCtrlplane(ctx context.Context, client *api.ClientWithResponses, workspaceId string, resources []api.ResourceProviderResource, clusterName, providerName string) error {
	// Generate default provider name if not specified
	if providerName == "" {
		providerName = fmt.Sprintf("helm-cluster-%s", clusterName)
	}

	log.Info("Upserting to Ctrlplane", "provider", providerName, "resources", len(resources))

	// Create or get resource provider
	resourceProvider, err := resourceprovider.New(client, workspaceId, providerName)
	if err != nil {
		return fmt.Errorf("failed to create resource provider: %w", err)
	}

	// Upsert all resources in one batch
	resp, err := resourceProvider.UpsertResource(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to upsert resources: %w", err)
	}

	log.Info("Successfully synced resources", "status", resp.Status)
	return nil
}

// namespaceOrAll returns a human-readable string for logging
func namespaceOrAll(namespace string) string {
	if namespace == "" {
		return "all namespaces"
	}
	return namespace
}
