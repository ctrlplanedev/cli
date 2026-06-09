package workflow

import (
	"fmt"
	"strings"

	apiv1 "buf.build/gen/go/ctrlplane/ctrlplane/protocolbuffers/go/ctrlplane/api/v1"
	"connectrpc.com/connect"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewTriggerCmd() *cobra.Command {
	var inputFlags []string

	cmd := &cobra.Command{
		Use:   "trigger <id-or-slug>",
		Short: "Trigger a workflow run",
		Long:  `Trigger a workflow run with the given inputs. The argument is auto-detected as a UUID or slug.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			client, err := api.NewConnectClient(viper.GetString("url"), viper.GetString("api-key"))
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			workspaceID, err := client.GetWorkspaceID(cmd.Context(), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			inputs := make(map[string]interface{})
			for _, input := range inputFlags {
				key, value, found := strings.Cut(input, "=")
				if !found {
					return fmt.Errorf("invalid input format %q, expected key=value", input)
				}
				if key == "" {
					return fmt.Errorf("invalid input format %q, empty key, expected key=value", input)
				}
				inputs[key] = value
			}

			if _, err := uuid.Parse(identifier); err == nil {
				resp, err := client.Workflow.CreateWorkflowRun(cmd.Context(), connect.NewRequest(&apiv1.CreateWorkflowRunRequest{
					WorkspaceId: workspaceID.String(),
					WorkflowId:  identifier,
					Inputs:      api.NewStruct(inputs),
				}))
				if err != nil {
					return fmt.Errorf("failed to trigger workflow: %w", err)
				}
				return cliutil.HandleProtoOutput(cmd, resp.Msg)
			}

			resp, err := client.Workflow.CreateWorkflowRunBySlug(cmd.Context(), connect.NewRequest(&apiv1.CreateWorkflowRunBySlugRequest{
				WorkspaceId: workspaceID.String(),
				Slug:        identifier,
				Inputs:      api.NewStruct(inputs),
			}))
			if err != nil {
				return fmt.Errorf("failed to trigger workflow by slug: %w", err)
			}
			return cliutil.HandleProtoOutput(cmd, resp.Msg)
		},
	}

	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Input key=value pair (can be specified multiple times)")

	return cmd
}
