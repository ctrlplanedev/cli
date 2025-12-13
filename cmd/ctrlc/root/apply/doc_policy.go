package apply

import (
	"fmt"
	"time"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	TypePolicy DocumentType = "Policy"
)

// PolicyTargetSelectorConfig represents a policy target selector in YAML
type PolicyTargetSelectorConfig struct {
	DeploymentSelector  *string `yaml:"deployments,omitempty"`
	EnvironmentSelector *string `yaml:"environments,omitempty"`
	ResourceSelector    *string `yaml:"resources,omitempty"`
}

// PolicyRuleConfig represents a policy rule in YAML
type PolicyRuleConfig struct {
	AnyApproval            *AnyApprovalRuleConfig            `yaml:"anyApproval,omitempty"`
	EnvironmentProgression *EnvironmentProgressionRuleConfig `yaml:"environmentProgression,omitempty"`
	GradualRollout         *GradualRolloutRuleConfig         `yaml:"gradualRollout,omitempty"`
	DeploymentWindow       *DeploymentWindowRuleConfig       `yaml:"deploymentWindow,omitempty"`
	VersionCooldown        *VersionCooldownRuleConfig        `yaml:"versionCooldown,omitempty"`
	Retry                  *RetryRuleConfig                  `yaml:"retry,omitempty"`
	Verification           *VerificationRuleConfig           `yaml:"verification,omitempty"`
}

// RetryRuleConfig represents a retry rule in YAML
type RetryRuleConfig struct {
	MaxRetries int32 `yaml:"maxRetries"`
}

// VersionCooldownRuleConfig represents a version cooldown rule in YAML
type VersionCooldownRuleConfig struct {
	Duration time.Duration `yaml:"duration"`
}

// DeploymentWindowRuleConfig represents a deployment window rule in YAML
type DeploymentWindowRuleConfig struct {
	Allow    *bool         `yaml:"allow,omitempty"`
	Timezone string        `yaml:"timezone"`
	Duration time.Duration `yaml:"duration"`
	RRule    string        `yaml:"rrule"`
}

// AnyApprovalRuleConfig represents an any approval rule
type AnyApprovalRuleConfig struct {
	MinApprovals int32 `yaml:"minApprovals"`
}

// EnvironmentProgressionRuleConfig represents an environment progression rule
type EnvironmentProgressionRuleConfig struct {
	DependsOnEnvironmentSelector SelectorConfig   `yaml:"dependsOnEnvironmentSelector"`
	MaximumAgeHours              *int32           `yaml:"maximumAgeHours,omitempty"`
	MinimumSockTimeMinutes       *int32           `yaml:"minimumSockTimeMinutes,omitempty"`
	MinimumSuccessPercentage     *float32         `yaml:"minimumSuccessPercentage,omitempty"`
	SuccessStatuses              *[]api.JobStatus `yaml:"successStatuses,omitempty"`
}

// GradualRolloutRuleConfig represents a gradual rollout rule
type GradualRolloutRuleConfig struct {
	RolloutType       *api.GradualRolloutRuleRolloutType `yaml:"rolloutType,omitempty"`
	TimeScaleInterval time.Duration                      `yaml:"timeScaleInterval"`
}

// VerificationRuleConfig represents a verification rule in YAML
type VerificationRuleConfig struct {
	TriggerOn *api.VerificationRuleTriggerOn `yaml:"triggerOn,omitempty"`
	Metrics   []VerificationMetricConfig     `yaml:"metrics"`
}

// VerificationMetricConfig represents a verification metric in YAML
type VerificationMetricConfig struct {
	Name             string        `yaml:"name"`
	Interval         time.Duration `yaml:"interval"`
	Count            int32         `yaml:"count"`
	SuccessCondition string        `yaml:"successCondition"`
	SuccessThreshold int32         `yaml:"successThreshold,omitempty"`

	FailureCondition *string `yaml:"failureCondition,omitempty"`
	FailureThreshold int32   `yaml:"failureThreshold"`

	Provider VerificationMetricDatadogConfig `yaml:"provider"`
}

