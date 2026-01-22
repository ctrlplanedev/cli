package apply

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewApplyCmd creates a new apply command
func NewApplyCmd() *cobra.Command {
	var filePatterns []string
	var selectors []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration file to create or update resources",
		Long:  `Apply a YAML configuration file to create or update resources in Ctrlplane.`,
		Example: heredoc.Doc(`
			# Apply a single resource file
			$ ctrlc apply -f system.yaml

			# Apply a multi-document file with systems, deployments, and environments
			$ ctrlc apply -f config.yaml

			# Apply all YAML files matching a glob pattern
			$ ctrlc apply -f "**/*.ctrlc.yaml"

			# Apply multiple patterns
			$ ctrlc apply -f infra/*.yaml -f apps/*.yaml

			# Exclude test files using ! prefix (git-style: last match wins)
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml"

			# Exclude multiple patterns
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml" -f "!**/staging/**"

			# Re-include a previously excluded file (last pattern wins)
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml" -f "**/important-test.yaml"

			# Declaratively manage all resources for "platform" team
			$ ctrlc apply -f config/ --selector team=platform

			# Multiple selectors (AND logic)
			$ ctrlc apply -f config/ --selector team=platform --selector env=staging

			# Preview changes first
			$ ctrlc apply -f config/ --selector env=staging --dry-run
		`),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), filePatterns, selectors, dryRun)
		},
	}

	cmd.Flags().StringArrayVarP(&filePatterns, "file", "f", nil, "Path or glob pattern to YAML files (can be specified multiple times, prefix with ! to exclude)")
	cmd.Flags().StringArrayVar(&selectors, "selector", nil, "Selector to match resources for declarative management (format: key=value, can be specified multiple times)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	cmd.MarkFlagRequired("file")

	return cmd
}

// expandGlob expands glob patterns to file paths, supporting ** for recursive matching
// It follows git-style pattern matching where later patterns override earlier ones
// and ! prefix negates (excludes) a pattern
func expandGlob(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	// Parse patterns into rules - ! prefix means exclude
	type patternRule struct {
		pattern string
		include bool // true = include, false = exclude
	}

	var rules []patternRule
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			rules = append(rules, patternRule{strings.TrimPrefix(p, "!"), false})
		} else {
			rules = append(rules, patternRule{p, true})
		}
	}

	// First, collect all potential files from include patterns
	candidateFiles := make(map[string]bool)
	for _, rule := range rules {
		if rule.include {
			matches, err := doublestar.FilepathGlob(rule.pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern '%s': %w", rule.pattern, err)
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				candidateFiles[match] = true
			}
		}
	}

	// For each candidate file, evaluate all rules in order - last match wins
	for filePath := range candidateFiles {
		included := false
		for _, rule := range rules {
			matched, err := doublestar.PathMatch(rule.pattern, filePath)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern '%s': %w", rule.pattern, err)
			}
			if matched {
				included = rule.include // last matching rule wins
			}
		}
		if included && !seen[filePath] {
			seen[filePath] = true
			files = append(files, filePath)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files matched patterns")
	}

	return files, nil
}

