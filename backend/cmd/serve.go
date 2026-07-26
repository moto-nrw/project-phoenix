package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/applog"
	appmiddleware "github.com/moto-nrw/project-phoenix/middleware"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start http server with configured api",
	Long:  `Starts a http server and serves the configured api`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := validateServeConfig(); err != nil {
			slog.Error("invalid server configuration", slog.String("error", err.Error()))
			os.Exit(1)
		}

		logFormat := "json"
		if viper.GetBool("log_textlogging") {
			logFormat = "text"
		}

		logger := applog.New(applog.Config{
			Level:  viper.GetString("log_level"),
			Format: logFormat,
			Env:    viper.GetString("app_env"),
		})

		if dsn := strings.TrimSpace(viper.GetString("sentry_dsn")); dsn != "" {
			sentryEnv := strings.TrimSpace(viper.GetString("sentry_environment"))
			err := sentry.Init(sentry.ClientOptions{
				Dsn:         dsn,
				Environment: sentryEnv,
				BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
					return scrubSentryEvent(event)
				},
			})
			if err != nil {
				slog.Error("sentry init failed", slog.String("error", err.Error()))
				os.Exit(1)
			}
			defer sentry.Flush(2 * time.Second)
			logger.Info("sentry error tracking initialized")
		}

		server, err := api.NewServer(logger)
		if err != nil {
			slog.Error("failed to create server", slog.String("error", err.Error()))
			os.Exit(1)
		}
		server.Start()
	},
}

func scrubSentryEvent(event *sentry.Event) *sentry.Event {
	if event == nil {
		return event
	}
	// The public /public/calendar/{token} feed authenticates purely by the token
	// in its URL. Sentry's HTTP integration captures that URL (and the derived
	// transaction name / breadcrumbs), so a failing feed request would otherwise
	// ship a replayable capability token to Sentry. Redact it everywhere the SDK
	// may have recorded the path.
	event.Message = appmiddleware.RedactFeedToken(event.Message)
	event.Transaction = appmiddleware.RedactFeedToken(event.Transaction)
	for _, bc := range event.Breadcrumbs {
		if bc == nil {
			continue
		}
		bc.Message = appmiddleware.RedactFeedToken(bc.Message)
		for key, value := range bc.Data {
			if s, ok := value.(string); ok {
				bc.Data[key] = appmiddleware.RedactFeedToken(s)
			}
		}
	}
	if event.Request != nil {
		event.Request.URL = appmiddleware.RedactFeedToken(event.Request.URL)
		event.Request.QueryString = appmiddleware.RedactFeedToken(event.Request.QueryString)
		event.Request.Data = ""
		for key := range event.Request.Headers {
			for _, sensitive := range []string{"X-Staff-PIN", "X-Staff-Id", "X-Staff-Auth-PIN", "X-Device-Key"} {
				if strings.EqualFold(key, sensitive) {
					event.Request.Headers[key] = "[filtered]"
					break
				}
			}
		}
	}
	return event
}

func init() {
	RootCmd.AddCommand(serveCmd)

	viper.SetDefault("log_level", "debug")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func validateServeConfig() error {
	required := []string{
		"port",
		"app_env",
		"log_textlogging",
		"auth_jwt_secret",
		"auth_jwt_expiry",
		"auth_jwt_refresh_expiry",
		"frontend_url",
		"parents_url",
		"phoenix_auth_password",
	}

	var missing []string
	for _, key := range required {
		if strings.TrimSpace(viper.GetString(key)) == "" {
			missing = append(missing, strings.ToUpper(key))
		}
	}

	appEnv := strings.TrimSpace(viper.GetString("app_env"))
	if strings.TrimSpace(viper.GetString("db_dsn")) == "" {
		if appEnv == "test" && strings.TrimSpace(viper.GetString("test_db_dsn")) != "" {
			// Explicit test database DSN is allowed for test runs.
		} else {
			missing = append(missing, "DB_DSN")
		}
	}

	if strings.TrimSpace(viper.GetString("sentry_dsn")) != "" &&
		strings.TrimSpace(viper.GetString("sentry_environment")) == "" {
		missing = append(missing, "SENTRY_ENVIRONMENT")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if viper.GetString("auth_jwt_secret") == "random" {
		return fmt.Errorf("AUTH_JWT_SECRET=random is not allowed for serve; set an explicit secret")
	}

	if viper.GetDuration("auth_jwt_expiry") <= 0 {
		return fmt.Errorf("AUTH_JWT_EXPIRY must be a positive duration")
	}
	if viper.GetDuration("auth_jwt_refresh_expiry") <= 0 {
		return fmt.Errorf("AUTH_JWT_REFRESH_EXPIRY must be a positive duration")
	}

	return nil
}
