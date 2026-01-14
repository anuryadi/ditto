package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/anuryadi/ditto/pkg/logger"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ditto",
	Short: "A powerful database synchronization tool",
	Long: `ditto is a CLI tool for synchronizing data between databases.

It supports comparing schemas, syncing data, and exporting to various formats.
Currently supports PostgreSQL and MySQL databases.

Examples:
  # Compare two database schemas
  ditto compare --source "postgres://user:pass@localhost/db1" --target "mysql://user:pass@localhost/db2"

  # Sync data from source to target
  ditto sync --source "postgres://..." --target "mysql://..." --tables users,orders

  # Export table to JSON
  ditto export --source "postgres://..." --table users --format json --output users.json`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Re-initialize logger with verbose flag
		logger.Init(verbose)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ditto.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".ditto" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ditto")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("DITTO")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			logger.Infof("Using config file: %s", viper.ConfigFileUsed())
		}
	}
}
