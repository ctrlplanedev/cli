package kubernetes

import (
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

			clusterResourceResponse, err := ctrlplaneClient.GetResourceByIdentifierWithResponse(cmd.Context(), workspaceId, clusterIdentifier)
			if err != nil {
				return fmt.Errorf("failed to get cluster resource: %w", err)
			}
			if clusterResourceResponse.StatusCode() != 200 {
				return fmt.Errorf("failed to get cluster resource: %s", clusterResourceResponse.Status())
			}
			clusterResource := clusterResourceResponse.JSON200

			config, context, err := getKubeConfig()
			if err != nil {
				return fmt.Errorf("failed to get kube config: %w", err)
			}

			clientset, err := kube.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			allNamespaces, err := clientset.CoreV1().Namespaces().List(cmd.Context(), metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("failed to get all namespaces: %w", err)
			}
			for _, namespace := range allNamespaces.Items {
				fmt.Printf("Namespace: %s\n", namespace.Name)
				statefulSetList, err := clientset.AppsV1().StatefulSets(namespace.Name).List(cmd.Context(), metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to get stateful sets for namespace %s: %w", namespace.Name, err)
				}
				for _, p := range statefulSetList.Items {
					fmt.Printf("StatefulSet: %s\n", p.Name)
				}
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
				metadata, err := generateVclusterMetadata(vcluster, clusterResource.Metadata)
				if err != nil {
					fmt.Printf("failed to generate vcluster metadata for %s: %v\n", vcluster.Name, err)
					continue
				}

				clonedParentConfig, err := deepClone(clusterResource.Config)
				if err != nil {
					fmt.Printf("failed to clone parent config for %s: %v\n", vcluster.Name, err)
					continue
				}

				resource := api.CreateResource{
					Name:       fmt.Sprintf("%s/%s", clusterResource.Name, vcluster.Name),
					Identifier: fmt.Sprintf("%s/vcluster/%s", clusterResource.Identifier, vcluster.Name),
					Kind:       fmt.Sprintf("%s/%s", clusterResource.Kind, vclusterKind),
					Version:    "ctrlplane.dev/kubernetes/cluster/v1",
					Metadata:   metadata,
					Config:     generateVclusterConfig(vcluster, clusterResource.Name, clonedParentConfig),
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
