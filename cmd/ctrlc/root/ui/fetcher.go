package ui

import (
	"context"
	"fmt"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
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

const timeLayout = "2006-01-02 15:04"

// --- top-level fetchers ---

func fetchData(client *api.Client, workspaceID string, rt resourceType) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		limit := int32(100)

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

func fetchDeployments(ctx context.Context, client *api.Client, workspaceID string, limit int32) dataMsg {
	resp, err := client.Deployment.ListDeployments(ctx, connect.NewRequest(&apiv1.ListDeploymentsRequest{
		WorkspaceId: workspaceID,
		Limit:       limit,
	}))
	if err != nil {
		return dataMsg{err: err}
	}

	items := resp.Msg.GetItems()
	rows := make([]tableRow, 0, len(items))
	for _, item := range items {
		dep := item.GetDeployment()
		desc := dep.GetDescription()
		// Get system names (now plural)
		systemNames := ""
		systems := item.GetSystems()
		if len(systems) > 0 {
			systemNames = systems[0].GetName()
			for i := 1; i < len(systems); i++ {
				systemNames += ", " + systems[i].GetName()
			}
		}
		rows = append(rows, tableRow{
			id:      dep.GetId(),
			cols:    []string{dep.GetName(), systemNames, dep.GetSlug(), desc},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
}

func fetchResources(ctx context.Context, client *api.Client, workspaceID string, limit int32) dataMsg {
	return fetchResourcesWithFilter(ctx, client, workspaceID, limit, "")
}

func fetchResourcesWithFilter(ctx context.Context, client *api.Client, workspaceID string, limit int32, filter string) dataMsg {
	req := &apiv1.ListResourcesRequest{
		WorkspaceId: workspaceID,
		Page:        &apiv1.Page{Limit: limit},
	}
	if filter != "" {
		cel := fmt.Sprintf("resource.name.contains('%s')", filter)
		req.Selector = &cel
	}

	resp, err := client.Resource.ListResources(ctx, connect.NewRequest(req))
	if err != nil {
		return dataMsg{err: err}
	}

	items := resp.Msg.GetItems()
	rows := make([]tableRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, tableRow{
			id:      item.GetIdentifier(),
			cols:    []string{item.GetName(), item.GetKind(), item.GetVersion(), item.GetIdentifier()},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
}

// fetchResourcesFiltered returns a tea.Cmd that fetches resources with server-side CEL filter
func fetchResourcesFiltered(client *api.Client, workspaceID string, filter string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		return fetchResourcesWithFilter(ctx, client, workspaceID, 100, filter)
	}
}

func fetchJobs(ctx context.Context, client *api.Client, workspaceID string, limit int32) dataMsg {
	resp, err := client.Job.ListJobs(ctx, connect.NewRequest(&apiv1.ListJobsRequest{
		WorkspaceId: workspaceID,
		Limit:       limit,
	}))
	if err != nil {
		return dataMsg{err: err}
	}

	items := resp.Msg.GetItems()
	rows := make([]tableRow, 0, len(items))
	for _, job := range items {
		id := job.GetId()
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows = append(rows, tableRow{
			id:      id,
			cols:    []string{shortID, job.GetStatus(), job.GetReleaseId(), "", "", job.GetCreatedAt().AsTime().Format(timeLayout)},
			rawItem: job,
		})
	}
	return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
}

func fetchEnvironments(ctx context.Context, client *api.Client, workspaceID string, limit int32) dataMsg {
	resp, err := client.System.ListEnvironments(ctx, connect.NewRequest(&apiv1.ListEnvironmentsRequest{
		WorkspaceId: workspaceID,
		Page:        &apiv1.Page{Limit: limit},
	}))
	if err != nil {
		return dataMsg{err: err}
	}

	items := resp.Msg.GetItems()
	rows := make([]tableRow, 0, len(items))
	for _, item := range items {
		id := item.GetId()
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows = append(rows, tableRow{
			id:      id,
			cols:    []string{item.GetName(), item.GetDescription(), shortID, item.GetCreatedAt().AsTime().Format(timeLayout)},
			rawItem: item,
		})
	}
	return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
}

func fetchVersions(ctx context.Context, client *api.Client, workspaceID string, limit int32) dataMsg {
	depResp, err := client.Deployment.ListDeployments(ctx, connect.NewRequest(&apiv1.ListDeploymentsRequest{
		WorkspaceId: workspaceID,
		Limit:       limit,
	}))
	if err != nil {
		return dataMsg{err: err}
	}

	var rows []tableRow
	total := 0

	versionLimit := int32(20)
	for _, dep := range depResp.Msg.GetItems() {
		deployment := dep.GetDeployment()
		resp, err := client.Deployment.ListDeploymentVersions(ctx, connect.NewRequest(&apiv1.ListDeploymentVersionsRequest{
			WorkspaceId:  workspaceID,
			DeploymentId: deployment.GetId(),
			Limit:        versionLimit,
		}))
		if err != nil {
			continue
		}
		total += int(resp.Msg.GetTotal())
		for _, item := range resp.Msg.GetItems() {
			rows = append(rows, tableRow{
				id:      item.GetId(),
				cols:    []string{item.GetTag(), deployment.GetName(), item.GetStatus(), item.GetName(), item.GetCreatedAt().AsTime().Format(timeLayout)},
				rawItem: item,
			})
		}
	}

	return dataMsg{rows: rows, total: total}
}

// --- drill-down fetchers ---

// fetchJobsForDeployment fetches all jobs and filters by deployment ID.
// NOTE: the Connect ListJobs RPC returns bare jobs without joined deployment
// metadata, so jobs are filtered server-side by deployment id.
func fetchJobsForDeployment(client *api.Client, workspaceID string, deploymentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resp, err := client.Job.ListJobs(ctx, connect.NewRequest(&apiv1.ListJobsRequest{
			WorkspaceId:  workspaceID,
			Limit:        100,
			DeploymentId: &deploymentID,
		}))
		if err != nil {
			return dataMsg{err: err}
		}

		var rows []tableRow
		for _, job := range resp.Msg.GetItems() {
			id := job.GetId()
			shortID := id
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			rows = append(rows, tableRow{
				id:      id,
				cols:    []string{shortID, job.GetStatus(), "", "", job.GetCreatedAt().AsTime().Format(timeLayout)},
				rawItem: job,
			})
		}
		return dataMsg{rows: rows, total: len(rows)}
	}
}

// fetchDeploymentsForResource fetches deployments associated with a resource
func fetchDeploymentsForResource(client *api.Client, workspaceID string, resourceIdentifier string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resp, err := client.Resource.ListResourceDeployments(ctx, connect.NewRequest(&apiv1.ListResourceDeploymentsRequest{
			WorkspaceId:        workspaceID,
			ResourceIdentifier: resourceIdentifier,
			Limit:              100,
		}))
		if err != nil {
			return dataMsg{err: err}
		}

		items := resp.Msg.GetItems()
		rows := make([]tableRow, 0, len(items))
		for _, dep := range items {
			// Join system IDs (now plural)
			systemIds := ""
			rows = append(rows, tableRow{
				id:      dep.GetId(),
				cols:    []string{dep.GetName(), dep.GetSlug(), systemIds, dep.GetDescription()},
				rawItem: dep,
			})
		}
		return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
	}
}

// fetchVersionsForDeployment fetches versions for a specific deployment
func fetchVersionsForDeployment(client *api.Client, workspaceID string, deploymentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resp, err := client.Deployment.ListDeploymentVersions(ctx, connect.NewRequest(&apiv1.ListDeploymentVersionsRequest{
			WorkspaceId:  workspaceID,
			DeploymentId: deploymentID,
			Limit:        100,
		}))
		if err != nil {
			return dataMsg{err: err}
		}

		items := resp.Msg.GetItems()
		rows := make([]tableRow, 0, len(items))
		for _, item := range items {
			rows = append(rows, tableRow{
				id:      item.GetId(),
				cols:    []string{item.GetTag(), item.GetStatus(), item.GetName(), item.GetCreatedAt().AsTime().Format(timeLayout)},
				rawItem: item,
			})
		}
		return dataMsg{rows: rows, total: int(resp.Msg.GetTotal())}
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
		return []string{"ID", "STATUS", "RELEASE", "ENVIRONMENT", "RESOURCE", "CREATED"}
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
