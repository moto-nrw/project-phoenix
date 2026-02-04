package cmd

import (
	"log"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/applog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start http server with configured api",
	Long:  `Starts a http server and serves the configured api`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := applog.New(applog.Config{
			Level:  viper.GetString("log_level"),
			Format: viper.GetString("log_format"),
			Env:    viper.GetString("app_env"),
		})

		server, err := api.NewServer(logger)
		if err != nil {
			log.Fatal(err)
		}
		server.Start()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.
	viper.SetDefault("port", "8080")
	viper.SetDefault("log_level", "debug")
	viper.SetDefault("log_format", "json")

	viper.SetDefault("login_url", "http://localhost:8080/login")
	viper.SetDefault("auth_jwt_secret", "random")
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
