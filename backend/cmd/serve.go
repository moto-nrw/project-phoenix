package cmd

import (
	"log"
	"time"

	"github.com/getsentry/sentry-go"
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

		if dsn := viper.GetString("sentry_dsn"); dsn != "" {
			err := sentry.Init(sentry.ClientOptions{
				Dsn:         dsn,
				Environment: viper.GetString("app_env"),
				BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
					if event.Request != nil {
						for _, key := range []string{"X-Staff-PIN", "X-Staff-Id", "X-Device-Key"} {
							if _, ok := event.Request.Headers[key]; ok {
								event.Request.Headers[key] = "[filtered]"
							}
						}
					}
					return event
				},
			})
			if err != nil {
				log.Fatalf("sentry init failed: %s", err)
			}
			defer sentry.Flush(2 * time.Second)
			logger.Info("sentry error tracking initialized")
		}

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
	viper.SetDefault("auth_jwt_expiry", "1h")
	viper.SetDefault("auth_jwt_refresh_expiry", "168h")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
