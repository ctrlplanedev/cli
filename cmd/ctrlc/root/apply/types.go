package apply

import "time"

// Config represents the structure of the YAML file
type Config struct {
	Systems       []System               `yaml:"systems"`
	Providers     ResourceProvider       `yaml:"resourceProvider"`
	Relationships []ResourceRelationship `yaml:"relationshipRules"`
	Policies      []Policy               `yaml:"policies,omitempty"`
}

type System struct {
	Slug         string        `yaml:"slug"`
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description"`
	Deployments  []Deployment  `yaml:"deployments"`
	Environments []Environment `yaml:"environments"`
}

type Environment struct {
	Name             string             `yaml:"name"`
	Description      *string            `yaml:"description,omitempty"`
	ResourceSelector *map[string]any    `yaml:"resourceSelector,omitempty"`
	Metadata         *map[string]string `yaml:"metadata,omitempty"`
}

type DirectDeploymentVariableValue struct {
	Value     any   `yaml:"value"`
	Sensitive *bool `yaml:"sensitive,omitempty"`

	IsDefault        *bool           `yaml:"isDefault,omitempty"`
	ResourceSelector *map[string]any `yaml:"resourceSelector,omitempty"`
}

type ReferenceDeploymentVariableValue struct {
	Reference    string   `yaml:"reference"`
	Path         []string `yaml:"path"`
	DefaultValue *any     `yaml:"defaultValue,omitempty"`

	IsDefault        *bool           `yaml:"isDefault,omitempty"`
	ResourceSelector *map[string]any `yaml:"resourceSelector,omitempty"`
}

type DeploymentVariable struct {
	Key             string                             `yaml:"key"`
	Config          map[string]any                     `yaml:"config"`
	Description     *string                            `yaml:"description"`
	DirectValues    []DirectDeploymentVariableValue    `yaml:"directValues"`
	ReferenceValues []ReferenceDeploymentVariableValue `yaml:"referenceValues"`
}

type ExitHook struct {
	Name     string    `yaml:"name"`
	JobAgent *JobAgent `yaml:"jobAgent"`
}

type Deployment struct {
	Slug             string                `yaml:"slug"`
	Name             string                `yaml:"name"`
	Description      *string               `yaml:"description"`
	JobAgent         *JobAgent             `yaml:"jobAgent,omitempty"`
	ResourceSelector *map[string]any       `yaml:"resourceSelector,omitempty"`
	Metadata         *map[string]string    `yaml:"metadata,omitempty"`
	Variables        *[]DeploymentVariable `yaml:"variables,omitempty"`
	ExitHooks        *[]ExitHook           `yaml:"exitHooks,omitempty"`
}

type JobAgent struct {
	Id     string         `yaml:"id"`
	Config map[string]any `yaml:"config"`
}

type ResourceProvider struct {
	Name      string     `yaml:"name"`
	Resources []Resource `yaml:"resources"`
}

type Variable struct {
	DefaultValue *any      `yaml:"defaultValue,omitempty"`
	Reference    *string   `yaml:"reference,omitempty"`
	Path         *[]string `yaml:"path,omitempty"`

	Key       string `yaml:"key"`
	Sensitive *bool  `yaml:"sensitive,omitempty"`
	Value     *any   `yaml:"value,omitempty"`
}

type Resource struct {
	Identifier string            `yaml:"identifier"`
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version"`
	Kind       string            `yaml:"kind"`
	Config     map[string]any    `yaml:"config"`
	Metadata   map[string]string `yaml:"metadata"`
	Variables  *[]Variable       `yaml:"variables,omitempty"`
}

type TargetResource struct {
	Kind           string            `yaml:"kind"`
	Version        string            `yaml:"version"`
	MetadataEquals map[string]string `yaml:"metadataEquals"`
}

type SourceResource struct {
	Kind    string `yaml:"kind"`
	Version string `yaml:"version"`
}

type MetadataKeysMatch struct {
	Key       *string `yaml:"key,omitempty"`
	SourceKey *string `yaml:"sourceKey,omitempty"`
	TargetKey *string `yaml:"targetKey,omitempty"`
}

type ResourceRelationship struct {
	Reference         string              `yaml:"reference"`
	Target            *TargetResource     `yaml:"target,omitempty"`
	Source            *SourceResource     `yaml:"source,omitempty"`
	MetadataKeysMatch []MetadataKeysMatch `yaml:"metadataKeysMatch"`
	DependencyType    string              `yaml:"dependencyType"`
}

// Policy structs
type Policy struct {
	Name                      string                     `yaml:"name"`
	Description               *string                    `yaml:"description,omitempty"`
	Priority                  *float32                   `yaml:"priority,omitempty"`
	Enabled                   *bool                      `yaml:"enabled,omitempty"`
	WorkspaceId               string                     `yaml:"workspaceId"`
	Targets                   []PolicyTarget             `yaml:"targets"`
	DenyWindows               []DenyWindow               `yaml:"denyWindows,omitempty"`
	DeploymentVersionSelector *DeploymentVersionSelector `yaml:"deploymentVersionSelector,omitempty"`
	VersionAnyApprovals       *VersionAnyApproval        `yaml:"versionAnyApprovals,omitempty"`
	VersionUserApprovals      []VersionUserApproval      `yaml:"versionUserApprovals,omitempty"`
	VersionRoleApprovals      []VersionRoleApproval      `yaml:"versionRoleApprovals,omitempty"`
	Concurrency               *int                       `yaml:"concurrency,omitempty"`
	EnvironmentVersionRollout *EnvironmentVersionRollout `yaml:"environmentVersionRollout,omitempty"`
}

type PolicyTarget struct {
	DeploymentSelector  *map[string]any `yaml:"deploymentSelector,omitempty"`
	EnvironmentSelector *map[string]any `yaml:"environmentSelector,omitempty"`
	ResourceSelector    *map[string]any `yaml:"resourceSelector,omitempty"`
}

type DenyWindow struct {
	TimeZone string         `yaml:"timeZone"`
	Rrule    map[string]any `yaml:"rrule"`
	Dtend    *time.Time     `yaml:"dtend,omitempty"`
}

type DeploymentVersionSelector struct {
	Name                      string         `yaml:"name"`
	DeploymentVersionSelector map[string]any `yaml:"deploymentVersionSelector"`
	Description               *string        `yaml:"description,omitempty"`
}

type VersionAnyApproval struct {
	RequiredApprovalsCount float32 `yaml:"requiredApprovalsCount"`
}

type VersionUserApproval struct {
	UserId string `yaml:"userId"`
}

type VersionRoleApproval struct {
	RoleId                 string  `yaml:"roleId"`
	RequiredApprovalsCount float32 `yaml:"requiredApprovalsCount"`
}

type EnvironmentVersionRollout struct {
	PositionGrowthFactor *float32 `yaml:"positionGrowthFactor,omitempty"`
	TimeScaleInterval    float32  `yaml:"timeScaleInterval"`
	RolloutType          *string  `yaml:"rolloutType,omitempty"`
}
