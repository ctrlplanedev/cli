package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewApplyCmd creates a new apply command
func NewApplyCmd() *cobra.Command {
	var filePath string
	var prune bool
	var pruneSelectors []string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration file to create or update resources",
		Long:  `Apply a YAML configuration file to create or update resources in Ctrlplane.`,
		Example: heredoc.Doc(`
			# Apply a single resource file
			$ ctrlc apply -f system.yaml

			# Apply a multi-document file with systems, deployments, and environments
			$ ctrlc apply -f config.yaml

			# Apply and prune resources not in the file
			$ ctrlc apply -f config.yaml --prune

			# Apply and prune only resources with specific metadata
			$ ctrlc apply -f config.yaml --prune --prune-selector managed-by=ctrlc

			# Apply and prune with multiple metadata selectors (AND logic)
			$ ctrlc apply -f config.yaml --prune --prune-selector managed-by=ctrlc --prune-selector env=prod
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), filePath, prune, pruneSelectors)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the YAML configuration file (required)")
	cmd.Flags().BoolVar(&prune, "prune", false, "Delete resources not defined in the YAML file")
	cmd.Flags().StringArrayVar(&pruneSelectors, "prune-selector", nil, "Only prune resources matching metadata key=value (can be repeated, uses AND logic)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// parsePruneSelectors parses key=value selector strings into a map
func parsePruneSelectors(selectors []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, s := range selectors {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid selector format %q, expected key=value", s)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

// matchesMetadata checks if the resource metadata contains all the required selector key-values
func matchesMetadata(metadata map[string]string, selectors map[string]string) bool {
	for key, value := range selectors {
		if metadata[key] != value {
			return false
		}
	}
	return true
}

func runApply(ctx context.Context, filePath string, prune bool, pruneSelectors []string) error {
	// Create API client
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)
	if workspaceID.String() == "00000000-0000-0000-0000-000000000000" {
		return fmt.Errorf("invalid workspace: %s", workspace)
	}

	// Parse prune selectors
	var metadataSelectors map[string]string
	if len(pruneSelectors) > 0 {
		metadataSelectors, err = parsePruneSelectors(pruneSelectors)
		if err != nil {
			return fmt.Errorf("failed to parse prune selectors: %w", err)
		}
	}

	// Parse the YAML file
	documents, err := ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in file")
		return nil
	}

	log.Info("Applying resources", "count", len(documents), "file", filePath, "prune", prune)

	// Create resolver for name-to-ID lookups
	resolver := NewResourceResolver(client, workspaceID.String())

	// Process documents in dependency order:
	// 1. Systems (required by deployments and environments)
	// 2. Deployments (belong to systems)
	// 3. Environments (belong to systems)
	// 4. Policies (reference deployments/environments)
	// 5. RelationshipRules (reference other resources)
	var results []*ApplyResult

	// Order of resource kinds for processing
	kindOrder := []ResourceKind{
		KindSystem,
		KindDeployment,
		KindEnvironment,
		KindResource,
		KindPolicy,
		KindRelationshipRule,
	}

	for _, kind := range kindOrder {
		for _, doc := range documents {
			if doc.Kind == kind {
				result, err := processDocument(ctx, client, workspaceID.String(), resolver, doc)
				if err != nil {
					log.Error("Failed to apply resource", "kind", doc.Kind, "error", err)
				}
				results = append(results, result)
			}
		}
	}

	// Handle pruning if enabled
	var pruneResults []*ApplyResult
	if prune {
		pruneResults, err = pruneResources(ctx, client, workspaceID.String(), resolver, documents, metadataSelectors)
		if err != nil {
			log.Error("Failed to prune resources", "error", err)
		}
	}

	// Print summary
	printResults(results, pruneResults)

	// Check for errors
	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to apply")
		}
	}
	for _, r := range pruneResults {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to prune")
		}
	}

	return nil
}

// pruneResources deletes resources that exist in the workspace but are not defined in the YAML file
// If metadataSelectors is provided, only resources matching ALL selector key-values will be pruned.
// Note: Systems, Deployments, and Environments don't have metadata in the API, so they will be
// skipped when metadata selectors are specified. Only Policies support metadata filtering.
func pruneResources(ctx context.Context, client *api.ClientWithResponses, workspaceID string, resolver *ResourceResolver, documents []ParsedDocument, metadataSelectors map[string]string) ([]*ApplyResult, error) {
	var results []*ApplyResult

	// Build sets of resource names from the YAML file for each type
	yamlSystems := make(map[string]bool)
	yamlDeployments := make(map[string]bool)  // key: systemID/name
	yamlEnvironments := make(map[string]bool) // key: systemID/name
	yamlPolicies := make(map[string]bool)

	for _, doc := range documents {
		fmt.Println("doc", doc.Kind)
		switch doc.Kind {
		case KindSystem:
			sysDoc, err := ParseSystem(doc.Raw)
			if err != nil {
				continue
			}
			yamlSystems[sysDoc.Name] = true

		case KindDeployment:
			depDoc, err := ParseDeployment(doc.Raw)
			if err != nil {
				continue
			}
			// Resolve system ID for the key
			systemID, err := resolver.ResolveSystemID(ctx, depDoc.System)
			if err != nil {
				continue
			}
			yamlDeployments[systemID+"/"+depDoc.Name] = true

		case KindEnvironment:
			envDoc, err := ParseEnvironment(doc.Raw)
			if err != nil {
				continue
			}
			// Resolve system ID for the key
			systemID, err := resolver.ResolveSystemID(ctx, envDoc.System)
			if err != nil {
				continue
			}
			yamlEnvironments[systemID+"/"+envDoc.Name] = true

		case KindPolicy:
			polDoc, err := ParsePolicy(doc.Raw)
			if err != nil {
				continue
			}
			yamlPolicies[polDoc.Name] = true
		}
	}

	// Prune in reverse dependency order: Policies, Environments, Deployments, Systems

	// Prune Policies (supports metadata filtering)
	policyResults := prunePolicies(ctx, client, workspaceID, yamlPolicies, metadataSelectors)
	results = append(results, policyResults...)

	// Skip Systems, Deployments, and Environments when metadata selectors are specified
	// since they don't have metadata in the API
	if len(metadataSelectors) > 0 {
		log.Warn("Metadata selectors specified; skipping prune for Systems, Deployments, and Environments (no metadata support)")
		return results, nil
	}

	// Prune Environments
	envResults := pruneEnvironments(ctx, client, workspaceID, yamlEnvironments)
	results = append(results, envResults...)

	// Prune Deployments
	depResults := pruneDeployments(ctx, client, workspaceID, yamlDeployments)
	results = append(results, depResults...)

	// Prune Systems
	sysResults := pruneSystems(ctx, client, workspaceID, yamlSystems)
	results = append(results, sysResults...)

	return results, nil
}

func pruneSystems(ctx context.Context, client *api.ClientWithResponses, workspaceID string, yamlSystems map[string]bool) []*ApplyResult {
	var results []*ApplyResult

	listResp, err := client.ListSystemsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		return results
	}

	if listResp.JSON200 == nil {
		return results
	}

	for _, sys := range listResp.JSON200.Items {
		if !yamlSystems[sys.Name] {
			// System exists but not in YAML - delete it
			result := &ApplyResult{
				Kind: KindSystem,
				Name: sys.Name,
				ID:   sys.Id,
			}

			resp, err := client.DeleteSystemWithResponse(ctx, workspaceID, sys.Id)
			if err != nil {
				result.Error = fmt.Errorf("failed to delete system: %w", err)
			} else if resp.StatusCode() >= 400 {
				result.Error = fmt.Errorf("failed to delete system: %s", string(resp.Body))
			} else {
				result.Action = "pruned"
			}

			results = append(results, result)
		}
	}

	return results
}

func pruneDeployments(ctx context.Context, client *api.ClientWithResponses, workspaceID string, yamlDeployments map[string]bool) []*ApplyResult {
	var results []*ApplyResult

	listResp, err := client.ListDeploymentsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		return results
	}

	if listResp.JSON200 == nil {
		return results
	}

	for _, dep := range listResp.JSON200.Items {
		key := dep.Deployment.SystemId + "/" + dep.Deployment.Name
		if !yamlDeployments[key] {
			// Deployment exists but not in YAML - delete it
			result := &ApplyResult{
				Kind: KindDeployment,
				Name: dep.Deployment.Name,
				ID:   dep.Deployment.Id,
			}

			resp, err := client.DeleteDeploymentWithResponse(ctx, workspaceID, dep.Deployment.Id)
			if err != nil {
				result.Error = fmt.Errorf("failed to delete deployment: %w", err)
			} else if resp.StatusCode() >= 400 {
				result.Error = fmt.Errorf("failed to delete deployment: %s", string(resp.Body))
			} else {
				result.Action = "pruned"
			}

			results = append(results, result)
		}
	}

	return results
}

func pruneEnvironments(ctx context.Context, client *api.ClientWithResponses, workspaceID string, yamlEnvironments map[string]bool) []*ApplyResult {
	var results []*ApplyResult

	listResp, err := client.ListEnvironmentsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		return results
	}

	if listResp.JSON200 == nil {
		return results
	}

	for _, env := range listResp.JSON200.Items {
		key := env.Environment.SystemId + "/" + env.Environment.Name
		if !yamlEnvironments[key] {
			// Environment exists but not in YAML - delete it
			result := &ApplyResult{
				Kind: KindEnvironment,
				Name: env.Environment.Name,
				ID:   env.Environment.Id,
			}

			resp, err := client.DeleteEnvironmentWithResponse(ctx, workspaceID, env.Environment.Id)
			if err != nil {
				result.Error = fmt.Errorf("failed to delete environment: %w", err)
			} else if resp.StatusCode() >= 400 {
				result.Error = fmt.Errorf("failed to delete environment: %s", string(resp.Body))
			} else {
				result.Action = "pruned"
			}

			results = append(results, result)
		}
	}

	return results
}

func prunePolicies(ctx context.Context, client *api.ClientWithResponses, workspaceID string, yamlPolicies map[string]bool, metadataSelectors map[string]string) []*ApplyResult {
	var results []*ApplyResult

	listResp, err := client.ListPoliciesWithResponse(ctx, workspaceID, nil)
	if err != nil {
		return results
	}

	if listResp.JSON200 == nil {
		return results
	}

	for _, pol := range listResp.JSON200.Items {
		if !yamlPolicies[pol.Name] {
			// If metadata selectors are specified, only prune if the policy matches
			if len(metadataSelectors) > 0 && !matchesMetadata(pol.Metadata, metadataSelectors) {
				continue
			}

			// Policy exists but not in YAML - delete it
			result := &ApplyResult{
				Kind: KindPolicy,
				Name: pol.Name,
				ID:   pol.Id,
			}

			resp, err := client.DeletePolicyWithResponse(ctx, workspaceID, pol.Id)
			if err != nil {
				result.Error = fmt.Errorf("failed to delete policy: %w", err)
			} else if resp.StatusCode() >= 400 {
				result.Error = fmt.Errorf("failed to delete policy: %s", string(resp.Body))
			} else {
				result.Action = "pruned"
			}

			results = append(results, result)
		}
	}

	return results
}

func processDocument(ctx context.Context, client *api.ClientWithResponses, workspaceID string, resolver *ResourceResolver, doc ParsedDocument) (*ApplyResult, error) {
	switch doc.Kind {
	case KindSystem:
		sysDoc, err := ParseSystem(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplySystem(ctx, client, workspaceID, sysDoc)

	case KindDeployment:
		depDoc, err := ParseDeployment(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplyDeployment(ctx, client, workspaceID, resolver, depDoc)

	case KindEnvironment:
		envDoc, err := ParseEnvironment(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplyEnvironment(ctx, client, workspaceID, resolver, envDoc)

	case KindPolicy:
		polDoc, err := ParsePolicy(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplyPolicy(ctx, client, workspaceID, polDoc)

	case KindRelationshipRule:
		relDoc, err := ParseRelationshipRule(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplyRelationshipRule(ctx, client, workspaceID, relDoc)

	case KindResource:
		resDoc, err := ParseResource(doc.Raw)
		if err != nil {
			return &ApplyResult{Kind: doc.Kind, Error: err}, err
		}
		return ApplyResource(ctx, client, workspaceID, resDoc)

	default:
		return &ApplyResult{Kind: doc.Kind, Error: fmt.Errorf("unknown resource kind: %s", doc.Kind)}, nil
	}
}

func printResults(results []*ApplyResult, pruneResults []*ApplyResult) {
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
			fmt.Printf("%s/%s: ", r.Kind, r.Name)
			red.Printf("%v\n", r.Error)
		} else {
			green.Print("✓ ")
			fmt.Printf("%s/", r.Kind)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			dim.Printf("(id: %s)\n", r.ID)
		}
	}

	// Print prune results
	for _, r := range pruneResults {
		if r.Error != nil {
			red.Print("✗ ")
			fmt.Printf("%s/%s: ", r.Kind, r.Name)
			red.Printf("%v\n", r.Error)
		} else {
			green.Print("✓ ")
			fmt.Printf("%s/", r.Kind)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			dim.Printf("(id: %s)\n", r.ID)
		}
	}

	fmt.Println()

	// Count successes and failures
	var success, failed, pruned int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			success++
		}
	}
	for _, r := range pruneResults {
		if r.Error != nil {
			failed++
		} else {
			pruned++
		}
	}

	// Summary with colors
	if len(pruneResults) > 0 {
		fmt.Printf("Applied %d resources: ", len(results)+len(pruneResults))
		green.Printf("%d succeeded", success)
		fmt.Print(", ")
		if failed > 0 {
			red.Printf("%d failed", failed)
		} else {
			fmt.Printf("%d failed", failed)
		}
		fmt.Print(", ")
		yellow.Printf("%d pruned\n", pruned)
	} else {
		fmt.Printf("Applied %d resources: ", len(results))
		green.Printf("%d succeeded", success)
		fmt.Print(", ")
		if failed > 0 {
			red.Printf("%d failed\n", failed)
		} else {
			fmt.Printf("%d failed\n", failed)
		}
	}
}
