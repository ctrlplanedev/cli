package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ctrlplanedev/cli/internal/api"
)

// resourceType identifies which resource view to show
type resourceType int

const (
	resourceTypeDeployments resourceType = iota
	resourceTypeResources
	resourceTypeJobs
	resourceTypeEnvironments
	resourceTypeVersions
)

func (r resourceType) String() string {
	switch r {
	case resourceTypeDeployments:
		return "Deployments"
	case resourceTypeResources:
		return "Resources"
	case resourceTypeJobs:
		return "Jobs"
	case resourceTypeEnvironments:
		return "Environments"
	case resourceTypeVersions:
		return "Deployment Versions"
	default:
		return "Unknown"
	}
}

// --- tea messages ---

type dataMsg struct {
	rows  []tableRow
	total int
	err   error
}

type tableRow struct {
	id      string      // unique ID for drill-down
	cols    []string    // display columns
	rawItem interface{} // original API object
}

// --- drillContext carries parent info for drill-down ---

type drillContext struct {
	deploymentID       string
	deploymentName     string
	resourceIdentifier string
	resourceName       string
}

// --- top-level fetchers ---

func fetchData(client *api.ClientWithResponses, workspaceID string, rt resourceType) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		limit := 100

		switch rt {
		case resourceTypeDeployments:
			return fetchDeployments(ctx, client, workspaceID, limit)
		case resourceTypeResources:
			return fetchResources(ctx, client, workspaceID, limit)
		case resourceTypeJobs:
			return fetchJobs(ctx, client, workspaceID, limit)
		case resourceTypeEnvironments:
			return fetchEnvironments(ctx, client, workspaceID, limit)
		case resourceTypeVersions:
			return fetchVersions(ctx, client, workspaceID, limit)
		default:
			return dataMsg{err: fmt.Errorf("unknown resource type")}
		}
	}
}

