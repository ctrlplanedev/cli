package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func processNamespace(_ context.Context, namespace corev1.Namespace) api.CreateResource {
	metadata := map[string]string{}
	for key, value := range namespace.Labels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}

	metadata["kubernetes/namespace"] = namespace.Name
	metadata["namespace/id"] = string(namespace.UID)
	metadata["namespace/api-version"] = namespace.APIVersion
	metadata["namespace/status"] = string(namespace.Status.Phase)

	return api.CreateResource{
		Version: "ctrlplane.dev/kubernetes/namespace/v1",
		Kind: "KubernetesNamespace",
		Name: namespace.Name,
		Identifier: string(namespace.UID),
		Config: map[string]any{
			"id": string(namespace.UID),
			"name": namespace.Name,
			"status": namespace.Status.Phase,
		},
		Metadata: metadata,
	}
}

func NewSyncKubernetesCmd() *cobra.Command {
	var clusterIdentifier string
	var providerName string
	var clusterName string

	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Sync Kubernetes resources on a cluster",
		Example: heredoc.Doc(`
			$ ctrlc sync kubernetes --cluster 1234567890
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Syncing Kubernetes resources on a cluster")
			if clusterIdentifier == "" {
				clusterIdentifier = viper.GetString("cluster-identifier")
			}

			config, configClusterName, err := getKubeConfig()
			if err != nil {
				return err
			}

			if clusterName == "" {
				clusterName = configClusterName
			}

			log.Info("Connected to cluster", "name", clusterName)

			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				return err
			}

			namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}

			resources := []api.CreateResource{}
			for _, namespace := range namespaces.Items {
				resource := processNamespace(context.Background(), namespace)
				resources = append(resources, resource)
			}

			return upsertToCtrlplane(context.Background(), resources, clusterIdentifier, clusterName, providerName)
		},
	}
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&clusterIdentifier, "cluster-identifier", "c", "", "The identifier of the parent cluster in ctrlplane (if not provided, will use the CLUSTER_IDENTIFIER environment variable)")
	cmd.Flags().StringVarP(&clusterName, "cluster-name", "n", "", "The name of the cluster")

	return cmd
}

// upsertToCtrlplane handles upserting resources to Ctrlplane
func upsertToCtrlplane(ctx context.Context, resources []api.CreateResource, clusterIdentifier string, clusterName string, providerName string) error {
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	clusterResource, _ := ctrlplaneClient.GetResourceByIdentifierWithResponse(ctx, workspaceId, clusterIdentifier)
	if clusterResource.JSON200 != nil {
		for _, resource := range resources {
			for key, value := range clusterResource.JSON200.Metadata {
				if strings.HasPrefix(key, "tags/") {
					continue
				}
				if _, exists := resource.Metadata[key]; !exists {
					resource.Metadata[key] = value
				}
			}
			resource.Metadata["kubernetes/name"] = clusterResource.JSON200.Name
		}
		if clusterName == "" {
			clusterName = clusterResource.JSON200.Name
		}
		
	}

	if providerName == "" {
		providerName = fmt.Sprintf("kubernetes-cluster-%s", clusterName)
	}

	log.Info("Using provider name", "provider", providerName)

	rp, err := api.NewResourceProvider(ctrlplaneClient, workspaceId, providerName)
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
