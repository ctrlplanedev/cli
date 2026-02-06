package providers

import (
	"fmt"
	"time"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const policyTypeName = "Policy"

type PolicyProvider struct{}

func init() {
	RegisterProvider(&PolicyProvider{})
}

func (p *PolicyProvider) TypeName() string {
	return policyTypeName
}

func (p *PolicyProvider) Order() int {
	return 500
}

func (p *PolicyProvider) Parse(raw []byte) (ResourceSpec, error) {
	var spec PolicySpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse policy document: %w", err)
	}
	if spec.DisplayName == "" {
		return nil, fmt.Errorf("policy document missing required 'name' field")
	}
	return &spec, nil
}

type PolicyTargetSelectorConfig struct {
	DeploymentSelector  *string `yaml:"deployments,omitempty"`
	EnvironmentSelector *string `yaml:"environments,omitempty"`
	ResourceSelector    *string `yaml:"resources,omitempty"`
}

type PolicyRuleConfig struct {
	AnyApproval            *AnyApprovalRuleConfig            `yaml:"anyApproval,omitempty"`
	EnvironmentProgression *EnvironmentProgressionRuleConfig `yaml:"environmentProgression,omitempty"`
	GradualRollout         *GradualRolloutRuleConfig         `yaml:"gradualRollout,omitempty"`
	DeploymentWindow       *DeploymentWindowRuleConfig       `yaml:"deploymentWindow,omitempty"`
	VersionCooldown        *VersionCooldownRuleConfig        `yaml:"versionCooldown,omitempty"`
	Retry                  *RetryRuleConfig                  `yaml:"retry,omitempty"`
	Verification           *VerificationRuleConfig           `yaml:"verification,omitempty"`
}

type RetryRuleConfig struct {
	MaxRetries int32 `yaml:"maxRetries"`
}

type VersionCooldownRuleConfig struct {
	Duration time.Duration `yaml:"duration"`
}

type DeploymentWindowRuleConfig struct {
	Allow    *bool         `yaml:"allow,omitempty"`
	Timezone string        `yaml:"timezone"`
	Duration time.Duration `yaml:"duration"`
	RRule    string        `yaml:"rrule"`
}

type AnyApprovalRuleConfig struct {
	MinApprovals int32 `yaml:"minApprovals"`
}

type EnvironmentProgressionRuleConfig struct {
	DependsOnEnvironmentSelector SelectorConfig   `yaml:"dependsOnEnvironmentSelector"`
	MaximumAgeHours              *int32           `yaml:"maximumAgeHours,omitempty"`
	MinimumSockTimeMinutes       *int32           `yaml:"minimumSockTimeMinutes,omitempty"`
	MinimumSuccessPercentage     *float32         `yaml:"minimumSuccessPercentage,omitempty"`
	SuccessStatuses              *[]api.JobStatus `yaml:"successStatuses,omitempty"`
}

type GradualRolloutRuleConfig struct {
	RolloutType       *api.GradualRolloutRuleRolloutType `yaml:"rolloutType,omitempty"`
	TimeScaleInterval time.Duration                      `yaml:"timeScaleInterval"`
}

type VerificationRuleConfig struct {
	TriggerOn *api.VerificationRuleTriggerOn `yaml:"triggerOn,omitempty"`
	Metrics   []VerificationMetricConfig     `yaml:"metrics"`
}

type VerificationMetricConditionConfig struct {
	Condition string `yaml:"condition"`
	Threshold int32  `yaml:"threshold"`
}

type VerificationMetricConfig struct {
	Name     string        `yaml:"name"`
	Interval time.Duration `yaml:"interval"`
	Count    int32         `yaml:"count"`

	Failure VerificationMetricConditionConfig `yaml:"failure,omitempty"`
	Success VerificationMetricConditionConfig `yaml:"success,omitempty"`

	Provider MetricProviderConfig `yaml:"provider"`
}

type MetricProviderConfig struct {
	Type string `yaml:"type"` // "datadog", "sleep"

	Aggregator *string           `yaml:"aggregator,omitempty"`
	Queries    map[string]string `yaml:"queries,omitempty"`
	ApiKey     string            `yaml:"apiKey,omitempty"`
	AppKey     string            `yaml:"appKey,omitempty"`
	Site       string            `yaml:"site,omitempty"`
	Formula    *string           `yaml:"formula,omitempty"`
	Interval   *time.Duration    `yaml:"interval,omitempty"`

	Duration time.Duration `yaml:"duration,omitempty"`
}

