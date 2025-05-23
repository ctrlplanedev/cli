package kubernetes

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/Masterminds/semver"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/sirupsen/logrus"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateVclusterMetadata(vcluster find.VCluster, clusterMetadata api.MetadataMap) (map[string]string, error) {
	metadata := make(map[string]string)
	parsedVersion, err := semver.NewVersion(vcluster.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vcluster version: %w", err)
	}
	metadata["vcluster/version"] = vcluster.Version
	metadata["vcluster/version-major"] = strconv.FormatInt(parsedVersion.Major(), 10)
	metadata["vcluster/version-minor"] = strconv.FormatInt(parsedVersion.Minor(), 10)
	metadata["vcluster/version-patch"] = strconv.FormatInt(parsedVersion.Patch(), 10)
	metadata["vcluster/name"] = vcluster.Name
	metadata["vcluster/namespace"] = vcluster.Namespace
	metadata["vcluster/status"] = string(vcluster.Status)
	metadata["vcluster/created"] = vcluster.Created.Format(time.RFC3339)
	metadata["kubernetes/flavor"] = "vcluster"

	for key, value := range clusterMetadata {
		metadata[key] = value
	}

	return metadata, nil
}

func generateVclusterConfig(vcluster find.VCluster, clusterName string, clusterConfig map[string]interface{}) map[string]interface{} {
	config := make(map[string]interface{})
	config["name"] = clusterName
	config["namespace"] = vcluster.Namespace
	config["status"] = vcluster.Status
	config["vcluster"] = vcluster.Name

	for key, value := range clusterConfig {
		config[key] = value
	}

	return config
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
				clusterIdentifier = viper.GetString("cluster_identifier")
			}

			if clusterIdentifier == "" {
				return fmt.Errorf("cluster identifier is required, please set the CLUSTER_IDENTIFIER environment variable or use the --cluster-identifier flag")
			}

			ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			clusterResourceResponse, err := ctrlplaneClient.GetResourceByIdentifierWithResponse(cmd.Context(), workspaceId, clusterIdentifier)
			if err != nil {
				return fmt.Errorf("failed to get cluster resource: %w", err)
			}
			clusterResource := clusterResourceResponse.JSON200

			logger := log.NewStdoutLogger(os.Stdout, os.Stdout, os.Stdout, logrus.InfoLevel)
			namespace := metav1.NamespaceAll
			vclusters, err := find.ListVClusters(cmd.Context(), "", "", namespace, logger)
			if err != nil {
				return err
			}

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
				resource := api.CreateResource{
					Name:       fmt.Sprintf("%s/%s/%s", clusterResource.Name, vcluster.Namespace, vcluster.Name),
					Identifier: fmt.Sprintf("%s/%s/%s", clusterResource.Name, vcluster.Namespace, vcluster.Name),
					Kind:       "ClusterAPI",
					Version:    clusterResource.Version,
					Metadata:   metadata,
					Config:     generateVclusterConfig(vcluster, clusterResource.Name, clusterResource.Config),
				}
				resourcesToUpsert = append(resourcesToUpsert, resource)
			}

			upsertResp, err := rp.UpsertResource(cmd.Context(), resourcesToUpsert)
			if err != nil {
				return fmt.Errorf("failed to upsert resources: %w", err)
			}
			fmt.Printf("Response from upserting resources: %v\n", upsertResp.StatusCode)
			return nil
		},
	}

	cmd.Flags().StringVar(&clusterIdentifier, "cluster-identifier", "c", "The identifier of the parent cluster in ctrlplane (if not provided, will use the CLUSTER_IDENTIFIER environment variable)")
	cmd.Flags().StringVar(&providerName, "provider", "p", "The name of the resource provider (optional)")

	return cmd
}
