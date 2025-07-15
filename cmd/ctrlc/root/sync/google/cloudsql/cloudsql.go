package cloudsql

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/Masterminds/semver"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/kinds"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/api/sqladmin/v1"
)

// getDatabaseTypeAndVersion extracts the database type and version from the
// Cloud SQL database version string
func getDatabaseTypeAndVersion(databaseVersion string) (string, *semver.Version) {
	// Default values
	dbType := "unknown"
	versionObj, _ := semver.NewVersion("0.0.0")

	// Handle MySQL versions
	if strings.Contains(databaseVersion, "MYSQL") {
		dbType = "mysql"

		// Handle common MySQL versions directly
		switch databaseVersion {
		case "MYSQL_5_1":
			versionObj, _ = semver.NewVersion("5.1.0")
			return dbType, versionObj
		case "MYSQL_5_5":
			versionObj, _ = semver.NewVersion("5.5.0")
			return dbType, versionObj
		case "MYSQL_5_6":
			versionObj, _ = semver.NewVersion("5.6.0")
			return dbType, versionObj
		case "MYSQL_5_7":
			versionObj, _ = semver.NewVersion("5.7.0")
			return dbType, versionObj
		case "MYSQL_8_0":
			versionObj, _ = semver.NewVersion("8.0.0")
			return dbType, versionObj
		case "MYSQL_8_0_18":
			versionObj, _ = semver.NewVersion("8.0.18")
			return dbType, versionObj
		}

		// Parse other MySQL versions
		if strings.HasPrefix(databaseVersion, "MYSQL_") {
			parts := strings.Split(strings.TrimPrefix(databaseVersion, "MYSQL_"), "_")
			if len(parts) >= 2 {
				versionStr := parts[0] + "." + parts[1]
				if len(parts) >= 3 {
					versionStr += "." + parts[2]
				} else {
					versionStr += ".0" // Add patch version for semver compatibility
				}

				versionObj, _ = semver.NewVersion(versionStr)
				return dbType, versionObj
			}
		}

		return dbType, versionObj
	}

	// Handle PostgreSQL versions
	if strings.Contains(databaseVersion, "POSTGRES") {
		dbType = "postgres"

		if strings.HasPrefix(databaseVersion, "POSTGRES_") {
			parts := strings.Split(strings.TrimPrefix(databaseVersion, "POSTGRES_"), "_")
			if len(parts) >= 1 {
				versionStr := parts[0]
				if len(parts) >= 2 {
					versionStr += "." + parts[1]
				} else {
					versionStr += ".0" // Add minor version for semver compatibility
				}
				if len(parts) >= 3 {
					versionStr += "." + parts[2]
				} else {
					versionStr += ".0" // Add patch version for semver compatibility
				}

				versionObj, _ = semver.NewVersion(versionStr)
				return dbType, versionObj
			}
		}

		return dbType, versionObj
	}

	// Handle SQL Server versions
	if strings.Contains(databaseVersion, "SQLSERVER") {
		dbType = "sqlserver"

		if strings.HasPrefix(databaseVersion, "SQLSERVER_") {
			parts := strings.Split(strings.TrimPrefix(databaseVersion, "SQLSERVER_"), "_")
			if len(parts) >= 1 {
				versionStr := parts[0]
				if len(parts) >= 2 {
					versionStr += "." + parts[1]
				} else {
					versionStr += ".0" // Add minor version for semver compatibility
				}
				if len(parts) >= 3 {
					versionStr += "." + parts[2]
				} else {
					versionStr += ".0" // Add patch version for semver compatibility
				}

				versionObj, _ = semver.NewVersion(versionStr)
				return dbType, versionObj
			}
		}
	}

	return dbType, versionObj
}

func getInstanceHostAndPort(instance *sqladmin.DatabaseInstance) (string, int) {
	// Default port based on database type
	var port int
	switch {
	case strings.Contains(instance.DatabaseVersion, "POSTGRES"):
		port = 5432
	case strings.Contains(instance.DatabaseVersion, "MYSQL"):
		port = 3306
	case strings.Contains(instance.DatabaseVersion, "SQLSERVER"):
		port = 1433
	default:
		port = 5432
	}

	// Get the primary IP address
	var host string
	if instance.IpAddresses != nil {
		for _, ip := range instance.IpAddresses {
			if ip.Type == "PRIMARY" {
				host = ip.IpAddress
				break
			}
		}

		if host == "" {
			for _, ip := range instance.IpAddresses {
				if ip.Type == "PRIVATE" {
					host = ip.IpAddress
					break
				}
			}
		}

		if host == "" {
			log.Error("No IP address found for instance", "instance", instance.Name)
		}
	} else {
		log.Error("No IP addresses found for instance", "instance", instance.Name)
	}

	return host, port
}

