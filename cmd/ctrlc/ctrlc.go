package main

import (
	"fmt"
	"os"

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
	viper.BindEnv("config", "CTRLPLANE_CONFIG")

	cmd.PersistentFlags().String("url", "", "API URL")
	viper.BindPFlag("url", cmd.PersistentFlags().Lookup("url"))
	viper.BindEnv("url", "CTRLPLANE_URL")

	cmd.PersistentFlags().String("api-key", "", "API key for authentication")
	viper.BindPFlag("api-key", cmd.PersistentFlags().Lookup("api-key"))
	viper.BindEnv("api-key", "CTRLPLANE_API_KEY")
}

func main() {
	cmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".ctrlc")
		viper.SetConfigType("yaml")
		viper.SafeWriteConfig()
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
}
