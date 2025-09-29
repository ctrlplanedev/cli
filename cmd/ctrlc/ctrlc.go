package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root"
	"github.com/ctrlplanedev/cli/internal/telemetry"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cmd     = root.NewRootCmd()
)

func init() {
	cobra.OnInitialize(initConfig)
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/.ctrlc.yaml)")
	viper.BindEnv("config", "CTRLPLANE_CONFIG")

	cmd.PersistentFlags().String("url", "https://app.ctrlplane.dev", "API URL")
	viper.BindPFlag("url", cmd.PersistentFlags().Lookup("url"))
	viper.BindEnv("url", "CTRLPLANE_URL")

	cmd.PersistentFlags().String("api-key", "", "API key for authentication")
	viper.BindPFlag("api-key", cmd.PersistentFlags().Lookup("api-key"))
	viper.BindEnv("api-key", "CTRLPLANE_API_KEY")

	cmd.PersistentFlags().String("workspace", "", "Ctrlplane Workspace ID")
	viper.BindPFlag("workspace", cmd.PersistentFlags().Lookup("workspace"))
	viper.BindEnv("workspace", "CTRLPLANE_WORKSPACE")

	viper.BindEnv("cluster-identifier", "CTRLPLANE_CLUSTER_IDENTIFIER")
}

func main() {
	ctx := context.Background()

	// Initialize telemetry
	shutdown, err := telemetry.InitTelemetry(ctx)
	if err != nil {
		log.Warn("Failed to initialize telemetry", "error", err)
		// Continue execution even if telemetry fails
	}

	// Ensure telemetry is properly shut down
	if shutdown != nil {
		defer func() {
			// Give a brief moment for any pending spans to be exported
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if shutdownErr := shutdown(shutdownCtx); shutdownErr != nil {
				log.Debug("Error during telemetry shutdown", "error", shutdownErr)
			}
		}()
	}

	// Determine command name for the root span
	commandName := "help"
	if len(os.Args) > 1 {
		commandName = strings.Join(os.Args[1:], " ")
	}

	// Start root span
	ctx, rootSpan := telemetry.StartRootSpan(ctx, commandName, os.Args[1:])
	defer rootSpan.End()

	// Execute command with telemetry context
	if err := executeWithTelemetry(ctx, cmd); err != nil {
		telemetry.SetSpanError(rootSpan, err)
		os.Exit(1)
	}

	telemetry.SetSpanSuccess(rootSpan)
}

// executeWithTelemetry wraps the command execution with telemetry context
func executeWithTelemetry(ctx context.Context, cmd *cobra.Command) error {
	// Set the context in the command so it can be used by subcommands
	cmd.SetContext(ctx)
	return cmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.Error("Can't find home directory", "error", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".ctrlc")
		viper.SetConfigType("yaml")
		viper.SafeWriteConfig()
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Error("Can't read config", "error", err)
		os.Exit(1)
	}
}
