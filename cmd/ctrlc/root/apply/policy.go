package apply

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/spf13/viper"
)

func processAllPolicies(
	ctx context.Context,
	client *api.ClientWithResponses,
	policies []Policy,
) {
	if len(policies) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, policy := range policies {
		wg.Add(1)
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

	body := createPolicyRequestBody(policy)
	if _, err := upsertPolicy(ctx, client, body); err != nil {
		log.Error("Failed to create policy", "name", policy.Name, "error", err)
		return
	}
}

func createPolicyRequestBody(policy Policy) api.UpsertPolicyJSONRequestBody {
	workspace := viper.GetString("workspace")
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
		rrule := window.RRule
		denyWindows[i] = struct {
			Dtend    *time.Time              `json:"dtend,omitempty"`
			Rrule    *map[string]interface{} `json:"rrule,omitempty"`
			TimeZone string                  `json:"timeZone"`
		}{
			Dtend:    window.DtEnd,
			Rrule:    &rrule,
			TimeZone: window.TimeZone,
		}
	}

	// Convert version any approval
	var versionAnyApprovals api.VersionAnyApproval
	if policy.VersionAnyApprovals != nil {
		versionAnyApprovals = api.VersionAnyApproval{
			RequiredApprovalsCount: policy.VersionAnyApprovals.RequiredApprovalsCount,
		}
	}

	versionUserApprovals := make([]api.VersionUserApproval, len(policy.VersionUserApprovals))
	for i, approval := range policy.VersionUserApprovals {
		versionUserApprovals[i] = api.VersionUserApproval{
			UserId: approval.UserId,
		}
	}

	versionRoleApprovals := make([]struct {
		RequiredApprovalsCount *float32 `json:"requiredApprovalsCount,omitempty"`
		RoleId                 string   `json:"roleId"`
	}, len(policy.VersionRoleApprovals))
	for i, approval := range policy.VersionRoleApprovals {
		count := approval.RequiredApprovalsCount
		versionRoleApprovals[i] = struct {
			RequiredApprovalsCount *float32 `json:"requiredApprovalsCount,omitempty"`
			RoleId                 string   `json:"roleId"`
		}{
			RequiredApprovalsCount: &count,
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

	return api.UpsertPolicyJSONRequestBody{
		Name:                      policy.Name,
		Description:               policy.Description,
		Priority:                  policy.Priority,
		Enabled:                   policy.Enabled,
		WorkspaceId:               workspace,
		Targets:                   targets,
		DenyWindows:               &denyWindows,
		DeploymentVersionSelector: deploymentVersionSelector,
		VersionAnyApprovals:       &versionAnyApprovals,
		VersionUserApprovals:      &versionUserApprovals,
		VersionRoleApprovals:      &versionRoleApprovals,
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
