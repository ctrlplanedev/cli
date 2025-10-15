package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"strconv"

	"github.com/avast/retry-go"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/telemetry"
	"github.com/hashicorp/go-tfe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	Kind    = "Workspace"
	Version = "terraform/v1"
)

type WorkspaceResource struct {
	Config     map[string]interface{}
	Identifier string
	Kind       string
	Metadata   map[string]string
	Name       string
	Version    string
}

func getLinksMetadata(workspace *tfe.Workspace, baseURL url.URL) *string {
	if workspace.Organization == nil {
		return nil
	}
	links := map[string]string{
		"Terraform Workspace": fmt.Sprintf("%s/app/%s/workspaces/%s", baseURL.String(), workspace.Organization.Name, workspace.Name),
	}
	linksJSON, err := json.Marshal(links)
	if err != nil {
		log.Error("Failed to marshal links", "error", err)
		return nil
	}
	linksString := string(linksJSON)
	return &linksString
}

func getWorkspaceVariables(ctx context.Context, workspace *tfe.Workspace, client *tfe.Client) map[string]string {
	ctx, span := telemetry.StartSpan(ctx, "terraform.get_workspace_variables",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("terraform.workspace_id", workspace.ID),
			attribute.Int("terraform.variables_total", len(workspace.Variables)),
		),
	)
	defer span.End()

	variables := make(map[string]string)
	processedCount := 0

	for _, variable := range workspace.Variables {
		if variable == nil || variable.Sensitive {
			continue
		}

		// Create a child span for each variable read
		varCtx, varSpan := telemetry.StartSpan(ctx, "terraform.read_variable",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("terraform.variable_key", variable.Key),
				attribute.String("terraform.variable_id", variable.ID),
			),
		)

		fetchedVariable, err := client.Variables.Read(varCtx, workspace.ID, variable.ID)
		if err != nil {
			log.Error("Failed to read variable", "error", err, "variable", variable.Key)
			telemetry.SetSpanError(varSpan, err)
			varSpan.End()
			continue
		}

		if fetchedVariable.Category != tfe.CategoryTerraform || fetchedVariable.Sensitive {
			telemetry.AddSpanAttribute(varSpan, "terraform.variable_skipped", true)
			telemetry.AddSpanAttribute(varSpan, "terraform.variable_category", string(fetchedVariable.Category))
			varSpan.End()
			continue
		}

		variables[fetchedVariable.Key] = fetchedVariable.Value
		processedCount++
		telemetry.SetSpanSuccess(varSpan)
		varSpan.End()

		time.Sleep(50 * time.Millisecond)
	}

	telemetry.AddSpanAttribute(span, "terraform.variables_processed", processedCount)
	telemetry.SetSpanSuccess(span)
	return variables
}

func getWorkspaceVcsRepo(workspace *tfe.Workspace) map[string]string {
	vcsRepo := make(map[string]string)
	if workspace.VCSRepo != nil {
		vcsRepo["terraform-cloud/vcs-repo/identifier"] = workspace.VCSRepo.Identifier
		vcsRepo["terraform-cloud/vcs-repo/branch"] = workspace.VCSRepo.Branch
		vcsRepo["terraform-cloud/vcs-repo/repository-http-url"] = workspace.VCSRepo.RepositoryHTTPURL
	}
	return vcsRepo
}

func getWorkspaceTags(workspace *tfe.Workspace) map[string]string {
	tags := make(map[string]string)
	for _, tag := range workspace.Tags {
		if tag != nil {
			key := fmt.Sprintf("terraform-cloud/tag/%s", tag.Name)
			tags[key] = "true"
		}
	}
	return tags
}

