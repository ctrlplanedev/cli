package applyv2

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ctrlplanedev/cli/internal/api/providers"
	"gopkg.in/yaml.v3"
)

// ParseFile reads a YAML file and returns parsed specs using the provider framework.
func ParseFile(filePath string) ([]providers.TypedSpec, error) {
	data, err := readFileOrURL(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseYAML(data)
}

// ParseYAML parses multi-document YAML and returns typed specs.
func ParseYAML(data []byte) ([]providers.TypedSpec, error) {
	var specs []providers.TypedSpec

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

		if len(raw) == 0 {
			continue
		}

		typeVal, ok := raw["type"]
		if !ok {
			return nil, fmt.Errorf("document missing required 'type' field")
		}

		typeStr, ok := typeVal.(string)
		if !ok {
			return nil, fmt.Errorf("'type' field must be a string")
		}

		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to re-encode document: %w", err)
		}

		spec, err := providers.DefaultProviderEngine.Parse(typeStr, rawBytes)
		if err != nil {
			return nil, err
		}

		specs = append(specs, providers.TypedSpec{Type: typeStr, Spec: spec})
	}

	return specs, nil
}

func readFileOrURL(path string) ([]byte, error) {
	if !isRemoteURL(path) {
		return os.ReadFile(path)
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
	}

	return io.ReadAll(resp.Body)
}
