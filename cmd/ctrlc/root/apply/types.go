package apply

// Config represents the structure of the YAML file
type Config struct {
	Systems       []System               `yaml:"systems"`
	Providers     ResourceProvider       `yaml:"resourceProvider"`
	Relationships []ResourceRelationship `yaml:"relationshipRules"`
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

type DeploymentVariableValue struct {
	Value     *any  `yaml:"value,omitempty"`
	Sensitive *bool `yaml:"sensitive,omitempty"`

	DefaultValue *any      `yaml:"defaultValue,omitempty"`
	Reference    *string   `yaml:"reference,omitempty"`
	Path         *[]string `yaml:"path,omitempty"`

	Default          *bool           `yaml:"default,omitempty"`
	ResourceSelector *map[string]any `yaml:"resourceSelector,omitempty"`
}

type DeploymentVariable struct {
	Key    string                    `yaml:"key"`
	Config map[string]any            `yaml:"config"`
	Description *string               `yaml:"description"`
	Values []DeploymentVariableValue `yaml:"values"`
}

type Deployment struct {
	Slug             string                `yaml:"slug"`
	Name             string                `yaml:"name"`
	Description      *string               `yaml:"description"`
	JobAgent         *JobAgent             `yaml:"jobAgent,omitempty"`
	ResourceSelector *map[string]any       `yaml:"resourceSelector,omitempty"`
	Metadata         *map[string]string    `yaml:"metadata,omitempty"`
	Variables        *[]DeploymentVariable `yaml:"variables,omitempty"`
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

type ResourceRelationship struct {
	Reference         string          `yaml:"reference"`
	Target            *TargetResource `yaml:"target,omitempty"`
	Source            *SourceResource `yaml:"source,omitempty"`
	MetadataKeysMatch []string        `yaml:"metadataKeysMatch"`
	DependencyType    string          `yaml:"dependencyType"`
}
