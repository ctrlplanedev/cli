package apply

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ParsedDocument represents a parsed YAML document with its kind identified
type ParsedDocument struct {
	Kind ResourceKind
	Raw  []byte
}

// ParseFile reads a YAML file and returns parsed documents
func ParseFile(filePath string) ([]ParsedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseYAML(data)
}

// ParseYAML parses multi-document YAML and returns parsed documents
func ParseYAML(data []byte) ([]ParsedDocument, error) {
	var documents []ParsedDocument

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var raw map[string]any
		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode YAML document: %w", err)
		}

		// Skip empty documents
		if len(raw) == 0 {
			continue
		}

		// Get the type field
		typeVal, ok := raw["type"]
		if !ok {
			return nil, fmt.Errorf("document missing required 'type' field")
		}

		kindStr, ok := typeVal.(string)
		if !ok {
			return nil, fmt.Errorf("'type' field must be a string")
		}

		kind := ResourceKind(kindStr)
		if !isValidKind(kind) {
			return nil, fmt.Errorf("unknown resource kind: %s", kindStr)
		}

		// Re-encode to bytes for later parsing into specific types
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to re-encode document: %w", err)
		}

		documents = append(documents, ParsedDocument{
			Kind: kind,
			Raw:  rawBytes,
		})
	}

	return documents, nil
}

// isValidKind checks if the given kind is a supported resource kind
func isValidKind(kind ResourceKind) bool {
	switch kind {
	case KindSystem, KindDeployment, KindEnvironment, KindPolicy, KindRelationshipRule, KindResource:
		return true
	default:
		return false
	}
}

// ParseSystem parses a raw document into a SystemDocument
func ParseSystem(raw []byte) (*SystemDocument, error) {
	var doc SystemDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse system document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("system document missing required 'name' field")
	}
	return &doc, nil
}

// ParseDeployment parses a raw document into a DeploymentDocument
func ParseDeployment(raw []byte) (*DeploymentDocument, error) {
	var doc DeploymentDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse deployment document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("deployment document missing required 'name' field")
	}
	return &doc, nil
}

// ParseEnvironment parses a raw document into an EnvironmentDocument
func ParseEnvironment(raw []byte) (*EnvironmentDocument, error) {
	var doc EnvironmentDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse environment document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("environment document missing required 'name' field")
	}
	return &doc, nil
}

// ParsePolicy parses a raw document into a PolicyDocument
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

// ParseRelationshipRule parses a raw document into a RelationshipRuleDocument
func ParseRelationshipRule(raw []byte) (*RelationshipRuleDocument, error) {
	var doc RelationshipRuleDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse relationship rule document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("relationship rule document missing required 'name' field")
	}
	if doc.Reference == "" {
		return nil, fmt.Errorf("relationship rule document missing required 'reference' field")
	}
	if doc.RelationshipType == "" {
		return nil, fmt.Errorf("relationship rule document missing required 'relationshipType' field")
	}
	return &doc, nil
}

// ParseResource parses a raw document into a ResourceDocument
func ParseResource(raw []byte) (*ResourceDocument, error) {
	var doc ResourceDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse resource document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("resource document missing required 'name' field")
	}
	if doc.Identifier == "" {
		return nil, fmt.Errorf("resource document missing required 'identifier' field")
	}
	if doc.Kind == "" {
		return nil, fmt.Errorf("resource document missing required 'kind' field")
	}
	if doc.Version == "" {
		return nil, fmt.Errorf("resource document missing required 'version' field")
	}
	return &doc, nil
}
