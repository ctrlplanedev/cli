package apply

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func processAllPolicies(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID string,
	policies []Policy,
) {
	if len(policies) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, policy := range policies {
		wg.Add(1)
		policy.WorkspaceId = workspaceID
		go processPolicy(ctx, client, policy, &wg)
	}
	wg.Wait()
}

func processPolicy(
	ctx context.Context,
	client *api.ClientWithResponses,
	policy Policy,
	policyWg *sync.WaitGroup,
) {
	defer policyWg.Done()

	if policy.Name == "" {
		log.Error("Policy name is required", "policy", policy)
		return
	}

	body := createPolicyRequestBody(policy)
	if _, err := upsertPolicy(ctx, client, body); err != nil {
		log.Error("Failed to create policy", "name", policy.Name, "error", err)
		return
	}
}

func createPolicyRequestBody(policy Policy) api.UpsertPolicyJSONRequestBody {
	// Convert targets
	targets := make([]api.PolicyTarget, len(policy.Targets))
	for i, target := range policy.Targets {
		targets[i] = api.PolicyTarget{
			DeploymentSelector:  target.DeploymentSelector,
			EnvironmentSelector: target.EnvironmentSelector,
			ResourceSelector:    target.ResourceSelector,
		}
	}

	// Convert deny windows
	denyWindows := make([]struct {
		Dtend    *time.Time              `json:"dtend,omitempty"`
		Rrule    *map[string]interface{} `json:"rrule,omitempty"`
		TimeZone string                  `json:"timeZone"`
	}, len(policy.DenyWindows))
	for i, window := range policy.DenyWindows {
		rrule := window.Rrule
		denyWindows[i] = struct {
			Dtend    *time.Time              `json:"dtend,omitempty"`
			Rrule    *map[string]interface{} `json:"rrule,omitempty"`
			TimeZone string                  `json:"timeZone"`
		}{
			Dtend:    window.Dtend,
			Rrule:    &rrule,
			TimeZone: window.TimeZone,
		}
	}

	// Convert version any approval
	var versionAnyApprovals *api.VersionAnyApproval
	if policy.VersionAnyApprovals != nil {
		versionAnyApprovals = &api.VersionAnyApproval{
			RequiredApprovalsCount: policy.VersionAnyApprovals.RequiredApprovalsCount,
		}
	}

	versionUserApprovals := make([]api.VersionUserApproval, len(policy.VersionUserApprovals))
	for i, approval := range policy.VersionUserApprovals {
		versionUserApprovals[i] = api.VersionUserApproval{
			UserId: approval.UserId,
		}
	}

	versionRoleApprovals := make([]api.VersionRoleApproval, len(policy.VersionRoleApprovals))
	for i, approval := range policy.VersionRoleApprovals {
		count := approval.RequiredApprovalsCount
		versionRoleApprovals[i] = api.VersionRoleApproval{
			RequiredApprovalsCount: count,
			RoleId:                 approval.RoleId,
		}
	}

	// Create deployment version selector if present
	var deploymentVersionSelector *api.DeploymentVersionSelector
	if policy.DeploymentVersionSelector != nil {
		deploymentVersionSelector = &api.DeploymentVersionSelector{
			Name:                      policy.DeploymentVersionSelector.Name,
			DeploymentVersionSelector: policy.DeploymentVersionSelector.DeploymentVersionSelector,
			Description:               policy.DeploymentVersionSelector.Description,
		}
	}

	var concurrency *api.PolicyConcurrency
	if policy.Concurrency != nil {
		floatConcurrency := api.PolicyConcurrency(*policy.Concurrency)
		concurrency = &floatConcurrency
	}

	var environmentVersionRollout *api.InsertEnvironmentVersionRollout
	if policy.EnvironmentVersionRollout != nil {
		var rolloutType *api.InsertEnvironmentVersionRolloutRolloutType
		if policy.EnvironmentVersionRollout.RolloutType != nil {
			parsedRolloutType := *policy.EnvironmentVersionRollout.RolloutType
			rolloutTypeCasted := api.InsertEnvironmentVersionRolloutRolloutType(parsedRolloutType)
			rolloutType = &rolloutTypeCasted
		}
		environmentVersionRollout = &api.InsertEnvironmentVersionRollout{
			PositionGrowthFactor: policy.EnvironmentVersionRollout.PositionGrowthFactor,
			TimeScaleInterval:    policy.EnvironmentVersionRollout.TimeScaleInterval,
			RolloutType:          rolloutType,
		}
	}

	return api.UpsertPolicyJSONRequestBody{
		Name:                      policy.Name,
		Description:               policy.Description,
		Priority:                  policy.Priority,
		Enabled:                   policy.Enabled,
		WorkspaceId:               policy.WorkspaceId,
		Targets:                   targets,
		DenyWindows:               &denyWindows,
		DeploymentVersionSelector: deploymentVersionSelector,
		VersionAnyApprovals:       versionAnyApprovals,
		VersionUserApprovals:      &versionUserApprovals,
		VersionRoleApprovals:      &versionRoleApprovals,
		Concurrency:               concurrency,
		EnvironmentVersionRollout: environmentVersionRollout,
	}
}

func upsertPolicy(
	ctx context.Context,
	client *api.ClientWithResponses,
	policy api.UpsertPolicyJSONRequestBody,
) (string, error) {
	resp, err := client.UpsertPolicyWithResponse(ctx, policy)

	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("API returned error status: %d", resp.StatusCode())
	}

	if resp.JSON200 != nil {
		return resp.JSON200.Id.String(), nil
	}

	return "", fmt.Errorf("unexpected response format")
}