func fetchDeployments(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int) dataMsg {
	resp, err := client.ListDeploymentsWithResponse(ctx, workspaceID, &api.ListDeploymentsParams{Limit: &limit})
	if err != nil {
		return dataMsg{err: err}
	}
	if resp.JSON200 == nil {
		return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
	}

	rows := make([]tableRow, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		desc := ""
		if item.Deployment.Description != nil {
			desc = *item.Deployment.Description
		}
		// Get system names (now plural)
		systemNames := ""
		if len(item.Systems) > 0 {
			systemNames = item.Systems[0].Name
			for i := 1; i < len(item.Systems); i++ {
				systemNames += ", " + item.Systems[i].Name
			}
		}
		rows = append(rows, tableRow{
			id:      item.Deployment.Id,
			cols:    []string{item.Deployment.Name, systemNames, item.Deployment.Slug, desc},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: resp.JSON200.Total}
}

func fetchResources(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int) dataMsg {
	return fetchResourcesWithFilter(ctx, client, workspaceID, limit, "")
}

func fetchResourcesWithFilter(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int, filter string) dataMsg {
	params := &api.GetAllResourcesParams{Limit: &limit}
	if filter != "" {
		cel := fmt.Sprintf("resource.name.contains('%s')", filter)
		params.Cel = &cel
	}

	resp, err := client.GetAllResourcesWithResponse(ctx, workspaceID, params)
	if err != nil {
		return dataMsg{err: err}
	}
	if resp.JSON200 == nil {
		return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
	}

	rows := make([]tableRow, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		rows = append(rows, tableRow{
			id:      item.Identifier,
			cols:    []string{item.Name, item.Kind, item.Version, item.Identifier},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: resp.JSON200.Total}
}

// fetchResourcesFiltered returns a tea.Cmd that fetches resources with server-side CEL filter
func fetchResourcesFiltered(client *api.ClientWithResponses, workspaceID string, filter string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		return fetchResourcesWithFilter(ctx, client, workspaceID, 100, filter)
	}
}

func fetchJobs(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int) dataMsg {
	resp, err := client.GetJobsWithResponse(ctx, workspaceID, &api.GetJobsParams{Limit: &limit})
	if err != nil {
		return dataMsg{err: err}
	}
	if resp.JSON200 == nil {
		return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
	}

	rows := make([]tableRow, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		depName := ""
		if item.Deployment != nil {
			depName = item.Deployment.Name
		}
		envName := ""
		if item.Environment != nil {
			envName = item.Environment.Name
		}
		resName := ""
		if item.Resource != nil {
			resName = item.Resource.Name
		}
		rows = append(rows, tableRow{
			id:      item.Job.Id,
			cols:    []string{item.Job.Id[:8], string(item.Job.Status), depName, envName, resName, item.Job.CreatedAt.Format("2006-01-02 15:04")},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: resp.JSON200.Total}
}

func fetchEnvironments(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int) dataMsg {
	resp, err := client.ListEnvironmentsWithResponse(ctx, workspaceID, &api.ListEnvironmentsParams{Limit: &limit})
	if err != nil {
		return dataMsg{err: err}
	}
	if resp.JSON200 == nil {
		return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
	}

	rows := make([]tableRow, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		desc := ""
		if item.Description != nil {
			desc = *item.Description
		}
		rows = append(rows, tableRow{
			id:      item.Id,
			cols:    []string{item.Name, desc, item.Id[:8], item.CreatedAt.Format("2006-01-02 15:04")},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: resp.JSON200.Total}
}

func fetchVersions(ctx context.Context, client *api.ClientWithResponses, workspaceID string, limit int) dataMsg {
	depResp, err := client.ListDeploymentsWithResponse(ctx, workspaceID, &api.ListDeploymentsParams{Limit: &limit})
	if err != nil {
		return dataMsg{err: err}
	}
	if depResp.JSON200 == nil {
		return dataMsg{err: fmt.Errorf("unexpected response: %d", depResp.HTTPResponse.StatusCode)}
	}

	var rows []tableRow
	total := 0

	versionLimit := 20
	for _, dep := range depResp.JSON200.Items {
		resp, err := client.ListDeploymentVersionsWithResponse(ctx, workspaceID, dep.Deployment.Id, &api.ListDeploymentVersionsParams{Limit: &versionLimit})
		if err != nil {
			continue
		}
		if resp.JSON200 == nil {
			continue
		}
		total += resp.JSON200.Total
		for _, item := range resp.JSON200.Items {
			rows = append(rows, tableRow{
				id:      item.Id,
				cols:    []string{item.Tag, dep.Deployment.Name, string(item.Status), item.Name, item.CreatedAt.Format("2006-01-02 15:04")},
				rawItem: item,
			})
		}
	}

	return dataMsg{rows: rows, total: total}
}

// --- drill-down fetchers ---

// fetchJobsForDeployment fetches all jobs and filters by deployment ID
func fetchJobsForDeployment(client *api.ClientWithResponses, workspaceID string, deploymentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		limit := 100
		resp, err := client.GetJobsWithResponse(ctx, workspaceID, &api.GetJobsParams{Limit: &limit})
		if err != nil {
			return dataMsg{err: err}
		}
		if resp.JSON200 == nil {
			return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
		}

		var rows []tableRow
		for _, item := range resp.JSON200.Items {
			if item.Deployment == nil || item.Deployment.Id != deploymentID {
				continue
			}
			envName := ""
			if item.Environment != nil {
				envName = item.Environment.Name
			}
			resName := ""
			if item.Resource != nil {
				resName = item.Resource.Name
			}
			rows = append(rows, tableRow{
				id:      item.Job.Id,
				cols:    []string{item.Job.Id[:8], string(item.Job.Status), envName, resName, item.Job.CreatedAt.Format("2006-01-02 15:04")},
				rawItem: item,
			})
		}
		return dataMsg{rows: rows, total: len(rows)}
	}
}

// fetchDeploymentsForResource fetches deployments associated with a resource
func fetchDeploymentsForResource(client *api.ClientWithResponses, workspaceID string, resourceIdentifier string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		limit := 100
		resp, err := client.GetDeploymentsForResourceWithResponse(ctx, workspaceID, resourceIdentifier, &api.GetDeploymentsForResourceParams{Limit: &limit})
		if err != nil {
			return dataMsg{err: err}
		}
		if resp.JSON200 == nil {
			return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
		}

		rows := make([]tableRow, 0, len(resp.JSON200.Items))
		for _, dep := range resp.JSON200.Items {
			desc := ""
			if dep.Description != nil {
				desc = *dep.Description
			}
			// Join system IDs (now plural)
			systemIds := ""
			if len(dep.SystemIds) > 0 {
				systemIds = dep.SystemIds[0]
				for i := 1; i < len(dep.SystemIds); i++ {
					systemIds += ", " + dep.SystemIds[i]
				}
			}
			rows = append(rows, tableRow{
				id:      dep.Id,
				cols:    []string{dep.Name, dep.Slug, systemIds, desc},
				rawItem: dep,
			})
		}
		return dataMsg{rows: rows, total: resp.JSON200.Total}
	}
}

// fetchVersionsForDeployment fetches versions for a specific deployment
func fetchVersionsForDeployment(client *api.ClientWithResponses, workspaceID string, deploymentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		limit := 100
		resp, err := client.ListDeploymentVersionsWithResponse(ctx, workspaceID, deploymentID, &api.ListDeploymentVersionsParams{Limit: &limit})
		if err != nil {
			return dataMsg{err: err}
		}
		if resp.JSON200 == nil {
			return dataMsg{err: fmt.Errorf("unexpected response: %d", resp.HTTPResponse.StatusCode)}
		}

		rows := make([]tableRow, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			rows = append(rows, tableRow{
				id:      item.Id,
				cols:    []string{item.Tag, string(item.Status), item.Name, item.CreatedAt.Format("2006-01-02 15:04")},
				rawItem: item,
			})
		}
		return dataMsg{rows: rows, total: resp.JSON200.Total}
	}
}

// columnsForResource returns the column headers for each resource type
func columnsForResource(rt resourceType) []string {
	switch rt {
	case resourceTypeDeployments:
		return []string{"NAME", "SYSTEM", "SLUG", "DESCRIPTION"}
	case resourceTypeResources:
		return []string{"NAME", "KIND", "VERSION", "IDENTIFIER"}
	case resourceTypeJobs:
		return []string{"ID", "STATUS", "DEPLOYMENT", "ENVIRONMENT", "RESOURCE", "CREATED"}
	case resourceTypeEnvironments:
		return []string{"NAME", "DESCRIPTION", "ID", "CREATED"}
	case resourceTypeVersions:
		return []string{"TAG", "DEPLOYMENT", "STATUS", "NAME", "CREATED"}
	default:
		return []string{"NAME"}
	}
}

// columnsForDrillDown returns columns for drill-down sub-views
func columnsForDrillDown(kind string) []string {
	switch kind {
	case "deployment-jobs":
		return []string{"ID", "STATUS", "ENVIRONMENT", "RESOURCE", "CREATED"}
	case "deployment-versions":
		return []string{"TAG", "STATUS", "NAME", "CREATED"}
	case "resource-deployments":
		return []string{"NAME", "SLUG", "SYSTEM", "DESCRIPTION"}
	default:
		return []string{"NAME"}
	}
}
