package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	CLI_NAME = "crit"
)

var (
	// Used for flags
	cfgFile      string
	apiToken     string
	apiEndpoint  string
	outputFormat string
	verbose      bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   CLI_NAME,
	Short: "A CLI application for structured queries against APIs",
	Long: `

A CLI application for running structured criteria queries against APIs.
Built using the go-crit library for type-safe query building and execution.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	RootCmd.PersistentFlags().StringP("output", "o", "table", "Output format (table, json, yaml)")
	RootCmd.PersistentFlags().String("token", "", "API token for authentication")
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	RootCmd.PersistentFlags().DurationP("timeout", "t", 30, "Request timeout in seconds")

	// Bind to viper
	viper.BindPFlag("output", RootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("timeout", RootCmd.PersistentFlags().Lookup("timeout"))

	// Allow environment variables
	viper.BindEnv("token", "ZENDESK_API_TOKEN")
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// Search config in home directory or current directory with name ".go-crit"
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath(".")
	viper.SetConfigName(".crit")

	// Read in environment variables
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}