type PolicySpec struct {
	Type        string                       `yaml:"type,omitempty"`
	DisplayName string                       `yaml:"name"`
	Description *string                      `yaml:"description,omitempty"`
	Enabled     *bool                        `yaml:"enabled,omitempty"`
	Priority    *int                         `yaml:"priority,omitempty"`
	Metadata    map[string]string            `yaml:"metadata,omitempty"`
	Selectors   []PolicyTargetSelectorConfig `yaml:"selectors"`
	Rules       []PolicyRuleConfig           `yaml:"rules"`
}

func (p *PolicySpec) Name() string {
	return p.DisplayName
}

func (p *PolicySpec) Identity() string {
	return p.DisplayName
}

func (p *PolicySpec) Lookup(ctx Context) (string, error) {
	listResp, err := ctx.APIClient().ListPoliciesWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to list policies: %w", err)
	}

	var existingID string
	matchingCount := 0
	if listResp.JSON200 != nil {
		for _, pol := range listResp.JSON200.Items {
			if pol.Name == p.DisplayName {
				existingID = pol.Id
				matchingCount++
			}
		}
	}

	if matchingCount > 1 {
		return "", fmt.Errorf("multiple policies found with name '%s', unable to determine which to update", p.DisplayName)
	}

	return existingID, nil
}

func (p *PolicySpec) Create(ctx Context, id string) error {
	return p.upsert(ctx, id)
}

func (p *PolicySpec) Update(ctx Context, existingID string) error {
	return p.upsert(ctx, existingID)
}

func (p *PolicySpec) Delete(ctx Context, existingID string) error {
	resp, err := ctx.APIClient().RequestPolicyDeletionWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), existingID)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return fmt.Errorf("failed to delete policy: %s", string(resp.Body))
	}
	return nil
}

func (p *PolicySpec) upsert(ctx Context, policyID string) error {
	selectors, err := buildPolicySelectors(p.Selectors, policyID)
	if err != nil {
		return err
	}

	rules, err := buildPolicyRules(p.Rules, policyID)
	if err != nil {
		return err
	}

	upsertReq := api.RequestPolicyCreationJSONRequestBody{
		Name:        p.DisplayName,
		Description: p.Description,
		Priority:    p.Priority,
		Enabled:     p.Enabled,
		Rules:       &rules,
	}

	if p.Metadata != nil {
		upsertReq.Metadata = &p.Metadata
	}

	if p.Selectors != nil {
		upsertReq.Selectors = &selectors
	}

	resp, err := ctx.APIClient().RequestPolicyCreationWithResponse(ctx.Ctx(), ctx.WorkspaceIDValue(), upsertReq)
	if err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}

	if resp.JSON202 == nil {
		return fmt.Errorf("failed to update policy: %s", string(resp.Body))
	}

	return nil
}

