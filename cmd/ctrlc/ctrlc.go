package main

import (
	"os"
	"strings"

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
	viper.SetEnvPrefix("CTRLC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	cobra.OnInitialize(initConfig)
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/.ctrlc.yaml)")
	viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))
	viper.BindEnv("config", "CTRLC_CONFIG", "CTRLPLANE_CONFIG")

	cmd.PersistentFlags().String("url", "https://app.ctrlplane.dev", "API URL")
	viper.BindPFlag("url", cmd.PersistentFlags().Lookup("url"))
	viper.BindEnv("url", "CTRLC_URL", "CTRLPLANE_URL")

	cmd.PersistentFlags().String("api-key", "", "API key for authentication")
	viper.BindPFlag("api-key", cmd.PersistentFlags().Lookup("api-key"))
	viper.BindEnv("api-key", "CTRLC_API_KEY", "CTRLPLANE_API_KEY")

	cmd.PersistentFlags().String("workspace", "", "Ctrlplane Workspace ID")
	viper.BindPFlag("workspace", cmd.PersistentFlags().Lookup("workspace"))
	viper.BindEnv("workspace", "CTRLC_WORKSPACE", "CTRLPLANE_WORKSPACE")

	viper.BindEnv("cluster-identifier", "CTRLC_CLUSTER_IDENTIFIER", "CTRLPLANE_CLUSTER_IDENTIFIER")
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	configFile := cfgFile
	if configFile == "" {
		configFile = viper.GetString("config")
	}

	if configFile != "" {
		viper.SetConfigFile(configFile)
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
