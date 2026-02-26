package pipe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/avast/retry-go"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/pkg/resourceprovider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSyncPipeCmd() *cobra.Command {
	var providerName string

	cmd := &cobra.Command{
		Use:   "pipe",
		Short: "Sync resources from stdin into Ctrlplane",
		Example: heredoc.Doc(`
			# One-shot sync from a script
			$ ./discover-databases.sh | ctrlc sync pipe --provider "custom-db"

			# Inline JSON
			$ echo '[{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server","config":{},"metadata":{}}]' \
			    | ctrlc sync pipe --provider "my-servers"

			# Single resource (no array wrapper needed)
			$ echo '{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server"}' \
			    | ctrlc sync pipe --provider "my-servers"

			# Resource with variables
			$ echo '[{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server","variables":{"env":"prod","tier":"frontend"}}]' \
			    | ctrlc sync pipe --provider "my-servers"

			# From curl with jq transformation
			$ curl -s https://cmdb.internal/api/servers \
			    | jq '[.[] | {name, identifier: .id, version: "cmdb/v1", kind: "Server", config: ., metadata: {}}]' \
			    | ctrlc sync pipe --provider "cmdb"
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Detect piped stdin
			stat, err := os.Stdin.Stat()
			if err != nil {
				return fmt.Errorf("failed to stat stdin: %w", err)
			}
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("no piped input detected -- pipe JSON resources to this command")
			}

			// Read all stdin
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("stdin is empty -- expected JSON resource array")
			}

			// Parse JSON -- try array first, then single object
			resourceInputs, err := parseResources(data)
			if err != nil {
				return err
			}

			// Validate required fields
			if err := validateResources(resourceInputs); err != nil {
				return err
			}

			resources := toAPIResources(resourceInputs)
			log.Info("Syncing resources from stdin", "count", len(resources), "provider", providerName)

			// Create API client
			apiURL := viper.GetString("url")
			apiKey := viper.GetString("api-key")
			workspace := viper.GetString("workspace")
			ctrlplaneClient, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Upsert resource provider
			rp, err := resourceprovider.New(ctrlplaneClient, workspace, providerName)
			if err != nil {
				return fmt.Errorf("failed to create resource provider: %w", err)
			}

			// Upsert resources
			ctx := context.Background()
			upsertResp, err := rp.UpsertResource(ctx, resources)
			if err != nil {
				return fmt.Errorf("failed to upsert resources: %w", err)
			}

			workspaceID := ctrlplaneClient.GetWorkspaceID(ctx, workspace).String()
			if err := syncResourceVariables(ctx, ctrlplaneClient, workspaceID, resourceInputs); err != nil {
				return err
			}

			log.Info("Response from upserting resources", "status", upsertResp.Status)

			return cliutil.HandleResponseOutput(cmd, upsertResp)
		},
	}

	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "Resource provider name")
	cmd.MarkFlagRequired("provider")

	return cmd
}

type resourceInput struct {
	Name       string            `json:"name"`
	Identifier string            `json:"identifier"`
	Version    string            `json:"version"`
	Kind       string            `json:"kind"`
	Config     map[string]any    `json:"config"`
	Metadata   map[string]string `json:"metadata"`
	Variables  map[string]any    `json:"-"`

	hasVariables bool `json:"-"`
}

func (r *resourceInput) UnmarshalJSON(data []byte) error {
	type resourceInputAlias struct {
		Name       string                 `json:"name"`
		Identifier string                 `json:"identifier"`
		Version    string                 `json:"version"`
		Kind       string                 `json:"kind"`
		Config     map[string]interface{} `json:"config"`
		Metadata   map[string]string      `json:"metadata"`
	}

	var alias resourceInputAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Name = alias.Name
	r.Identifier = alias.Identifier
	r.Version = alias.Version
	r.Kind = alias.Kind
	r.Config = alias.Config
	r.Metadata = alias.Metadata

	varRaw, ok := raw["variables"]
	if !ok {
		r.hasVariables = false
		return nil
	}

	r.hasVariables = true
	if string(varRaw) == "null" {
		r.Variables = map[string]any{}
		return nil
	}

	var vars map[string]any
	if err := json.Unmarshal(varRaw, &vars); err != nil {
		return fmt.Errorf("invalid variables field: must be a JSON object")
	}
	r.Variables = vars
	return nil
}

// parseResources attempts to parse the raw JSON data as either an array of
// resources or a single resource object. A single object is normalized to a
// one-element array.
func parseResources(data []byte) ([]resourceInput, error) {
	// Try array first
	var resources []resourceInput
	if err := json.Unmarshal(data, &resources); err == nil {
		return resources, nil
	}

	// Try single object
	var single resourceInput
	if err := json.Unmarshal(data, &single); err == nil {
		return []resourceInput{single}, nil
	}

	// Show a snippet of the input for debugging
	snippet := string(data)
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	return nil, fmt.Errorf("invalid JSON input: %s", snippet)
}

func toAPIResources(resources []resourceInput) []api.ResourceProviderResource {
	out := make([]api.ResourceProviderResource, 0, len(resources))
	for _, r := range resources {
		out = append(out, api.ResourceProviderResource{
			Name:       r.Name,
			Identifier: r.Identifier,
			Version:    r.Version,
			Kind:       r.Kind,
			Config:     r.Config,
			Metadata:   r.Metadata,
		})
	}
	return out
}

// validateResources checks that each resource has the required fields:
// Name, Identifier, Version, Kind.
func validateResources(resources []resourceInput) error {
	var errs []string
	for i, r := range resources {
		var missing []string
		if r.Name == "" {
			missing = append(missing, "name")
		}
		if r.Identifier == "" {
			missing = append(missing, "identifier")
		}
		if r.Version == "" {
			missing = append(missing, "version")
		}
		if r.Kind == "" {
			missing = append(missing, "kind")
		}
		if len(missing) > 0 {
			errs = append(errs, fmt.Sprintf("resource[%d]: missing required field(s) '%s'", i, strings.Join(missing, "', '")))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func syncResourceVariables(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID string,
	resources []resourceInput,
) error {
	for _, resource := range resources {
		if !resource.hasVariables {
			continue
		}

		vars := resource.Variables
		if vars == nil {
			vars = map[string]any{}
		}

		err := retry.Do(
			func() error {
				varsResp, err := client.RequestResourceVariablesUpdateWithResponse(
					ctx,
					workspaceID,
					resource.Identifier,
					api.RequestResourceVariablesUpdateJSONRequestBody(vars),
				)
				if err != nil {
					return retry.Unrecoverable(fmt.Errorf("failed to update resource variables for '%s': %w", resource.Identifier, err))
				}
				if varsResp == nil {
					return retry.Unrecoverable(fmt.Errorf("failed to update resource variables for '%s': empty response", resource.Identifier))
				}
				if varsResp.StatusCode() == 404 {
					return fmt.Errorf("resource '%s' not found yet, retrying", resource.Identifier)
				}
				if varsResp.StatusCode() >= 400 {
					return retry.Unrecoverable(
						fmt.Errorf("failed to update resource variables for '%s': %s", resource.Identifier, string(varsResp.Body)),
					)
				}
				return nil
			},
			retry.Attempts(10),
			retry.Delay(100*time.Millisecond),
			retry.MaxDelay(15*time.Second),
			retry.DelayType(retry.BackOffDelay),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
