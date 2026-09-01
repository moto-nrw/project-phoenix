package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		config := currentServeConfig()
		if err := validateServeConfig(config); err != nil {
			return fmt.Errorf("invalid server configuration: %w", err)
		}

		logFormat := "json"
		if config.LogTextLogging {
			logFormat = "text"
		}

		logger := applog.New(applog.Config{
			Level:  config.LogLevel,
			Format: logFormat,
			Env:    config.AppEnv,
		})
		applog.ConfigureDefault(logger)

		if dsn := strings.TrimSpace(config.SentryDSN); dsn != "" {
			sentryEnv := strings.TrimSpace(config.SentryEnvironment)
			err := sentry.Init(sentry.ClientOptions{
				Dsn:         dsn,
				Environment: sentryEnv,
				BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
					return scrubSentryEvent(event)
				},
			})
			if err != nil {
				return fmt.Errorf("initialize sentry: %w", err)
			}
			defer sentry.Flush(2 * time.Second)
			logger.Info("sentry error tracking initialized")
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := api.WithRuntime(ctx, api.ServeConfig{
			Port:       config.Port,
			EnableCORS: config.EnableCORS,
			Logger:     logger,
		}, func(runtime *api.Runtime) error {
			return runtime.Serve(ctx)
		}); err != nil {
			return fmt.Errorf("run Serve runtime: %w", err)
		}
		return nil
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

type serveConfig struct {
	Port, AppEnv, LogTextLoggingRaw              string
	JWTSecret, JWTExpiry, JWTRefreshExpiry       string
	FrontendURL, ParentsURL, PhoenixAuthPassword string
	DatabaseDSN, TestDatabaseDSN                 string
	SentryDSN, SentryEnvironment, LogLevel       string
	LogTextLogging, EnableCORS                   bool
}

func currentServeConfig() serveConfig {
	return serveConfig{
		Port: viper.GetString("port"), AppEnv: viper.GetString("app_env"),
		LogTextLoggingRaw: viper.GetString("log_textlogging"), LogTextLogging: viper.GetBool("log_textlogging"),
		JWTSecret: viper.GetString("auth_jwt_secret"), JWTExpiry: viper.GetString("auth_jwt_expiry"),
		JWTRefreshExpiry: viper.GetString("auth_jwt_refresh_expiry"), FrontendURL: viper.GetString("frontend_url"),
		ParentsURL: viper.GetString("parents_url"), PhoenixAuthPassword: viper.GetString("phoenix_auth_password"),
		DatabaseDSN: viper.GetString("db_dsn"), TestDatabaseDSN: viper.GetString("test_db_dsn"),
		SentryDSN: viper.GetString("sentry_dsn"), SentryEnvironment: viper.GetString("sentry_environment"),
		LogLevel: viper.GetString("log_level"), EnableCORS: viper.GetBool("enable_cors"),
	}
}

func validateServeConfig(config serveConfig) error {
	required := map[string]string{
		"PORT": config.Port, "APP_ENV": config.AppEnv, "LOG_TEXTLOGGING": config.LogTextLoggingRaw,
		"AUTH_JWT_SECRET": config.JWTSecret, "AUTH_JWT_EXPIRY": config.JWTExpiry,
		"AUTH_JWT_REFRESH_EXPIRY": config.JWTRefreshExpiry, "FRONTEND_URL": config.FrontendURL,
		"PARENTS_URL": config.ParentsURL, "PHOENIX_AUTH_PASSWORD": config.PhoenixAuthPassword,
	}

	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	appEnv := strings.TrimSpace(config.AppEnv)
	if strings.TrimSpace(config.DatabaseDSN) == "" {
		if appEnv == "test" && strings.TrimSpace(config.TestDatabaseDSN) != "" {
			// Explicit test database DSN is allowed for test runs.
		} else {
			missing = append(missing, "DB_DSN")
		}
	}

	if strings.TrimSpace(config.SentryDSN) != "" && strings.TrimSpace(config.SentryEnvironment) == "" {
		missing = append(missing, "SENTRY_ENVIRONMENT")
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if config.JWTSecret == "random" {
		return fmt.Errorf("AUTH_JWT_SECRET=random is not allowed for serve; set an explicit secret")
	}

	if duration, err := time.ParseDuration(config.JWTExpiry); err != nil || duration <= 0 {
		return fmt.Errorf("AUTH_JWT_EXPIRY must be a positive duration")
	}
	if duration, err := time.ParseDuration(config.JWTRefreshExpiry); err != nil || duration <= 0 {
		return fmt.Errorf("AUTH_JWT_REFRESH_EXPIRY must be a positive duration")
	}

	return nil
}
