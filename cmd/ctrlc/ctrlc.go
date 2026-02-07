package main

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root"
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
	mustBind(viper.BindEnv("config", "CTRLPLANE_CONFIG"))

	cmd.PersistentFlags().String("url", "https://app.ctrlplane.dev", "API URL")
	mustBind(viper.BindPFlag("url", cmd.PersistentFlags().Lookup("url")))
	mustBind(viper.BindEnv("url", "CTRLPLANE_URL"))

	cmd.PersistentFlags().String("api-key", "", "API key for authentication")
	mustBind(viper.BindPFlag("api-key", cmd.PersistentFlags().Lookup("api-key")))
	mustBind(viper.BindEnv("api-key", "CTRLPLANE_API_KEY"))

	cmd.PersistentFlags().String("workspace", "", "Ctrlplane Workspace ID")
	mustBind(viper.BindPFlag("workspace", cmd.PersistentFlags().Lookup("workspace")))
	mustBind(viper.BindEnv("workspace", "CTRLPLANE_WORKSPACE"))

	mustBind(viper.BindPFlag("log-level", cmd.PersistentFlags().Lookup("log-level")))
	mustBind(viper.BindEnv("log-level", "CTRLPLANE_LOG_LEVEL"))

	mustBind(viper.BindEnv("cluster-identifier", "CTRLPLANE_CLUSTER_IDENTIFIER"))
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
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
		mustBind(viper.SafeWriteConfig())
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Error("Can't read config", "error", err)
		os.Exit(1)
	}
}

func mustBind(err error) {
	if err != nil {
		log.Error("Config binding failed", "error", err)
		os.Exit(1)
	}
}
