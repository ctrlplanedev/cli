package run

import (
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/run/exec"
	"github.com/ctrlplanedev/cli/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runners listen for jobs and execute them.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	interval := viper.GetString("interval")
	if interval == "" {
		interval = "10s"
	}

	cmd.AddCommand(cliutil.AddIntervalSupport(exec.NewRunExecCmd(), interval))

	return cmd
}