func convertWorkspaceToResource(ctx context.Context, workspace *tfe.Workspace, client *tfe.Client) (WorkspaceResource, error) {
	if workspace == nil {
		return WorkspaceResource{}, fmt.Errorf("workspace is nil")
	}
	version := Version
	kind := Kind
	name := workspace.Name
	identifier := workspace.ID
	config := map[string]interface{}{
		"workspaceId": workspace.ID,
	}
	metadata := map[string]string{
		"ctrlplane/external-id":                workspace.ID,
		"terraform-cloud/workspace-name":       workspace.Name,
		"terraform-cloud/workspace-auto-apply": strconv.FormatBool(workspace.AutoApply),
		"terraform/version":                    workspace.TerraformVersion,
	}

	if workspace.Organization != nil {
		metadata["terraform-cloud/organization"] = workspace.Organization.Name
	}

	linksMetadata := getLinksMetadata(workspace, client.BaseURL())
	if linksMetadata != nil {
		metadata["ctrlplane/links"] = *linksMetadata
	}

	moreValues := []map[string]string{
		getWorkspaceVariables(ctx, workspace, client),
		getWorkspaceTags(workspace),
		getWorkspaceVcsRepo(workspace),
	}

	for _, moreValue := range moreValues {
		for key, value := range moreValue {
			metadata[key] = value
		}
	}

	return WorkspaceResource{
		Version:    version,
		Kind:       kind,
		Name:       name,
		Identifier: identifier,
		Config:     config,
		Metadata:   metadata,
	}, nil
}

func listWorkspacesWithRetry(ctx context.Context, client *tfe.Client, organization string, pageNum, pageSize int) (*tfe.WorkspaceList, error) {
	ctx, span := telemetry.StartSpan(ctx, "terraform.list_workspaces",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("terraform.organization", organization),
			attribute.Int("terraform.page_number", pageNum),
			attribute.Int("terraform.page_size", pageSize),
		),
	)
	defer span.End()

	var workspaces *tfe.WorkspaceList
	err := retry.Do(
		func() error {
			var err error
			workspaces, err = client.Workspaces.List(ctx, organization, &tfe.WorkspaceListOptions{
				ListOptions: tfe.ListOptions{
					PageNumber: pageNum,
					PageSize:   pageSize,
				},
			})
			return err
		},
		retry.Attempts(5),
		retry.Delay(time.Second),
		retry.MaxDelay(5*time.Second),
	)

	if err != nil {
		telemetry.SetSpanError(span, err)
	} else {
		telemetry.SetSpanSuccess(span)
		if workspaces != nil {
			telemetry.AddSpanAttribute(span, "terraform.workspaces_count", len(workspaces.Items))
		}
	}

	return workspaces, err
}

func listAllWorkspaces(ctx context.Context, client *tfe.Client, organization string) ([]*tfe.Workspace, error) {
	ctx, span := telemetry.StartSpan(ctx, "terraform.list_all_workspaces",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("terraform.organization", organization),
		),
	)
	defer span.End()

	var allWorkspaces []*tfe.Workspace
	pageNum := 1
	pageSize := 100

	for {
		workspaces, err := listWorkspacesWithRetry(ctx, client, organization, pageNum, pageSize)
		if err != nil {
			telemetry.SetSpanError(span, err)
			return nil, fmt.Errorf("failed to list workspaces: %w", err)
		}

		allWorkspaces = append(allWorkspaces, workspaces.Items...)
		if len(workspaces.Items) < pageSize {
			break
		}
		pageNum++
	}

	telemetry.AddSpanAttribute(span, "terraform.total_workspaces", len(allWorkspaces))
	telemetry.SetSpanSuccess(span)
	return allWorkspaces, nil
}

func getWorkspacesInOrg(ctx context.Context, client *tfe.Client, organization string) ([]WorkspaceResource, error) {
	ctx, span := telemetry.StartSpan(ctx, "terraform.get_workspaces_in_org",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("terraform.organization", organization),
		),
	)
	defer span.End()

	workspaces, err := listAllWorkspaces(ctx, client, organization)
	if err != nil {
		telemetry.SetSpanError(span, err)
		return nil, err
	}

	workspaceResources := []WorkspaceResource{}
	for _, workspace := range workspaces {
		workspaceResource, err := convertWorkspaceToResource(ctx, workspace, client)
		if err != nil {
			log.Error("Failed to convert workspace to resource", "error", err, "workspace", workspace.Name)
			continue
		}
		workspaceResources = append(workspaceResources, workspaceResource)
		time.Sleep(50 * time.Millisecond)
	}

	telemetry.AddSpanAttribute(span, "terraform.workspaces_processed", len(workspaceResources))
	telemetry.SetSpanSuccess(span)
	return workspaceResources, nil
}
