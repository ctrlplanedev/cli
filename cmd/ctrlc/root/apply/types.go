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

type Deployment struct {
	Slug             string             `yaml:"slug"`
	Name             string             `yaml:"name"`
	Description      *string            `yaml:"description"`
	JobAgent         *JobAgent          `yaml:"jobAgent,omitempty"`
	ResourceSelector *map[string]any    `yaml:"resourceSelector,omitempty"`
	Metadata         *map[string]string `yaml:"metadata,omitempty"`
}

type JobAgent struct {
	Id     string         `yaml:"id"`
	Config map[string]any `yaml:"config"`
}

type ResourceProvider struct {
	Name      string     `yaml:"name"`
	Resources []Resource `yaml:"resources"`
}

type Resource struct {
	Identifier string            `yaml:"identifier"`
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version"`
	Kind       string            `yaml:"kind"`
	Config     map[string]any    `yaml:"config"`
	Metadata   map[string]string `yaml:"metadata"`
	Variables  map[string]any    `yaml:"variables"`
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
