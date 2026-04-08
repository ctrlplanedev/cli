package resources

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/internal/resources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourcesCmd() *cobra.Command {
	var kinds []string
	var metadata []string
	var versions []string
	var providerIDs []string
	var output string

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Get resources",
		Long:  `Get resources with optional filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdStart := time.Now()
			defer func() {
				log.Debug("get resources completed", "duration", time.Since(cmdStart))
			}()

			log.Debug("resources command", "kinds", kinds, "metadata", metadata, "versions", versions, "providerIDs", providerIDs)

			if len(kinds) == 0 && len(metadata) == 0 && len(versions) == 0 && len(providerIDs) == 0 {
				return fmt.Errorf("at least one filter is required (--kind, --metadata, --version, or --provider-id)")
			}

			svc, err := resources.NewAPIResourceService(cmd.Context(), viper.GetString("url"), viper.GetString("api-key"), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			filters := buildFilters(kinds, metadata, versions, providerIDs)

			items, err := svc.Search(cmd.Context(), filters)
			if err != nil {
				return err
			}

			return cliutil.HandleAnyOutput(cmd, items, output)
		},
	}

	cmd.Flags().StringSliceVarP(&kinds, "kind", "k", nil, "Filter by resource kind (repeatable)")
	cmd.Flags().StringSliceVarP(&metadata, "metadata", "m", nil, "Filter by metadata key=value (repeatable)")
	cmd.Flags().StringSliceVarP(&versions, "version", "v", nil, "Filter by resource version (repeatable)")
	cmd.Flags().StringSliceVarP(&providerIDs, "provider-id", "p", nil, "Filter by provider ID (repeatable)")
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json or yaml)")

	return cmd
}

func buildFilters(kinds, metadata, versions, providerIDs []string) api.ListResourcesFilters {
	filters := api.ListResourcesFilters{}

	if len(kinds) > 0 {
		filters.Kinds = &kinds
	}
	if len(versions) > 0 {
		filters.Versions = &versions
	}
	if len(providerIDs) > 0 {
		filters.ProviderIds = &providerIDs
	}
	if len(metadata) > 0 {
		m := make(map[string]string)
		for _, kv := range metadata {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				m[parts[0]] = parts[1]
			}
		}
		filters.Metadata = &m
	}

	return filters
}
