// Command devtools runs the development and demo tools: the API seeder, the
// simulator and the demo process (ADR 0060). It is a binary of its own, so
// the production server image does not ship it; only the demo image and the
// development and CI environments build it. Every command refuses a target
// outside development or the demo (assertDevOnlyTarget, validateDemoTarget).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd is the root of the dev tools.
var RootCmd = &cobra.Command{
	Use:   "devtools",
	Short: "Development and demo tools: seed, simulate and the demo process",
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./dev.env)")
	// Fallback of the public demo (#3463); serve and demo must agree on it.
	RootCmd.PersistentFlags().Bool("demo-standing-school", false, "public demo: every demo access enters the standing demo school instead of a school of its own")
	_ = viper.BindPFlag("demo_standing_school", RootCmd.PersistentFlags().Lookup("demo-standing-school"))
}

// initConfig reads dev.env like the server's commands and makes the demo
// connection settings available to the persistence adapters, which read only
// the process environment.
func initConfig() {
	loadConfig(viper.GetViper(), cfgFile, isDocker())
	propagateDatabaseConfigFrom(viper.GetString, os.Setenv)
}

func loadConfig(config *viper.Viper, configFile string, docker bool) {
	if configFile != "" {
		config.SetConfigFile(configFile)
	} else if !docker {
		// In Docker all configuration comes from the Compose environment block.
		config.AddConfigPath(".")
		config.SetConfigName("dev")
		config.SetConfigType("env")
	}
	config.AutomaticEnv()
	if config.ReadInConfig() == nil {
		fmt.Println("Using config file:", config.ConfigFileUsed())
	}
}

func propagateDatabaseConfigFrom(get func(string) string, setenv func(string, string) error) {
	for _, key := range []string{
		"app_env", "db_dsn", "demo_db_dsn", "phoenix_demo_password",
		"db_max_open_conns", "db_max_idle_conns", "db_conn_max_lifetime", "db_conn_max_idle_time",
	} {
		if value := get(key); value != "" {
			if err := setenv(strings.ToUpper(key), value); err != nil {
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

// secureRandom streams the cryptographic entropy Security Runtime owns
// (#3364); the seeder draws generated credentials from it.
type secureRandom struct{}

func (secureRandom) Read(b []byte) (int, error) {
	if err := securityruntime.FillRandom(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func secureRandomSource() io.Reader { return secureRandom{} }
