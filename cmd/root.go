package cmd

import (
	"log/slog"
	"os"

	"github.com/devon-mar/regexupdater/regexupdater"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "regexupdater",
	Short: "Update anything with the power of regular expressions.",
}

var (
	cfgFile string
	config  *regexupdater.Config
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".regexupdater.yml", "config file (default is .regexupdater.yaml)")
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	var err error
	config, err = regexupdater.ReadConfig(cfgFile)
	if err != nil {
		slog.Error("Error loading config.", "err", err)
		os.Exit(1)
	}
}
