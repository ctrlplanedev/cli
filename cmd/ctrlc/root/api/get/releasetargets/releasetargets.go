package releasetargets

import (
	"fmt"
	"io"
	"os"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type resourceInput struct {
	Name       string            `yaml:"name"`
	Kind       string            `yaml:"kind"`
	Version    string            `yaml:"version"`
	Identifier string            `yaml:"identifier"`
	Config     map[string]any    `yaml:"config"`
	Metadata   map[string]string `yaml:"metadata"`
}

func NewReleaseTargetsCmd() *cobra.Command {
	var (
		filePath   string
		name       string
		kind       string
		version    string
		identifier string
		metadata   map[string]string
		config     map[string]string
		limit      int
		offset     int
	)

	cmd := &cobra.Command{
		Use:   "release-targets",
		Short: "Preview release targets for a resource",
		Long:  `Simulates which release targets would be created if a given resource were added to the workspace. No resources are actually created.`,
		Example: `  # Preview from a YAML file
  ctrlc api get release-targets -f resource.yaml

  # Preview from stdin
  cat resource.yaml | ctrlc api get release-targets -f -

  # Preview with inline flags
  ctrlc api get release-targets --name my-pod --kind kubernetes/pod --version v1 --identifier my-pod-id`,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := buildInput(filePath, name, kind, version, identifier, metadata, config)
			if err != nil {
				return err
			}

			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")

			client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID := client.GetWorkspaceID(cmd.Context(), workspace)

			params := &api.PreviewReleaseTargetsForResourceParams{}
			if limit > 0 {
				params.Limit = &limit
			}
			if offset > 0 {
				params.Offset = &offset
			}

			body := api.PreviewReleaseTargetsForResourceJSONRequestBody{
				Name:       input.Name,
				Kind:       input.Kind,
				Version:    input.Version,
				Identifier: input.Identifier,
				Config:     input.Config,
				Metadata:   input.Metadata,
			}

			resp, err := client.PreviewReleaseTargetsForResource(cmd.Context(), workspaceID.String(), params, body)
			if err != nil {
				return fmt.Errorf("failed to preview release targets: %w", err)
			}

			return cliutil.HandleResponseOutput(cmd, resp)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to a resource YAML file (use - for stdin)")
	cmd.Flags().StringVar(&name, "name", "", "Resource name")
	cmd.Flags().StringVar(&kind, "kind", "", "Resource kind")
	cmd.Flags().StringVar(&version, "version", "", "Resource version")
	cmd.Flags().StringVar(&identifier, "identifier", "", "Resource identifier")
	cmd.Flags().StringToStringVar(&metadata, "metadata", nil, "Resource metadata as key=value pairs")
	cmd.Flags().StringToStringVar(&config, "config", nil, "Resource config as key=value pairs")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Offset the results")

	return cmd
}

func buildInput(
	filePath, name, kind, version, identifier string,
	metadata map[string]string,
	config map[string]string,
) (*resourceInput, error) {
	if filePath != "" {
		return parseResourceFile(filePath)
	}

	if name == "" || kind == "" || version == "" || identifier == "" {
		return nil, fmt.Errorf("when not using -f, --name, --kind, --version, and --identifier are all required")
	}

	input := &resourceInput{
		Name:       name,
		Kind:       kind,
		Version:    version,
		Identifier: identifier,
		Metadata:   metadata,
		Config:     make(map[string]any),
	}

	for k, v := range config {
		input.Config[k] = v
	}

	if input.Metadata == nil {
		input.Metadata = make(map[string]string)
	}

	return input, nil
}

func parseResourceFile(filePath string) (*resourceInput, error) {
	var data []byte
	var err error

	if filePath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(filePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var input resourceInput
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to parse resource YAML: %w", err)
	}

	if input.Name == "" {
		return nil, fmt.Errorf("resource YAML missing required 'name' field")
	}
	if input.Kind == "" {
		return nil, fmt.Errorf("resource YAML missing required 'kind' field")
	}
	if input.Version == "" {
		return nil, fmt.Errorf("resource YAML missing required 'version' field")
	}
	if input.Identifier == "" {
		return nil, fmt.Errorf("resource YAML missing required 'identifier' field")
	}

	if input.Config == nil {
		input.Config = make(map[string]any)
	}
	if input.Metadata == nil {
		input.Metadata = make(map[string]string)
	}

	return &input, nil
}
