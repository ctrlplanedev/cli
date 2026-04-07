package resources

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/internal/resources"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourcesCmd() *cobra.Command {
	var cel string
	var jq string
	var autoAccept bool
	var output string

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Get resources",
		Long:  `Get resources with optional server-side CEL filtering or client-side jq filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdStart := time.Now()
			defer func() {
				log.Debug("get resources completed", "duration", time.Since(cmdStart))
			}()

			log.Debug("resources command", "cel", cel, "jq", jq, "autoAccept", autoAccept)

			if jq != "" {
				if _, err := gojq.Parse(jq); err != nil {
					return fmt.Errorf("invalid jq expression: %w", err)
				}
			}

			svc, err := resources.NewAPIResourceService(cmd.Context(), viper.GetString("url"), viper.GetString("api-key"), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			if jq != "" {
				return handleJQ(cmd, svc, jq, autoAccept, output)
			}

			var celPtr *string
			if cel != "" {
				encoded := url.QueryEscape(cel)
				celPtr = &encoded
			}

			items, err := svc.List(cmd.Context(), celPtr)
			if err != nil {
				return err
			}

			return cliutil.HandleAnyOutput(cmd, items, output)
		},
	}

	cmd.Flags().StringVar(&cel, "cel", "", "Server-side CEL filter expression")
	cmd.Flags().StringVar(&jq, "jq", "", "Client-side jq filter expression (fetches all resources)")
	cmd.Flags().BoolVar(&autoAccept, "auto-accept", false, "Skip confirmation prompt when using --jq")
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json or yaml)")
	cmd.MarkFlagsMutuallyExclusive("cel", "jq")

	return cmd
}

func handleJQ(cmd *cobra.Command, svc *resources.APIResourceService, jqExpr string, autoAccept bool, output string) error {
	total, err := svc.GetTotal(cmd.Context())
	if err != nil {
		return err
	}

	pages := int(math.Ceil(float64(total) / 200.0))

	if !autoAccept {
		message := fmt.Sprintf("Found %d resources. This will require ~%d API request(s). Continue?", total, pages)
		confirmed, err := cliutil.ConfirmAction(message)
		if err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirmed {
			fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
			return nil
		}
	}

	items, err := svc.List(cmd.Context(), nil)
	if err != nil {
		return err
	}

	// Convert resources to generic interface for jq processing
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal resources: %w", err)
	}
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("failed to unmarshal resources: %w", err)
	}

	results, err := cliutil.ApplyJQ(jqExpr, data)
	if err != nil {
		return err
	}

	if len(results) == 1 {
		return cliutil.HandleAnyOutput(cmd, results[0], output)
	}
	return cliutil.HandleAnyOutput(cmd, results, output)
}
