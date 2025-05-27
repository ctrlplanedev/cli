package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/Masterminds/semver"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/kinds"

	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/platform/kube"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	vclusterKind = "VCluster"
)

func getParentClusterResource(ctx context.Context, ctrlplaneClient *api.ClientWithResponses, workspaceId string, clusterIdentifier string) (ClusterResource, error) {
	clusterResourceResponse, err := ctrlplaneClient.GetResourceByIdentifierWithResponse(ctx, workspaceId, clusterIdentifier)
	if err != nil {
		return ClusterResource{}, fmt.Errorf("failed to get cluster resource: %w", err)
	}
	if clusterResourceResponse.StatusCode() != 200 {
		return ClusterResource{}, fmt.Errorf("failed to get cluster resource: %s", clusterResourceResponse.Status())
	}
	clusterResource := ClusterResource{
		Config:     clusterResourceResponse.JSON200.Config,
		Metadata:   clusterResourceResponse.JSON200.Metadata,
		Name:       clusterResourceResponse.JSON200.Name,
		Identifier: clusterResourceResponse.JSON200.Identifier,
		Kind:       clusterResourceResponse.JSON200.Kind,
		Version:    clusterResourceResponse.JSON200.Version,
	}

	return clusterResource, nil
}

func deepClone(src map[string]interface{}) (map[string]interface{}, error) {
	bytes, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var clone map[string]interface{}
	if err := json.Unmarshal(bytes, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func getNormalizedVclusterStatus(status find.Status) string {
	switch status {
	case find.StatusRunning:
		return "running"
	case find.StatusPaused:
		return "paused"
	case find.StatusWorkloadSleeping:
		return "sleeping"
	case find.StatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

func generateVclusterMetadata(vcluster find.VCluster, clusterMetadata api.MetadataMap) (map[string]string, error) {
	metadata := make(map[string]string)
	parsedVersion, err := semver.NewVersion(vcluster.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vcluster version: %w", err)
	}
	metadata[kinds.VClusterMetadataVersion] = vcluster.Version
	metadata[kinds.VClusterMetadataVersionMajor] = strconv.FormatInt(parsedVersion.Major(), 10)
	metadata[kinds.VClusterMetadataVersionMinor] = strconv.FormatInt(parsedVersion.Minor(), 10)
	metadata[kinds.VClusterMetadataVersionPatch] = strconv.FormatInt(parsedVersion.Patch(), 10)
	metadata[kinds.VClusterMetadataName] = vcluster.Name
	metadata[kinds.VClusterMetadataNamespace] = vcluster.Namespace
	metadata[kinds.VClusterMetadataStatus] = getNormalizedVclusterStatus(vcluster.Status)
	metadata[kinds.VClusterMetadataCreated] = vcluster.Created.Format(time.RFC3339)
	metadata[kinds.K8SMetadataFlavor] = vclusterKind

	if vcluster.Labels != nil {
		for key, value := range vcluster.Labels {
			metadata[fmt.Sprintf("tags/%s", key)] = value
		}
	}

	if vcluster.Annotations != nil {
		for key, value := range vcluster.Annotations {
			metadata[fmt.Sprintf("annotations/%s", key)] = value
		}
	}

	if clusterMetadata != nil {
		for key, value := range clusterMetadata {
			metadata[key] = value
		}
	}

	return metadata, nil
}

func generateVclusterConfig(vcluster find.VCluster, clusterName string, clusterConfig map[string]interface{}) map[string]interface{} {
	vclusterConfig := make(map[string]interface{})
	vclusterConfig["name"] = vcluster.Name
	vclusterConfig["namespace"] = vcluster.Namespace
	vclusterConfig["status"] = getNormalizedVclusterStatus(vcluster.Status)
	clusterConfig["vcluster"] = vclusterConfig

	return clusterConfig
}

type ClusterResource struct {
	Config     map[string]interface{}
	Metadata   api.MetadataMap
	Name       string
	Identifier string
	Kind       string
	Version    string
}

func getCreateResourceFromVcluster(vcluster find.VCluster, clusterResource ClusterResource) (api.CreateResource, error) {
	metadata, err := generateVclusterMetadata(vcluster, clusterResource.Metadata)
	if err != nil {
		return api.CreateResource{}, fmt.Errorf("failed to generate vcluster metadata: %w", err)
	}

	clonedParentConfig, err := deepClone(clusterResource.Config)
	if err != nil {
		return api.CreateResource{}, fmt.Errorf("failed to clone parent config: %w", err)
	}

	resource := api.CreateResource{
		Name:       fmt.Sprintf("%s/%s/%s", clusterResource.Name, vcluster.Namespace, vcluster.Name),
		Identifier: fmt.Sprintf("%s/%s/vcluster/%s", clusterResource.Identifier, vcluster.Namespace, vcluster.Name),
		Kind:       fmt.Sprintf("%s/%s", clusterResource.Kind, vclusterKind),
		Version:    "ctrlplane.dev/kubernetes/cluster/v1",
		Metadata:   metadata,
		Config:     generateVclusterConfig(vcluster, clusterResource.Name, clonedParentConfig),
	}

	return resource, nil
}

func NewSyncVclusterCmd() *cobra.Command {
	var clusterIdentifier string
	var providerName string

	cmd := &cobra.Command{
		Use:   "vcluster",
		Short: "Sync vcluster resources",
		Example: heredoc.Doc(`
			$ ctrlc sync vcluster
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspaceId := viper.GetString("workspace")

			if clusterIdentifier == "" {
				clusterIdentifier = viper.GetString("cluster-identifier")
			}

			if clusterIdentifier == "" {
				return fmt.Errorf("cluster identifier is required, please set the CTRLPLANE_CLUSTER_IDENTIFIER environment variable or use the --cluster-identifier flag")
			}

			ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			clusterResource, err := getParentClusterResource(cmd.Context(), ctrlplaneClient, workspaceId, clusterIdentifier)
			if err != nil {
				return fmt.Errorf("failed to get parent cluster resource: %w", err)
			}

			config, context, err := getKubeConfig()
			if err != nil {
				return fmt.Errorf("failed to get kube config: %w", err)
			}

			clientset, err := kube.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			namespace := metav1.NamespaceAll
			vclusters, err := find.ListOSSVClusters(cmd.Context(), clientset, context, "", namespace)
			if err != nil {
				return err
			}

			fmt.Printf("Found %d vclusters in namespace %s\n", len(vclusters), namespace)

			if providerName == "" {
				providerName = fmt.Sprintf("%s-vcluster-scanner", clusterResource.Name)
			}

			rp, err := api.NewResourceProvider(ctrlplaneClient, workspaceId, providerName)
			if err != nil {
				return fmt.Errorf("failed to create resource provider: %w", err)
			}

			resourcesToUpsert := []api.CreateResource{}
			for _, vcluster := range vclusters {
				resource, err := getCreateResourceFromVcluster(vcluster, clusterResource)
				if err != nil {
					return fmt.Errorf("failed to get create resource from vcluster: %w", err)
				}
				resourcesToUpsert = append(resourcesToUpsert, resource)
			}

			upsertResp, err := rp.UpsertResource(cmd.Context(), resourcesToUpsert)
			if err != nil {
				return fmt.Errorf("failed to upsert resources: %w", err)
			}
			fmt.Printf("Response from upserting resources: %v\n", upsertResp.StatusCode)
			fmt.Printf("Upserted %d resources\n", len(resourcesToUpsert))
			return nil
		},
	}

	cmd.Flags().StringVarP(&clusterIdentifier, "cluster-identifier", "c", "", "The identifier of the parent cluster in ctrlplane (if not provided, will use the CLUSTER_IDENTIFIER environment variable)")
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "The name of the resource provider (optional)")

	return cmd
}
