package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type syncConfig struct {
	clusterIdentifier string
	workspaceID       string
	clusterName       string
	kubeConfig        *rest.Config
	client            *api.ClientWithResponses
	resources         []api.ResourceProviderResource
}

func (s *syncConfig) FetchNodes(ctx context.Context, clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	resources := make([]api.ResourceProviderResource, 0)
	for _, node := range nodes.Items {
		processedNode := processNode(ctx, s.clusterName, node)
		resources = append(resources, processedNode)
	}
	s.resources = append(s.resources, resources...)
	return nil
}

func (s *syncConfig) FetchNamespaces(ctx context.Context, clientset *kubernetes.Clientset) error {
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	resources := make([]api.ResourceProviderResource, 0)
	for _, namespace := range namespaces.Items {
		resource := processNamespace(context.Background(), s.clusterName, namespace)
		resources = append(resources, resource)
	}
	s.resources = append(s.resources, resources...)
	return nil

}

func (s *syncConfig) FetchDeployments(ctx context.Context, clientset *kubernetes.Clientset) error {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	resources := make([]api.ResourceProviderResource, 0)
	for _, deployment := range deployments.Items {
		resource := processDeployment(context.Background(), s.clusterName, deployment)
		resources = append(resources, resource)
	}
	s.resources = append(s.resources, resources...)
	return nil
}

func newSync(clusterIdentifier string, workspaceID string, client *api.ClientWithResponses, kubeConfig *rest.Config, clusterName string) *syncConfig {
	return &syncConfig{
		clusterIdentifier: clusterIdentifier,
		workspaceID:       workspaceID,
		client:            client,
		clusterName:       clusterName,
		kubeConfig:        kubeConfig,
		resources:         make([]api.ResourceProviderResource, 0),
	}
}

func processNamespace(_ context.Context, clusterName string, namespace corev1.Namespace) api.ResourceProviderResource {
	metadata := map[string]string{}
	for key, value := range namespace.Labels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}

	metadata["kubernetes/namespace"] = namespace.Name
	metadata["namespace/id"] = string(namespace.UID)
	metadata["namespace/api-version"] = namespace.APIVersion
	metadata["namespace/status"] = string(namespace.Status.Phase)

	return api.ResourceProviderResource{
		Version:    "ctrlplane.dev/kubernetes/namespace/v1",
		Kind:       "KubernetesNamespace",
		Name:       fmt.Sprintf("%s/%s", clusterName, namespace.Name),
		Identifier: string(namespace.UID),
		Config: map[string]any{
			"id":     string(namespace.UID),
			"name":   namespace.Name,
			"status": namespace.Status.Phase,
		},
		Metadata: metadata,
	}
}

func processNode(_ context.Context, clusterName string, node corev1.Node) api.ResourceProviderResource {
	metadata := make(map[string]string)
	for key, value := range node.Labels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}
	metadata["kubernetes/uid"] = string(node.UID)
	metadata["kubernetes/created-at"] = node.CreationTimestamp.String()
	// Get node ready status
	nodeReady := "Unknown"
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			nodeReady = string(condition.Status)
			break
		}
	}
	metadata["kubernetes/node-ready"] = nodeReady
	// Get node role from labels
	nodeRole := "worker"
	for label := range node.Labels {
		if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok && role != "" {
			nodeRole = role
			break
		}
	}
	metadata["kubernetes/node-role"] = nodeRole
	metadata["kubernetes/kubelet-version"] = node.Status.NodeInfo.KubeletVersion
	metadata["kubernetes/os-image"] = node.Status.NodeInfo.OSImage
	metadata["kubernetes/architecture"] = node.Status.NodeInfo.Architecture
	metadata["kubernetes/container-runtime"] = node.Status.NodeInfo.ContainerRuntimeVersion

	return api.ResourceProviderResource{
		Version:    "ctrlplane.dev/kubernetes/node/v1",
		Kind:       "KubernetesNode",
		Name:       fmt.Sprintf("%s/%s", clusterName, node.Name),
		Identifier: string(node.UID),
		Config: map[string]any{
			"id":     string(node.UID),
			"name":   node.Name,
			"status": node.Status.Phase,
		},
		Metadata: metadata,
	}
}

func processDeployment(_ context.Context, clusterName string, deployment appsv1.Deployment) api.ResourceProviderResource {
	metadata := map[string]string{}
	for key, value := range deployment.Labels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}
	metadata["deployment/name"] = deployment.Name
	metadata["deployment/id"] = string(deployment.UID)
	metadata["deployment/api-version"] = deployment.APIVersion
	metadata["deployment/namespace"] = deployment.Namespace

	return api.ResourceProviderResource{
		Version:    "ctrlplane.dev/kubernetes/deployment/v1",
		Kind:       "KubernetesDeployment",
		Name:       fmt.Sprintf("%s/%s/%s", clusterName, deployment.Namespace, deployment.Name),
		Identifier: string(deployment.UID),
		Config: map[string]any{
			"id":        string(deployment.UID),
			"name":      deployment.Name,
			"namespace": deployment.Namespace,
		},
		Metadata: metadata,
	}
}

func (s *syncConfig) process(ctx context.Context, selectors ResourceTypes) ([]api.ResourceProviderResource, error) {
	clusterResource, err := s.client.GetResourceByIdentifierWithResponse(ctx, s.workspaceID, s.clusterIdentifier)
	if err != nil {
		log.Warn("Failed to get cluster resource", "identifier", s.clusterIdentifier, "error", err)
	}
	if clusterResource != nil && clusterResource.StatusCode() > 499 {
		log.Warn("Failed to get cluster resource", "status", clusterResource.StatusCode(), "identifier", s.clusterIdentifier, "error", err)
		return nil, fmt.Errorf("error access ctrlplane api: %s", clusterResource.Status())
	}

	if clusterResource != nil && clusterResource.JSON200 != nil {
		log.Info("Found cluster resource", "name", clusterResource.JSON200.Name)
		s.clusterName = clusterResource.JSON200.Name
	}

	clientset, err := kubernetes.NewForConfig(s.kubeConfig)
	if err != nil {
		return nil, err
	}

	if selectors.ShouldFetch(ResourceNamespace) {
		if err := s.FetchNamespaces(ctx, clientset); err != nil {
			return s.resources, err
		}
	}

	if selectors.ShouldFetch(ResourceDeployment) {
		if err := s.FetchDeployments(ctx, clientset); err != nil {
			return s.resources, err
		}
	}

	if selectors.ShouldFetch(ResourceNode) {
		if err := s.FetchNodes(ctx, clientset); err != nil {
			return s.resources, err
		}
	}

	if clusterResource != nil && clusterResource.JSON200 != nil {
		for _, resource := range s.resources {
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
	}

	return s.resources, nil
}