func runApply(ctx context.Context, filePatterns []string, selectors []string, dryRun bool) error {
	files, err := expandGlob(filePatterns)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files to apply")
	}

	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)

	var documents []Document
	for _, filePath := range files {
		docs, err := ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}
		documents = append(documents, docs...)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in files")
		return nil
	}

	log.Info("Applying resources", "count", len(documents), "files", len(files))

	docCtx := NewDocContext(workspaceID.String(), client)
	docCtx.Context = ctx

	var resourceDocs []*ResourceDocument
	var otherDocs []Document
	for _, doc := range documents {
		if rd, ok := doc.(*ResourceDocument); ok {
			resourceDocs = append(resourceDocs, rd)
		} else {
			otherDocs = append(otherDocs, doc)
		}
	}

	sort.Slice(otherDocs, func(i, j int) bool {
		return otherDocs[i].Order() > otherDocs[j].Order()
	})

	var results []ApplyResult
	for _, doc := range otherDocs {
		if dryRun {
			var docType DocumentType
			var docName string

			// Extract type and name based on document type
			switch d := doc.(type) {
			case *DeploymentDocument:
				docType = TypeDeployment
				docName = d.Name
			case *EnvironmentDocument:
				docType = TypeEnvironment
				docName = d.Name
			case *PolicyDocument:
				docType = TypePolicy
				docName = d.Name
			case *JobAgentDocument:
				docType = TypeJobAgent
				docName = d.Name
			default:
				docType = "Unknown"
				docName = "Unknown"
			}

			log.Info("DRY RUN: Would apply document", "type", docType, "name", docName)
			result := ApplyResult{
				Type:   docType,
				Name:   docName,
				Action: "would_apply",
				ID:     "",
				Error:  nil,
			}
			results = append(results, result)
		} else {
			result, err := doc.Apply(docCtx)
			if err != nil {
				log.Error("Failed to apply document", "error", err)
			}
			results = append(results, result)
		}
	}

	if len(resourceDocs) > 0 {
		if dryRun {
			for _, doc := range resourceDocs {
				log.Info("DRY RUN: Would apply resource", "name", doc.Name, "identifier", doc.Identifier)
				result := ApplyResult{
					Type:   TypeResource,
					Name:   doc.Name,
					Action: "would_upsert",
					ID:     doc.Identifier,
					Error:  nil,
				}
				results = append(results, result)
			}
		} else {
			resourceResults, err := applyResourcesBatch(docCtx, resourceDocs)
			if err != nil {
				log.Error("Failed to apply resources batch", "error", err)
			}
			results = append(results, resourceResults...)
		}
	}

	// Handle declarative management with selectors
	if len(selectors) > 0 {
		log.Info("Performing declarative management with selectors", "selectors", selectors)
		deleteResults, err := handleDeclarativeManagement(ctx, docCtx, selectors, resourceDocs, dryRun)
		if err != nil {
			log.Error("Failed declarative management", "error", err)
		} else {
			// Convert delete results to apply results for consistent display
			for _, dr := range deleteResults {
				results = append(results, ApplyResult{
					Type:   dr.Type,
					Name:   dr.Name,
					Action: dr.Action,
					ID:     dr.ID,
					Error:  dr.Error,
				})
			}
		}
	}

	if dryRun {
		printDryRunResults(results)
	} else {
		printResults(results)
	}

	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to apply")
		}
	}

	return nil
}