func NewSyncCloudSQLCmd() *cobra.Command {
	var project string
	var providerName string

	cmd := &cobra.Command{
		Use:   "cloudsql",
		Short: "Sync Google Cloud SQL instances into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure Google Cloud credentials are configured via environment variables or application default credentials
			
			# Sync all Cloud SQL instances from a project
			$ ctrlc sync google-cloud cloudsql --project my-project
		`),
		PreRunE: validateFlags(&project),
		RunE:    runSync(&project, &providerName),
	}

	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVarP(&project, "project", "c", "", "Google Cloud Project ID")
	cmd.MarkFlagRequired("project")

	return cmd
}

// validateFlags ensures required flags are set
func validateFlags(project *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *project == "" {
			return fmt.Errorf("project is required")
		}
		return nil
	}
}

// runSync contains the main sync logic
func runSync(project, providerName *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Info("Syncing Cloud SQL instances into Ctrlplane", "project", *project)

		ctx := context.Background()

		// Initialize SQL Admin client
		sqlService, err := initSQLAdminClient(ctx)
		if err != nil {
			return err
		}

		// List and process instances
		resources, err := processInstances(ctx, sqlService, *project)
		if err != nil {
			return err
		}

		// Upsert resources to Ctrlplane
		return upsertToCtrlplane(ctx, resources, project, providerName)
	}
}

// initSQLAdminClient creates a new Cloud SQL Admin client
func initSQLAdminClient(ctx context.Context) (*sqladmin.Service, error) {
	sqlService, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud SQL Admin client: %w", err)
	}
	return sqlService, nil
}

// processInstances lists and processes all Cloud SQL instances
func processInstances(_ context.Context, sqlService *sqladmin.Service, project string) ([]api.CreateResource, error) {
	instances, err := sqlService.Instances.List(project).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	log.Info("Found instances", "count", len(instances.Items))

	resources := []api.CreateResource{}
	for _, instance := range instances.Items {
		resource := processInstance(instance, project)
		resources = append(resources, resource)
	}

	return resources, nil
}

// processInstance handles processing of a single Cloud SQL instance
func processInstance(instance *sqladmin.DatabaseInstance, project string) api.CreateResource {
	// Extract region from zone
	region := strings.Join(strings.Split(instance.GceZone, "-")[:2], "-")

	// Get connection details
	host, port := getInstanceHostAndPort(instance)

	// Build console URL
	consoleUrl := fmt.Sprintf("https://console.cloud.google.com/sql/instances/%s?project=%s",
		instance.Name, project)

	metadata := buildInstanceMetadata(instance, project, region, host, port, consoleUrl)

	return api.CreateResource{
		Version:    "ctrlplane.dev/database/v1",
		Kind:       "GoogleCloudSQL",
		Name:       instance.Name,
		Identifier: instance.SelfLink,
		Config: map[string]any{
			"name": instance.Name,
			"host": host,
			"port": port,
			"ssl":  instance.Settings.IpConfiguration != nil && instance.Settings.IpConfiguration.RequireSsl,
			"googleCloudsql": map[string]any{
				"project":                    project,
				"region":                     region,
				"version":                    instance.DatabaseVersion,
				"state":                      strings.ToLower(instance.State),
				"connectionName":             instance.ConnectionName,
				"serviceAccountEmailAddress": instance.ServiceAccountEmailAddress,
			},
		},
		Metadata: metadata,
	}
}

// buildInstanceMetadata builds the metadata map for a Cloud SQL instance
func buildInstanceMetadata(instance *sqladmin.DatabaseInstance, project, region, host string, port int, consoleUrl string) map[string]string {
	dbType, versionObj := getDatabaseTypeAndVersion(instance.DatabaseVersion)

	log.Info("Database type and version", "dbType", dbType, "version", versionObj)
	metadata := map[string]string{
		kinds.DBMetadataType:         dbType,
		kinds.DBMetadataRegion:       region,
		kinds.DBMetadataState:        instance.State,
		kinds.DBMetadataTier:         instance.Settings.Tier,
		kinds.DBMetadataVersion:      versionObj.Original(),
		kinds.DBMetadataVersionMajor: strconv.FormatInt(versionObj.Major(), 10),
		kinds.DBMetadataVersionMinor: strconv.FormatInt(versionObj.Minor(), 10),
		kinds.DBMetadataVersionPatch: strconv.FormatInt(versionObj.Patch(), 10),
		kinds.DBMetadataHost:         host,
		kinds.DBMetadataPort:         strconv.Itoa(port),

		"google/connection-name":   instance.ConnectionName,
		"google/availability-type": strings.ToLower(instance.Settings.AvailabilityType),
		"google/project":           project,
		"google/instance-type":     instance.InstanceType,
		"google/self-link":         instance.SelfLink,
		"google/version":           instance.DatabaseVersion,
		"google/console-url":       consoleUrl,
		"google/sla-edition":       instance.Settings.Edition,
		"google/state":             strings.ToLower(instance.State),

		"google/disk-type":                   instance.Settings.DataDiskType,
		"google/disk-size-gb":                strconv.FormatInt(instance.Settings.DataDiskSizeGb, 10),
		"google/disk-iops":                   strconv.FormatInt(instance.Settings.DataDiskProvisionedIops, 10),
		"google/disk-provisioned-iops":       strconv.FormatInt(instance.Settings.DataDiskProvisionedIops, 10),
		"google/disk-provisioned-throughput": strconv.FormatInt(instance.Settings.DataDiskProvisionedThroughput, 10),

		"compute/machine-type": instance.Settings.Tier,
		"compute/disk-type":    instance.Settings.DataDiskType,
		"compute/disk-size":    strconv.FormatInt(instance.Settings.DataDiskSizeGb, 10),

		"ctrlplane/links": fmt.Sprintf("{ \"Google Cloud Console\": \"%s\" }", consoleUrl),
	}

	if instance.Settings != nil && instance.Settings.IpConfiguration != nil {
		ipConfig := instance.Settings.IpConfiguration
		if ipConfig.PrivateNetwork != "" {
			privateNetworkName := ""
			parts := strings.Split(ipConfig.PrivateNetwork, "/")
			if len(parts) >= 5 {
				privateNetworkName = parts[len(parts)-1]
			}
			if privateNetworkName != "" {
				metadata["network/name"] = privateNetworkName
			}
		}

		if ipConfig.RequireSsl {
			metadata[kinds.DBMetadataSSL] = "true"
		} else {
			metadata[kinds.DBMetadataSSL] = "false"
		}
	}

	// Add database flags
	for _, flag := range instance.Settings.DatabaseFlags {
		metadata[fmt.Sprintf("database/parameter/%s", flag.Name)] = flag.Value
	}

	// Add IP addresses
	if instance.IpAddresses != nil {
		for _, ip := range instance.IpAddresses {
			metadata[fmt.Sprintf("network/%s-ip", strings.ToLower(ip.Type))] = ip.IpAddress
		}
	}

	if instance.Settings.AvailabilityType != "" {
		metadata["compute/availability-type"] = instance.Settings.AvailabilityType
	}

	for key, value := range instance.Settings.UserLabels {
		metadata[fmt.Sprintf("tags/%s", key)] = value
	}

	return metadata
}

var relationshipRules = []api.CreateResourceRelationshipRule{
	{
		Reference:      "network",
		Name:           "Google Cloud SQL Network",
		DependencyType: api.ProvisionedIn,

		SourceVersion: "ctrlplane.dev/database/v1",
		SourceKind:    "GoogleCloudSQL",
		TargetVersion: "ctrlplane.dev/network/v1",
		TargetKind:    "GoogleNetwork",

		MetadataKeysMatches: &[]api.MetadataKeyMatchConstraint{
			{SourceKey: "google/project", TargetKey: "google/project"},
			{SourceKey: "network/id", TargetKey: "network/id"},
		},
	},
}

// upsertToCtrlplane handles upserting resources to Ctrlplane
func upsertToCtrlplane(ctx context.Context, resources []api.CreateResource, project, providerName *string) error {
	if *providerName == "" {
		*providerName = fmt.Sprintf("google-cloudsql-%s", *project)
	}

	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspaceId := viper.GetString("workspace")

	ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	log.Info("Upserting resource provider", "name", *providerName)
	rp, err := api.NewResourceProvider(ctrlplaneClient, workspaceId, *providerName)
	if err != nil {
		return fmt.Errorf("failed to create resource provider: %w", err)
	}

	err = rp.AddResourceRelationshipRule(ctx, relationshipRules)
	if err != nil {
		log.Error("Failed to add resource relationship rule", "name", *providerName, "error", err)
	}

	upsertResp, err := rp.UpsertResource(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to upsert resources: %w", err)
	}

	log.Info("Response from upserting resources", "status", upsertResp.Status)
	return nil
}
