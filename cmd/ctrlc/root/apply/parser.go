package apply

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFile reads a YAML file and returns parsed documents
func ParseFile(filePath string) ([]Document, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseYAML(data)
}

// ParseYAML parses multi-document YAML and returns parsed documents
func ParseYAML(data []byte) ([]Document, error) {
	var documents []Document

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

		typeStr, ok := typeVal.(string)
		if !ok {
			return nil, fmt.Errorf("'type' field must be a string")
		}

		docType := DocumentType(typeStr)

		// Re-encode to bytes for parsing into specific types
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to re-encode document: %w", err)
		}

		// Parse into specific document type
		doc, err := parseDocument(docType, rawBytes)
		if err != nil {
			return nil, err
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

// parseDocument parses raw bytes into the appropriate Document type
func parseDocument(docType DocumentType, raw []byte) (Document, error) {
	switch docType {
	case TypeJobAgent:
		return ParseJobAgent(raw)
	case TypeDeployment:
		return ParseDeployment(raw)
	case TypeEnvironment:
		return ParseEnvironment(raw)
	case TypeResource:
		return ParseResource(raw)
	case TypePolicy:
		return ParsePolicy(raw)
	default:
		return nil, fmt.Errorf("unknown document type: %s", docType)
	}
}
