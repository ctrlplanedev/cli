package apply

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DeleteResult represents the result of deleting a resource
type DeleteResult struct {
	Kind   ResourceKind
	Name   string
	Action string // "deleted", "not_found"
	ID     string
	Error  error
}

// NewDeleteCmd creates a new delete command
func NewDeleteCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources defined in a YAML configuration file",
		Long:  `Delete resources defined in a YAML configuration file from Ctrlplane.`,
		Example: heredoc.Doc(`
			# Delete resources defined in a file
			$ ctrlc delete -f system.yaml

			# Delete resources from a multi-document file
			$ ctrlc delete -f config.yaml
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), filePath)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the YAML configuration file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runDelete(ctx context.Context, filePath string) error {
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

	// Parse the YAML file
	documents, err := ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in file")
		return nil
	}

	log.Info("Deleting resources", "count", len(documents), "file", filePath)

	// Create resolver for name-to-ID lookups
	resolver := NewResourceResolver(client, workspaceID.String())

	// Process documents in reverse order of dependencies
	// Delete in order: RelationshipRules, Policies, Environments, Deployments, Systems
	var results []*DeleteResult

	// Delete RelationshipRules first
	for _, doc := range documents {
		if doc.Kind == KindRelationshipRule {
			result := deleteDocument(ctx, client, workspaceID.String(), resolver, doc)
			results = append(results, result)
		}
	}

	// Delete Policies
	for _, doc := range documents {
		if doc.Kind == KindPolicy {
			result := deleteDocument(ctx, client, workspaceID.String(), resolver, doc)
			results = append(results, result)
		}
	}

	// Delete Environments
	for _, doc := range documents {
		if doc.Kind == KindEnvironment {
			result := deleteDocument(ctx, client, workspaceID.String(), resolver, doc)
			results = append(results, result)
		}
	}

	// Delete Deployments
	for _, doc := range documents {
		if doc.Kind == KindDeployment {
			result := deleteDocument(ctx, client, workspaceID.String(), resolver, doc)
			results = append(results, result)
		}
	}

	// Delete Systems last
	for _, doc := range documents {
		if doc.Kind == KindSystem {
			result := deleteDocument(ctx, client, workspaceID.String(), resolver, doc)
			results = append(results, result)
		}
	}

	// Print summary
	printDeleteResults(results)

	// Check for errors
	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to delete")
		}
	}

	return nil
}

func deleteDocument(ctx context.Context, client *api.ClientWithResponses, workspaceID string, resolver *ResourceResolver, doc ParsedDocument) *DeleteResult {
	switch doc.Kind {
	case KindSystem:
		sysDoc, err := ParseSystem(doc.Raw)
		if err != nil {
			return &DeleteResult{Kind: doc.Kind, Error: err}
		}
		return DeleteSystem(ctx, client, workspaceID, sysDoc)

	case KindDeployment:
		depDoc, err := ParseDeployment(doc.Raw)
		if err != nil {
			return &DeleteResult{Kind: doc.Kind, Error: err}
		}
		return DeleteDeployment(ctx, client, workspaceID, resolver, depDoc)

	case KindEnvironment:
		envDoc, err := ParseEnvironment(doc.Raw)
		if err != nil {
			return &DeleteResult{Kind: doc.Kind, Error: err}
		}
		return DeleteEnvironment(ctx, client, workspaceID, resolver, envDoc)

	case KindPolicy:
		polDoc, err := ParsePolicy(doc.Raw)
		if err != nil {
			return &DeleteResult{Kind: doc.Kind, Error: err}
		}
		return DeletePolicy(ctx, client, workspaceID, polDoc)

	case KindRelationshipRule:
		relDoc, err := ParseRelationshipRule(doc.Raw)
		if err != nil {
			return &DeleteResult{Kind: doc.Kind, Error: err}
		}
		return DeleteRelationshipRule(ctx, client, workspaceID, relDoc)

	default:
		return &DeleteResult{Kind: doc.Kind, Error: fmt.Errorf("unknown resource kind: %s", doc.Kind)}
	}
}

// DeleteSystem deletes a system by name
func DeleteSystem(ctx context.Context, client *api.ClientWithResponses, workspaceID string, doc *SystemDocument) *DeleteResult {
	result := &DeleteResult{
		Kind: KindSystem,
		Name: doc.Name,
	}

	// Find system by name
	listResp, err := client.ListSystemsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list systems: %w", err)
		return result
	}

	var systemID string
	if listResp.JSON200 != nil {
		for _, sys := range listResp.JSON200.Items {
			if sys.Name == doc.Name {
				systemID = sys.Id
				break
			}
		}
	}

	if systemID == "" {
		result.Action = "not_found"
		return result
	}

	// Delete the system
	resp, err := client.DeleteSystemWithResponse(ctx, workspaceID, systemID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete system: %w", err)
		return result
	}

	if resp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete system: %s", string(resp.Body))
		return result
	}

	result.ID = systemID
	result.Action = "deleted"
	return result
}

// DeleteDeployment deletes a deployment by name
func DeleteDeployment(ctx context.Context, client *api.ClientWithResponses, workspaceID string, resolver *ResourceResolver, doc *DeploymentDocument) *DeleteResult {
	result := &DeleteResult{
		Kind: KindDeployment,
		Name: doc.Name,
	}

	// Resolve system ID
	systemID, err := resolver.ResolveSystemID(ctx, doc.System)
	if err != nil {
		result.Error = err
		return result
	}

	// Find deployment by name
	listResp, err := client.ListDeploymentsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list deployments: %w", err)
		return result
	}

	targetSlug := getSlug(doc.Slug, doc.Name)

	var deploymentID string
	if listResp.JSON200 != nil {
		for _, dep := range listResp.JSON200.Items {
			if dep.Deployment.Slug == targetSlug && dep.Deployment.SystemId == systemID {
				deploymentID = dep.Deployment.Id
				break
			}
		}
	}

	if deploymentID == "" {
		result.Action = "not_found"
		return result
	}

	// Delete the deployment
	resp, err := client.DeleteDeploymentWithResponse(ctx, workspaceID, deploymentID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete deployment: %w", err)
		return result
	}

	if resp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete deployment: %s", string(resp.Body))
		return result
	}

	result.ID = deploymentID
	result.Action = "deleted"
	return result
}

// DeleteEnvironment deletes an environment by name
func DeleteEnvironment(ctx context.Context, client *api.ClientWithResponses, workspaceID string, resolver *ResourceResolver, doc *EnvironmentDocument) *DeleteResult {
	result := &DeleteResult{
		Kind: KindEnvironment,
		Name: doc.Name,
	}

	// Resolve system ID
	systemID, err := resolver.ResolveSystemID(ctx, doc.System)
	if err != nil {
		result.Error = err
		return result
	}

	// Find environment by name
	listResp, err := client.ListEnvironmentsWithResponse(ctx, workspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list environments: %w", err)
		return result
	}

	var environmentID string
	if listResp.JSON200 != nil {
		for _, env := range listResp.JSON200.Items {
			if env.Environment.Name == doc.Name && env.Environment.SystemId == systemID {
				environmentID = env.Environment.Id
				break
			}
		}
	}

	if environmentID == "" {
		result.Action = "not_found"
		return result
	}

	// Delete the environment
	resp, err := client.DeleteEnvironmentWithResponse(ctx, workspaceID, environmentID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete environment: %w", err)
		return result
	}

	if resp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete environment: %s", string(resp.Body))
		return result
	}

	result.ID = environmentID
	result.Action = "deleted"
	return result
}

// DeletePolicy deletes a policy by name
func DeletePolicy(ctx context.Context, client *api.ClientWithResponses, workspaceID string, doc *PolicyDocument) *DeleteResult {
	result := &DeleteResult{
		Kind: KindPolicy,
		Name: doc.Name,
	}

	// Find policy by name
	listResp, err := client.ListPoliciesWithResponse(ctx, workspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list policies: %w", err)
		return result
	}

	var policyID string
	if listResp.JSON200 != nil {
		for _, pol := range listResp.JSON200.Items {
			if pol.Name == doc.Name {
				policyID = pol.Id
				break
			}
		}
	}

	if policyID == "" {
		result.Action = "not_found"
		return result
	}

	// Delete the policy
	resp, err := client.DeletePolicyWithResponse(ctx, workspaceID, policyID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete policy: %w", err)
		return result
	}

	if resp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete policy: %s", string(resp.Body))
		return result
	}

	result.ID = policyID
	result.Action = "deleted"
	return result
}

// DeleteRelationshipRule deletes a relationship rule by reference
func DeleteRelationshipRule(ctx context.Context, client *api.ClientWithResponses, workspaceID string, doc *RelationshipRuleDocument) *DeleteResult {
	result := &DeleteResult{
		Kind: KindRelationshipRule,
		Name: doc.Name,
	}

	// Note: There's no list endpoint for relationship rules in the API,
	// so we need the ID. For now, we'll return not_found if we don't have the ID.
	// In practice, users would need to track the ID separately or we'd need to
	// add support for storing IDs after creation.
	result.Action = "not_found"
	result.Error = fmt.Errorf("relationship rule deletion requires the resource ID; consider storing IDs after creation")
	return result
}

func printDeleteResults(results []*DeleteResult) {
	fmt.Println()
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("✗ %s/%s: %v\n", r.Kind, r.Name, r.Error)
		} else if r.Action == "not_found" {
			fmt.Printf("- %s/%s not found (skipped)\n", r.Kind, r.Name)
		} else {
			fmt.Printf("✓ %s/%s %s (id: %s)\n", r.Kind, r.Name, r.Action, r.ID)
		}
	}
	fmt.Println()

	// Count successes and failures
	var success, failed, notFound int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else if r.Action == "not_found" {
			notFound++
		} else {
			success++
		}
	}
	fmt.Printf("Deleted %d resources: %d succeeded, %d not found, %d failed\n", len(results), success, notFound, failed)
}
