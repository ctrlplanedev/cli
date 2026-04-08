package resource

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/ctrlplanedev/cli/internal/resources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewResourceCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "resource <identifier>",
		Short: "Get a single resource by identifier",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdStart := time.Now()
			defer func() {
				log.Debug("get resource completed", "duration", time.Since(cmdStart))
			}()

			svc, err := resources.NewAPIResourceService(cmd.Context(), viper.GetString("url"), viper.GetString("api-key"), viper.GetString("workspace"))
			if err != nil {
				return err
			}

			resource, err := svc.GetByIdentifier(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			return cliutil.HandleAnyOutput(cmd, resource, output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json or yaml)")

	return cmd
}
