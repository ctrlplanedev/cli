package helm

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// getKubeConfig loads Kubernetes configuration and returns both the config and a suggested cluster name.
// It tries multiple sources in order of precedence:
// 1. KUBECONFIG environment variable
// 2. Default location (~/.kube/config)
// 3. In-cluster config (when running inside a Kubernetes pod)
//
// Returns: (*rest.Config, clusterName, error)
func getKubeConfig() (*rest.Config, string, error) {
	// Try 1: KUBECONFIG environment variable
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		log.Info("Loading kubeconfig from environment variable", "path", kubeconfigPath)
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, "", err
		}
		clusterName, err := getCurrentContextName(kubeconfigPath)
		return config, clusterName, err
	}

	// Try 2: Default location (~/.kube/config)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
		if _, err := os.Stat(kubeconfigPath); err == nil {
			log.Info("Loading kubeconfig from home directory", "path", kubeconfigPath)
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return nil, "", err
			}
			clusterName, err := getCurrentContextName(kubeconfigPath)
			return config, clusterName, err
		}
	}

	// Try 3: In-cluster config (running inside a pod)
	log.Info("Loading in-cluster kubeconfig")
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, "", err
	}

	// When running in-cluster, derive name from the namespace
	clusterName, err := getInClusterName()
	return config, clusterName, err
}

// getCurrentContextName extracts the current context name from a kubeconfig file.
// This is typically a meaningful cluster name like "production-cluster" or "minikube".
func getCurrentContextName(kubeconfigPath string) (string, error) {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", err
	}
	return kubeconfig.CurrentContext, nil
}

// getInClusterName returns a cluster name when running inside a Kubernetes pod.
// It reads the pod's namespace as a fallback identifier since there's no kubeconfig.
func getInClusterName() (string, error) {
	// When running inside a pod, read the namespace from the service account mount
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		// If we can't read the namespace, return a default
		return "unknown-cluster", nil
	}

	return string(nsBytes), nil
}

// getConfigFlags converts a rest.Config into Helm-compatible ConfigFlags.
// Helm's action package requires genericclioptions.ConfigFlags rather than rest.Config directly.
func getConfigFlags(config *rest.Config, namespace string) *genericclioptions.ConfigFlags {
	configFlags := genericclioptions.NewConfigFlags(true)

	// Set the Kubernetes API server URL
	if config.Host != "" {
		host := config.Host
		configFlags.APIServer = &host
	}

	// Set the target namespace (if specified)
	if namespace != "" {
		configFlags.Namespace = &namespace
	}

	// Set authentication token (if using token-based auth)
	if config.BearerToken != "" {
		token := config.BearerToken
		configFlags.BearerToken = &token
	}

	// Set TLS options
	if config.Insecure {
		insecure := true
		configFlags.Insecure = &insecure
	}

	// CA data is embedded in the config and will be used automatically
	// We rely on the kubeconfig file being available rather than creating a temp file

	// Set kubeconfig path (needed by Helm for some operations)
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
		}
	}
	if kubeconfigPath != "" {
		configFlags.KubeConfig = &kubeconfigPath
	}

	return configFlags
}