func buildPolicySelectors(selectors []PolicyTargetSelectorConfig, policyID string) ([]api.PolicyTargetSelector, error) {
	apiSelectors := make([]api.PolicyTargetSelector, 0)
	for idx, sel := range selectors {
		id := uuid.NewSHA1(uuid.MustParse(policyID), []byte(fmt.Sprintf("selector-%d", idx))).String()
		apiSel := api.PolicyTargetSelector{
			Id:                  id,
			DeploymentSelector:  fromCelSelector("true"),
			EnvironmentSelector: fromCelSelector("true"),
			ResourceSelector:    fromCelSelector("true"),
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
		apiSelectors = append(apiSelectors, apiSel)
	}
	return apiSelectors, nil
}

func buildPolicyRules(rules []PolicyRuleConfig, policyID string) ([]api.PolicyRule, error) {
	apiRules := make([]api.PolicyRule, 0)
	for idx, rule := range rules {
		apiRule := api.PolicyRule{
			Id:       uuid.NewSHA1(uuid.MustParse(policyID), []byte(fmt.Sprintf("rule-%d", idx))).String(),
			PolicyId: policyID,
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

		if rule.GradualRollout != nil {
			rolloutType := api.GradualRolloutRuleRolloutTypeLinear
			if rule.GradualRollout.RolloutType != nil {
				rolloutType = *rule.GradualRollout.RolloutType
			}
			apiRule.GradualRollout = &api.GradualRolloutRule{
				RolloutType:       rolloutType,
				TimeScaleInterval: int32(rule.GradualRollout.TimeScaleInterval.Seconds()),
			}
		}

		if rule.EnvironmentProgression != nil {
			selector, err := buildSelector(&rule.EnvironmentProgression.DependsOnEnvironmentSelector)
			if err != nil {
				return nil, fmt.Errorf("failed to build environment progression selector: %w", err)
			}
			if selector == nil {
				return nil, fmt.Errorf("environment progression rule requires a dependsOnEnvironmentSelector")
			}
			apiRule.EnvironmentProgression = &api.EnvironmentProgressionRule{
				DependsOnEnvironmentSelector: *selector,
				MaximumAgeHours:              rule.EnvironmentProgression.MaximumAgeHours,
				MinimumSockTimeMinutes:       rule.EnvironmentProgression.MinimumSockTimeMinutes,
				MinimumSuccessPercentage:     rule.EnvironmentProgression.MinimumSuccessPercentage,
				SuccessStatuses:              rule.EnvironmentProgression.SuccessStatuses,
			}
		}

		if rule.Verification != nil {
			apiRule.Verification = &api.VerificationRule{
				Metrics:   make([]api.VerificationMetricSpec, 0),
				TriggerOn: rule.Verification.TriggerOn,
			}
			for _, metric := range rule.Verification.Metrics {
				if metric.Success.Condition == "" {
					return nil, fmt.Errorf("verification metric '%s' missing required 'success.condition' field", metric.Name)
				}

				provider := api.MetricProvider{}
				switch metric.Provider.Type {
				case "datadog":
					site := metric.Provider.Site
					if site == "" {
						site = "datadoghq.com"
					}
					cfg := api.DatadogMetricProvider{
						Type:    api.Datadog,
						ApiKey:  metric.Provider.ApiKey,
						AppKey:  metric.Provider.AppKey,
						Queries: metric.Provider.Queries,
						Formula: metric.Provider.Formula,
						Site:    &site,
					}
					if metric.Provider.Aggregator != nil {
						ag := api.DatadogMetricProviderAggregator(*metric.Provider.Aggregator)
						cfg.Aggregator = &ag
					}
					if metric.Provider.Interval != nil {
						interval := int64(metric.Provider.Interval.Seconds())
						cfg.IntervalSeconds = &interval
					}
					_ = provider.FromDatadogMetricProvider(cfg)
				case "sleep":
					_ = provider.FromSleepMetricProvider(api.SleepMetricProvider{
						Type:            api.Sleep,
						DurationSeconds: int32(metric.Provider.Duration.Seconds()),
					})
				default:
					return nil, fmt.Errorf("unsupported metric provider type: %s", metric.Provider.Type)
				}

				metricSpec := api.VerificationMetricSpec{
					Name:             metric.Name,
					IntervalSeconds:  int32(metric.Interval.Seconds()),
					Count:            int(metric.Count),
					SuccessCondition: metric.Success.Condition,
					Provider:         provider,
				}

				if metric.Success.Threshold != 0 {
					successThreshold := int(metric.Success.Threshold)
					metricSpec.SuccessThreshold = &successThreshold
				}

				if metric.Failure.Threshold != 0 {
					failureThreshold := int(metric.Failure.Threshold)
					metricSpec.FailureThreshold = &failureThreshold
				}

				if metric.Failure.Condition != "" {
					metricSpec.FailureCondition = &metric.Failure.Condition
				}

				apiRule.Verification.Metrics = append(apiRule.Verification.Metrics, metricSpec)
			}
		}
		apiRules = append(apiRules, apiRule)
	}
	return apiRules, nil
}

type SelectorConfig struct {
	Cel  *string        `yaml:"cel,omitempty"`
	Json map[string]any `yaml:"json,omitempty"`
}

func buildSelector(cfg *SelectorConfig) (*api.Selector, error) {
	if cfg == nil {
		return nil, nil
	}

	var selector api.Selector

	if cfg.Cel != nil {
		if err := selector.FromCelSelector(api.CelSelector{Cel: *cfg.Cel}); err != nil {
			return nil, fmt.Errorf("failed to create CEL selector: %w", err)
		}
		return &selector, nil
	}

	if cfg.Json != nil {
		if err := selector.FromJsonSelector(api.JsonSelector{Json: cfg.Json}); err != nil {
			return nil, fmt.Errorf("failed to create JSON selector: %w", err)
		}
		return &selector, nil
	}

	return nil, nil
}

func fromCelSelector(cel string) *api.Selector {
	var selector api.Selector
	selector.FromCelSelector(api.CelSelector{Cel: cel})
	return &selector
}
