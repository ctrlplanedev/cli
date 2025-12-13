package apply

// SelectorConfig represents a resource selector in YAML
type SelectorConfig struct {
	Cel  *string        `yaml:"cel,omitempty"`
	Json map[string]any `yaml:"json,omitempty"`
}

// ApplyResult represents the result of applying a document
type ApplyResult struct {
	Type   DocumentType
	Name   string
	Action string // "created", "updated", "unchanged", "upserted"
	ID     string
	Error  error
}