// handleDeclarativeManagement performs declarative resource management based on selectors
func handleDeclarativeManagement(ctx context.Context, docCtx *DocContext, selectors []string, declaredResources []*ResourceDocument, dryRun bool) ([]DeleteResult, error) {
	var results []DeleteResult

	// Parse selectors into key-value pairs
	selectorMap, err := parseSelectors(selectors)
	if err != nil {
		return nil, fmt.Errorf("failed to parse selectors: %w", err)
	}

	// Build CEL query from selectors
	celQuery, err := buildCELQueryFromSelectors(selectorMap)
	if err != nil {
		return nil, fmt.Errorf("failed to build CEL query: %w", err)
	}

	// Query resources matching the selector
	params := &api.GetAllResourcesParams{
		Limit: func() *int { l := 1000; return &l }(), // Get a large number of resources
	}
	if celQuery != "" {
		encodedQuery := url.QueryEscape(celQuery)
		params.Cel = &encodedQuery
	}

	allResourcesResp, err := docCtx.Client.GetAllResourcesWithResponse(ctx, docCtx.WorkspaceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %w", err)
	}

	if allResourcesResp.JSON200 == nil {
		return nil, fmt.Errorf("failed to get resources: %s", string(allResourcesResp.Body))
	}

	// Create a map of declared resource identifiers for quick lookup
	declaredMap := make(map[string]*ResourceDocument)
	for _, doc := range declaredResources {
		declaredMap[doc.Identifier] = doc
	}

	// Find resources that match selector but are not declared
	for _, resource := range allResourcesResp.JSON200.Items {
		if _, exists := declaredMap[resource.Identifier]; !exists {
			// This resource matches the selector but is not declared - it should be deleted
			result := DeleteResult{
				Type:   TypeResource,
				Name:   resource.Name,
				ID:     resource.Identifier,
				Action: "deleted",
			}

			if dryRun {
				result.Action = "would_delete"
				log.Info("DRY RUN: Would delete resource", "name", resource.Name, "identifier", resource.Identifier)
			} else {
				log.Info("Deleting resource matching selector but not in config", "name", resource.Name, "identifier", resource.Identifier)

				// Delete the resource using the delete endpoint
				deleteErr := deleteResource(ctx, docCtx, resource.Identifier)
				if deleteErr != nil {
					result.Error = fmt.Errorf("failed to delete resource: %w", deleteErr)
				}
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// deleteResource deletes a resource by identifier
func deleteResource(ctx context.Context, docCtx *DocContext, identifier string) error {
	// Query for the resource to get its ID
	celQuery := fmt.Sprintf(`resource.identifier == "%s"`, identifier)
	encodedQuery := url.QueryEscape(celQuery)
	params := &api.GetAllResourcesParams{
		Cel:   &encodedQuery,
		Limit: func() *int { l := 1; return &l }(),
	}

	resp, err := docCtx.Client.GetAllResourcesWithResponse(ctx, docCtx.WorkspaceID, params)
	if err != nil {
		return fmt.Errorf("failed to query resource: %w", err)
	}

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		return fmt.Errorf("resource not found: %s", identifier)
	}

	resource := resp.JSON200.Items[0]

	// If the resource has a provider ID, we can use the provider API to remove it
	// by setting the provider's resources without this resource
	if resource.ProviderId != nil && *resource.ProviderId != "" {
		// For provider-managed resources, the deletion happens through the provider's
		// SetResourceProvidersResources endpoint by not including the resource
		// This requires re-syncing all resources for that provider, which is complex
		// For now, we'll log a warning about this limitation
		log.Warn("Resource is managed by a provider - deletion requires provider sync",
			"identifier", identifier,
			"providerId", *resource.ProviderId)
		return fmt.Errorf("resource %s is managed by provider %s - manual deletion or provider re-sync required",
			identifier, *resource.ProviderId)
	}

	// For resources without a provider, there's currently no direct delete API
	// This is a limitation of the current API
	return fmt.Errorf("no direct delete API available for resource %s - resource may need to be deleted via UI or provider re-sync", identifier)
}

// parseSelectors parses selector strings in key=value format
func parseSelectors(selectors []string) (map[string]string, error) {
	selectorMap := make(map[string]string)

	for _, selector := range selectors {
		parts := strings.Split(selector, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid selector format: %s (expected key=value)", selector)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid selector format: %s (key and value cannot be empty)", selector)
		}
		selectorMap[key] = value
	}

	return selectorMap, nil
}

// buildCELQueryFromSelectors builds a CEL query string from selector map
func buildCELQueryFromSelectors(selectors map[string]string) (string, error) {
	if len(selectors) == 0 {
		return "", nil
	}

	var conditions []string
	for key, value := range selectors {
		// Build a condition that checks if the metadata contains the key-value pair
		// Using bracket notation for map access: resource.metadata["key"] == "value"
		condition := fmt.Sprintf(`resource.metadata["%s"] == "%s"`, key, value)
		conditions = append(conditions, condition)
	}

	// Join all conditions with AND
	return strings.Join(conditions, " && "), nil
}

func printResults(results []ApplyResult) {
	fmt.Println()

	// Color definitions
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.Faint)

	// Print apply results
	for _, r := range results {
		if r.Error != nil {
			red.Print("✗ ")
			fmt.Printf("%s/%s: ", r.Type, r.Name)
			red.Printf("%v\n", r.Error)
		} else {
			if r.Action == "deleted" {
				red.Print("✗ ")
			} else {
				green.Print("✓ ")
			}
			fmt.Printf("%s/", r.Type)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			dim.Printf("(id: %s)\n", r.ID)
		}
	}

	fmt.Println()

	// Count successes, deletions, and failures
	var upserted, deleted, failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else if r.Action == "deleted" {
			deleted++
		} else {
			upserted++
		}
	}

	// Summary with colors
	fmt.Printf("Applied %d resources: ", len(results))
	green.Printf("%d upserted", upserted)
	fmt.Print(", ")
	red.Printf("%d deleted", deleted)
	fmt.Print(", ")
	if failed > 0 {
		red.Printf("%d failed\n", failed)
	} else {
		fmt.Printf("%d failed\n", failed)
	}
}

// printDryRunResults prints the results of a dry run with appropriate formatting
func printDryRunResults(results []ApplyResult) {
	fmt.Println()
	fmt.Println("DRY RUN - No changes will be made")
	fmt.Println()

	// Color definitions
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.Faint)

	// Print dry run results
	for _, r := range results {
		if r.Error != nil {
			red.Print("✗ ")
			fmt.Printf("%s/%s: ", r.Type, r.Name)
			red.Printf("%v\n", r.Error)
		} else {
			if strings.HasPrefix(r.Action, "would_delete") {
				red.Print("✗ ")
			} else {
				green.Print("✓ ")
			}
			fmt.Printf("%s/", r.Type)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			if r.ID != "" {
				dim.Printf("(id: %s)", r.ID)
			}
			fmt.Println()
		}
	}

	fmt.Println()

	// Count different action types
	var upserted, deleted, failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else if strings.Contains(r.Action, "delete") {
			deleted++
		} else {
			upserted++
		}
	}

	// Summary with colors
	fmt.Printf("Would apply %d resources: ", len(results))
	green.Printf("%d upserted", upserted)
	fmt.Print(", ")
	red.Printf("%d deleted", deleted)
	fmt.Print(", ")
	if failed > 0 {
		red.Printf("%d failed\n", failed)
	} else {
		fmt.Printf("%d failed\n", failed)
	}
}
