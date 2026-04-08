package resources

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/internal/resources"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourcesCmd() *cobra.Command {
	var kinds []string
	var metadata []string
	var versions []string
	var providerIDs []string
	var jq string
	var autoAccept bool
	var output string

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Get resources",
		Long:  `Get resources with optional filtering or client-side jq filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdStart := time.Now()
			defer func() {
				log.Debug("get resources completed", "duration", time.Since(cmdStart))
			}()

			log.Debug("resources command", "kinds", kinds, "metadata", metadata, "versions", versions, "providerIDs", providerIDs, "jq", jq, "autoAccept", autoAccept)

			if jq != "" {
				if _, err := gojq.Parse(jq); err != nil {
					return fmt.Errorf("invalid jq expression: %w", err)
				}
			}

			svc, err := resources.NewAPIResourceService(cmd.Context(), viper.GetString("url"), viper.GetString("api-key"), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			filters := buildFilters(kinds, metadata, versions, providerIDs)

			if jq != "" {
				return handleJQ(cmd, svc, filters, jq, autoAccept, output)
			}

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
	cmd.Flags().StringVar(&jq, "jq", "", "Client-side jq filter expression (fetches all resources)")
	cmd.Flags().BoolVar(&autoAccept, "auto-accept", false, "Skip confirmation prompt when using --jq")
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

func handleJQ(cmd *cobra.Command, svc *resources.APIResourceService, filters api.ListResourcesFilters, jqExpr string, autoAccept bool, output string) error {
	total, err := svc.SearchTotal(cmd.Context(), filters)
	if err != nil {
		return err
	}

	pages := int(math.Ceil(float64(total) / 200.0))

	if !autoAccept {
		message := fmt.Sprintf("Found %d resources. This will require ~%d API request(s). Continue?", total, pages)
		confirmed, err := cliutil.ConfirmAction(message)
		if err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirmed {
			fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
			return nil
		}
	}

	items, err := svc.Search(cmd.Context(), filters)
	if err != nil {
		return err
	}

	// Convert resources to generic interface for jq processing
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal resources: %w", err)
	}
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("failed to unmarshal resources: %w", err)
	}

	results, err := cliutil.ApplyJQ(jqExpr, data)
	if err != nil {
		return err
	}

	if len(results) == 1 {
		return cliutil.HandleAnyOutput(cmd, results[0], output)
	}
	return cliutil.HandleAnyOutput(cmd, results, output)
}
