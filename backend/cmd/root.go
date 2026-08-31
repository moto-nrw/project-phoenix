package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "phoenix",
	Short: "RFID-based student attendance and room management system",
	Long:  `Project Phoenix - A GDPR-compliant RFID-based student attendance and room management system for educational institutions.`,
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

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./dev.env")
	RootCmd.PersistentFlags().Bool("db_debug", false, "log sql to console")
	_ = viper.BindPFlag("db_debug", RootCmd.PersistentFlags().Lookup("db_debug"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else if !isDocker() {
		// Only load dev.env for local development (go run main.go serve).
		// In Docker, all config comes from the docker-compose environment block.
		// Loading dev.env inside Docker causes conflicts (e.g., DB_DSN pointing
		// to localhost instead of the Docker network hostname).
		viper.AddConfigPath(".")
		viper.SetConfigName("dev")
		viper.SetConfigType("env")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if viper.ReadInConfig() == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
	propagateDatabaseConfig()
}

// propagateDatabaseConfig makes the effective local Viper configuration
// available to persistence adapters, which intentionally depend only on the
// process environment.
func propagateDatabaseConfig() {
	for _, key := range []string{
		"app_env", "db_dsn", "test_db_dsn", "phoenix_auth_password",
		"db_max_open_conns", "db_max_idle_conns", "db_conn_max_lifetime", "db_conn_max_idle_time",
	} {
		if value := viper.GetString(key); value != "" {
			if err := os.Setenv(strings.ToUpper(key), value); err != nil {
				panic(fmt.Errorf("propagate %s configuration: %w", key, err))
			}
		}
	}
}

// isDocker reports whether the process is running inside a Docker container.
func isDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}
