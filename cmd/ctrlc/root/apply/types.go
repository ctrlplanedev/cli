package apply

import "github.com/ctrlplanedev/cli/internal/api"

// ResourceKind represents the type of resource in a YAML document
type ResourceKind string

const (
	KindSystem           ResourceKind = "System"
	KindDeployment       ResourceKind = "Deployment"
	KindEnvironment      ResourceKind = "Environment"
	KindPolicy           ResourceKind = "Policy"
	KindRelationshipRule ResourceKind = "RelationshipRule"
	KindResource         ResourceKind = "Resource"
)

// BaseDocument contains the common fields for all resource documents
type BaseDocument struct {
	Type ResourceKind `yaml:"type"`
	Name string       `yaml:"name"`
}

// SystemDocument represents a System resource in YAML
type SystemDocument struct {
	BaseDocument `yaml:",inline"`
	Description  *string `yaml:"description,omitempty"`
}

// SelectorConfig represents a resource selector in YAML
type SelectorConfig struct {
	Cel  *string        `yaml:"cel,omitempty"`
	Json map[string]any `yaml:"json,omitempty"`
}

// DeploymentDocument represents a Deployment resource in YAML
type DeploymentDocument struct {
	BaseDocument     `yaml:",inline"`
	System           string         `yaml:"system"`
	Slug             *string        `yaml:"slug,omitempty"`
	Description      *string        `yaml:"description,omitempty"`
	ResourceSelector *string        `yaml:"resourceSelector,omitempty"`
	JobAgentId       *string        `yaml:"jobAgentId,omitempty"`
	JobAgentConfig   map[string]any `yaml:"jobAgentConfig,omitempty"`
}

// EnvironmentDocument represents an Environment resource in YAML
type EnvironmentDocument struct {
	BaseDocument     `yaml:",inline"`
	System           string          `yaml:"system"`
	Description      *string         `yaml:"description,omitempty"`
	ResourceSelector *SelectorConfig `yaml:"resourceSelector,omitempty"`
}

// PolicyTargetSelectorConfig represents a policy target selector in YAML
type PolicyTargetSelectorConfig struct {
	DeploymentSelector  *SelectorConfig `yaml:"deploymentSelector,omitempty"`
	EnvironmentSelector *SelectorConfig `yaml:"environmentSelector,omitempty"`
	ResourceSelector    *SelectorConfig `yaml:"resourceSelector,omitempty"`
}

// PolicyRuleConfig represents a policy rule in YAML
type PolicyRuleConfig struct {
	AnyApproval            *AnyApprovalRuleConfig            `yaml:"anyApproval,omitempty"`
	EnvironmentProgression *EnvironmentProgressionRuleConfig `yaml:"environmentProgression,omitempty"`
	GradualRollout         *GradualRolloutRuleConfig         `yaml:"gradualRollout,omitempty"`
	DeploymentWindow       *DeploymentWindowRuleConfig       `yaml:"deploymentWindow,omitempty"`
}

// DeploymentWindowRuleConfig represents a deployment window rule in YAML
type DeploymentWindowRuleConfig struct {
	Timezone string `yaml:"timezone"`
	Duration string `yaml:"duration"`
	RRule    string `yaml:"rrule"`
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
	TimeScaleInterval int32 `yaml:"timeScaleInterval"`
}

// PolicyDocument represents a Policy resource in YAML
type PolicyDocument struct {
	BaseDocument `yaml:",inline"`
	Description  *string                      `yaml:"description,omitempty"`
	Enabled      *bool                        `yaml:"enabled,omitempty"`
	Priority     *int                         `yaml:"priority,omitempty"`
	Metadata     map[string]string            `yaml:"metadata,omitempty"`
	Selectors    []PolicyTargetSelectorConfig `yaml:"selectors"`
	Rules        []PolicyRuleConfig           `yaml:"rules"`
}

// RelationshipRuleDocument represents a RelationshipRule resource in YAML
type RelationshipRuleDocument struct {
	Kind             ResourceKind            `yaml:"kind"`
	Name             string                  `yaml:"name"`
	Reference        string                  `yaml:"reference"`
	RelationshipType string                  `yaml:"relationshipType"`
	FromType         api.RelatableEntityType `yaml:"fromType"`
	ToType           api.RelatableEntityType `yaml:"toType"`
	Description      *string                 `yaml:"description,omitempty"`
	Metadata         map[string]string       `yaml:"metadata,omitempty"`
	FromSelector     *SelectorConfig         `yaml:"fromSelector,omitempty"`
	ToSelector       *SelectorConfig         `yaml:"toSelector,omitempty"`
	Matcher          *MatcherConfig          `yaml:"matcher,omitempty"`
}

// MatcherConfig represents a matcher for relationship rules
type MatcherConfig struct {
	Cel *string `yaml:"cel,omitempty"`
}

// ResourceDocument represents a Resource in YAML
type ResourceDocument struct {
	BaseDocument `yaml:",inline"`
	Identifier   string            `yaml:"identifier"`          // Unique identifier for the resource
	Kind         string            `yaml:"kind"`                // The kind of resource (e.g., "Cluster", "Namespace")
	Version      string            `yaml:"version"`             // Version string for the resource
	Config       map[string]any    `yaml:"config,omitempty"`    // Arbitrary configuration
	Metadata     map[string]string `yaml:"metadata,omitempty"`  // Key-value metadata
	Variables    map[string]any    `yaml:"variables,omitempty"` // Key-value variables
	Provider     string            `yaml:"provider,omitempty"`  // Optional: Name of the resource provider
}

// ApplyResult represents the result of applying a resource
type ApplyResult struct {
	Kind   ResourceKind
	Name   string
	Action string // "created", "updated", "unchanged"
	ID     string
	Error  error
}
