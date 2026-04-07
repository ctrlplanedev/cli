package resource

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/internal/resources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourceCmd() *cobra.Command {
	var autoAccept bool
	var output string

	cmd := &cobra.Command{
		Use:   "resource <identifier>",
		Short: "Delete a resource by identifier",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdStart := time.Now()
			defer func() {
				log.Debug("delete resource completed", "duration", time.Since(cmdStart))
			}()

			identifier := args[0]
			log.Debug("delete resource", "identifier", identifier, "output", output, "autoAccept", autoAccept)

			svc, err := resources.NewAPIResourceService(cmd.Context(), viper.GetString("url"), viper.GetString("api-key"), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			log.Debug("fetching resource before delete", "identifier", identifier)
			resource, err := svc.GetByIdentifier(cmd.Context(), identifier)
			if err != nil {
				return err
			}
			log.Debug("resource found", "identifier", resource.Identifier, "name", resource.Name, "kind", resource.Kind)

			if output != "silent" {
				if err := cliutil.HandleAnyOutput(cmd, resource, output); err != nil {
					return err
				}
			}

			if !autoAccept {
				message := fmt.Sprintf("Are you sure you want to delete resource %q?", identifier)
				confirmed, err := cliutil.ConfirmAction(message)
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}
				if !confirmed {
					log.Debug("delete aborted by user")
					fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
					return nil
				}
				log.Debug("delete confirmed by user")
			}

			log.Debug("deleting resource", "identifier", identifier)
			result, err := svc.DeleteByIdentifier(cmd.Context(), identifier)
			if err != nil {
				return err
			}
			log.Debug("delete accepted", "id", result.Id, "message", result.Message)

			return nil
		},
	}

	cmd.Flags().BoolVar(&autoAccept, "auto-accept", false, "Skip confirmation prompt")
	cmd.Flags().StringVarP(&output, "output", "o", "silent", "Output format (json, yaml, or silent)")

	return cmd
}