// VerificationMetricDatadogConfig represents the configuration for a Datadog provider
type VerificationMetricDatadogConfig struct {
	Type   string `yaml:"type"`
	Query  string `yaml:"query"`
	ApiKey string `yaml:"apiKey"`
	AppKey string `yaml:"appKey"`
	Site   string `yaml:"site"`
}

// PolicyDocument represents a Policy in YAML
type PolicyDocument struct {
	BaseDocument `yaml:",inline"`
	Description  *string                      `yaml:"description,omitempty"`
	Enabled      *bool                        `yaml:"enabled,omitempty"`
	Priority     *int                         `yaml:"priority,omitempty"`
	Metadata     map[string]string            `yaml:"metadata,omitempty"`
	Selectors    []PolicyTargetSelectorConfig `yaml:"selectors"`
	Rules        []PolicyRuleConfig           `yaml:"rules"`
}

func ParsePolicy(raw []byte) (*PolicyDocument, error) {
	var doc PolicyDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse policy document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("policy document missing required 'name' field")
	}
	return &doc, nil
}

var _ Document = &PolicyDocument{}

func (d *PolicyDocument) Order() int {
	return 500 // Medium priority - policies can reference deployments/environments
}

func (d *PolicyDocument) Apply(ctx *DocContext) (ApplyResult, error) {
	result := ApplyResult{
		Type: TypePolicy,
		Name: d.Name,
	}

	// Find existing policy by name
	listResp, err := ctx.Client.ListPoliciesWithResponse(ctx.Context, ctx.WorkspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list policies: %w", err)
		return result, result.Error
	}

	var existingID string
	matchingCount := 0
	if listResp.JSON200 != nil {
		for _, pol := range listResp.JSON200.Items {
			if pol.Name == d.Name {
				existingID = pol.Id
				matchingCount++
			}
		}
		if matchingCount > 1 {
			result.Error = fmt.Errorf("multiple policies found with name '%s', unable to determine which to update", d.Name)
			return result, result.Error
		}
	}

	policyId := uuid.New().String()
	if existingID != "" {
		policyId = existingID
	}

	// Build selectors (use empty slice, not nil, so JSON marshals to [] not null)
	selectors := make([]api.PolicyTargetSelector, 0)
	for idx, sel := range d.Selectors {
		id := uuid.NewSHA1(uuid.MustParse(policyId), []byte(fmt.Sprintf("selector-%d", idx))).String()
		apiSel := api.PolicyTargetSelector{
			Id: id,
			DeploymentSelector: fromCelSelector("true"),
			EnvironmentSelector: fromCelSelector("true"),
			ResourceSelector: fromCelSelector("true"),
		}
	
		if sel.DeploymentSelector != nil {
			apiSel.DeploymentSelector = fromCelSelector(*sel.DeploymentSelector)
		}
		if sel.EnvironmentSelector != nil {
			apiSel.EnvironmentSelector = fromCelSelector(*sel.EnvironmentSelector)
		}
		if sel.ResourceSelector != nil {
			apiSel.ResourceSelector = fromCelSelector(*sel.ResourceSelector)
		}
		selectors = append(selectors, apiSel)
	}

	rules := make([]api.PolicyRule, 0)
	for idx, rule := range d.Rules {
		apiRule := api.PolicyRule{
			Id:       uuid.NewSHA1(uuid.MustParse(policyId), []byte(fmt.Sprintf("rule-%d", idx))).String(),
			PolicyId: policyId,
		}
		if rule.AnyApproval != nil {
			apiRule.AnyApproval = &api.AnyApprovalRule{
				MinApprovals: rule.AnyApproval.MinApprovals,
			}
		}
		if rule.DeploymentWindow != nil {
			allow := true
			if rule.DeploymentWindow.Allow != nil {
				allow = *rule.DeploymentWindow.Allow
			}
			apiRule.DeploymentWindow = &api.DeploymentWindowRule{
				AllowWindow:     allow,
				Timezone:        &rule.DeploymentWindow.Timezone,
				Rrule:           rule.DeploymentWindow.RRule,
				DurationMinutes: int32(rule.DeploymentWindow.Duration.Minutes()),
			}
		}
		if rule.VersionCooldown != nil {
			apiRule.VersionCooldown = &api.VersionCooldownRule{
				IntervalSeconds: int32(rule.VersionCooldown.Duration.Seconds()),
			}
		}

		if rule.Retry != nil {
			apiRule.Retry = &api.RetryRule{
				MaxRetries: rule.Retry.MaxRetries,
			}
		}

		if rule.Verification != nil {
			apiRule.Verification = &api.VerificationRule{
				Metrics:   make([]api.VerificationMetricSpec, 0),
				TriggerOn: rule.Verification.TriggerOn,
			}
			for _, metric := range rule.Verification.Metrics {
				provider := api.MetricProvider{}
				switch metric.Provider.Type {
				case "datadog":
					site := metric.Provider.Site
					if site == "" {
						site = "datadoghq.com"
					}
					_ = provider.FromDatadogMetricProvider(api.DatadogMetricProvider{
						Type:   api.Datadog,
						Query:  metric.Provider.Query,
						ApiKey: metric.Provider.ApiKey,
						AppKey: metric.Provider.AppKey,
						Site:   &site,
					})
				default:
					result.Error = fmt.Errorf("unsupported metric provider type: %s", metric.Provider.Type)
					return result, result.Error
				}

				metricSpec := api.VerificationMetricSpec{
					Name:             metric.Name,
					IntervalSeconds:  int32(metric.Interval.Seconds()),
					Count:            int(metric.Count),
					SuccessCondition: metric.SuccessCondition,
					Provider:         provider,
				}

				if metric.SuccessThreshold != 0 {
					successThreshold := int(metric.SuccessThreshold)
					metricSpec.SuccessThreshold = &successThreshold
				}

				if metric.FailureThreshold != 0 {
					failureThreshold := int(metric.FailureThreshold)
					metricSpec.FailureThreshold = &failureThreshold
				}

				if metric.FailureCondition != nil {
					metricSpec.FailureCondition = metric.FailureCondition
				}

				apiRule.Verification.Metrics = append(apiRule.Verification.Metrics, metricSpec)
			}
		}
		rules = append(rules, apiRule)
	}

	opts := api.UpsertPolicyJSONRequestBody{
		Name:        d.Name,
		Description: d.Description,
		Priority:    d.Priority,
		Enabled:     d.Enabled,
		Rules:       &rules,
	}

	if d.Metadata != nil {
		opts.Metadata = &d.Metadata
	}

	if d.Selectors != nil {
		opts.Selectors = &selectors
	}

	// Update existing policy
	resp, err := ctx.Client.UpsertPolicyWithResponse(ctx.Context, ctx.WorkspaceID, policyId, opts)
	if err != nil {
		result.Error = fmt.Errorf("failed to update policy: %w", err)
		return result, result.Error
	}

	if resp.JSON202 == nil {
		result.Error = fmt.Errorf("failed to update policy: %s", string(resp.Body))
		return result, result.Error
	}

	result.ID = resp.JSON202.Id
	if existingID != "" {
		result.Action = "updated"
	} else {
		result.Action = "created"
	}

	return result, nil
}

func (d *PolicyDocument) Delete(ctx *DocContext) (DeleteResult, error) {
	result := DeleteResult{
		Type: TypePolicy,
		Name: d.Name,
	}

	// Find the policy by name
	listResp, err := ctx.Client.ListPoliciesWithResponse(ctx.Context, ctx.WorkspaceID, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list policies: %w", err)
		return result, result.Error
	}

	var policyID string
	if listResp.JSON200 != nil {
		for _, pol := range listResp.JSON200.Items {
			if pol.Name == d.Name {
				policyID = pol.Id
				break
			}
		}
	}

	if policyID == "" {
		result.Action = "not_found"
		return result, nil
	}

	resp, err := ctx.Client.DeletePolicyWithResponse(ctx.Context, ctx.WorkspaceID, policyID)
	if err != nil {
		result.Error = fmt.Errorf("failed to delete policy: %w", err)
		return result, result.Error
	}
	if resp.StatusCode() >= 400 {
		result.Error = fmt.Errorf("failed to delete policy: %s", string(resp.Body))
		return result, result.Error
	}

	result.ID = policyID
	result.Action = "deleted"
	return result, nil
}
