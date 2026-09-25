package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
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

		if strings.TrimSpace(config.SentryDSN) != "" {
			if err := sentry.Init(sentryClientOptions(config)); err != nil {
				return fmt.Errorf("initialize sentry: %w", err)
			}
			defer sentry.Flush(2 * time.Second)
			logger.Info("sentry error tracking initialized")
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := api.WithRuntime(ctx, api.ServeConfig{
			Port:         config.Port,
			FrontendURL:  config.FrontendURL,
			PublicAPIURL: config.PublicAPIURL,
			EnableCORS:   config.EnableCORS,
			Logger:       logger,
			// A malformed DSN stops the runtime build.
			SentryPyrePortalDSN: config.SentryPyrePortalDSN,
		}, func(runtime *api.Runtime) error {
			return runtime.Serve(ctx)
		}); err != nil {
			return fmt.Errorf("run Serve runtime: %w", err)
		}
		return nil
	},
}

// release is the full commit SHA of the deployed build. The image build sets
// it through -ldflags from the SENTRY_RELEASE build argument (#3638), the same
// value the frontend reports. Local builds leave it empty.
var release string

// sentryEnvironments are the APP_ENV values a backend may report under. The
// Sentry environment is APP_ENV itself, so demo events filter apart from
// schools and staging without a second variable that could drift.
var sentryEnvironments = []string{"production", "staging", "demo", "development"}

func sentryClientOptions(config serveConfig) sentry.ClientOptions {
	return sentry.ClientOptions{
		Dsn:         strings.TrimSpace(config.SentryDSN),
		Environment: strings.TrimSpace(config.AppEnv),
		Release:     release,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			return scrubSentryEvent(event)
		},
	}
}

func scrubSentryEvent(event *sentry.Event) *sentry.Event {
	if event == nil {
		return event
	}
	// Public calendar and request feeds authenticate purely by the token in the
	// URL. Sentry's HTTP integration captures that URL (and the derived
	// transaction name / breadcrumbs), so a failing feed request would otherwise
	// ship a replayable capability token to Sentry. Redact it everywhere the SDK
	// may have recorded the path.
	//
	// Error texts reach Sentry as exception values and, from background work,
	// as breadcrumbs (#3640). A mail server's rejection names the recipient's
	// e-mail address, which must never leave for Sentry (#3590).
	event.Message = scrubSentryText(event.Message)
	event.Transaction = appmiddleware.RedactFeedToken(event.Transaction)
	for i := range event.Exception {
		event.Exception[i].Value = scrubSentryText(event.Exception[i].Value)
	}
	for _, bc := range event.Breadcrumbs {
		if bc == nil {
			continue
		}
		bc.Message = scrubSentryText(bc.Message)
		for key, value := range bc.Data {
			if s, ok := value.(string); ok {
				bc.Data[key] = scrubSentryText(s)
			}
		}
	}
	if event.Request != nil {
		event.Request.URL = appmiddleware.RedactFeedToken(event.Request.URL)
		event.Request.QueryString = appmiddleware.RedactFeedToken(event.Request.QueryString)
		event.Request.Data = ""
		for key := range event.Request.Headers {
			for _, sensitive := range []string{"Authorization", "X-Staff-PIN", "X-Staff-Auth-PIN", "X-Staff-Id", "X-Device-Key"} {
				if strings.EqualFold(key, sensitive) {
					event.Request.Headers[key] = "[filtered]"
					break
				}
			}
		}
	}
	return event
}

// sentryEmailAddress matches an e-mail address inside free text.
var sentryEmailAddress = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// scrubSentryText redacts feed tokens and e-mail addresses in free text.
func scrubSentryText(s string) string {
	return sentryEmailAddress.ReplaceAllString(appmiddleware.RedactFeedToken(s), "[email]")
}

func init() {
	RootCmd.AddCommand(serveCmd)

	viper.SetDefault("log_level", "debug")
	// Capacity of the public demo (#3466); required under APP_ENV=demo.
	serveCmd.Flags().Int("demo-max-active-schools", 0, "public demo: how many demo schools may exist at once; further requests answer 503")
	_ = viper.BindPFlag("demo_max_active_schools", serveCmd.Flags().Lookup("demo-max-active-schools"))

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type serveConfig struct {
	Port, AppEnv, LogTextLoggingRaw                            string
	JWTSecret, JWTExpiry, JWTRefreshExpiry                     string
	FrontendURL, PublicAPIURL, ParentsURL, PhoenixAuthPassword string
	DatabaseDSN, TestDatabaseDSN                               string
	SentryDSN, SentryPyrePortalDSN                             string
	LogLevel                                                   string
	LogTextLogging, EnableCORS                                 bool
}

func currentServeConfig() serveConfig {
	return serveConfig{
		Port: viper.GetString("port"), AppEnv: viper.GetString("app_env"),
		LogTextLoggingRaw: viper.GetString("log_textlogging"), LogTextLogging: viper.GetBool("log_textlogging"),
		JWTSecret: viper.GetString("auth_jwt_secret"), JWTExpiry: viper.GetString("auth_jwt_expiry"),
		JWTRefreshExpiry: viper.GetString("auth_jwt_refresh_expiry"), FrontendURL: viper.GetString("frontend_url"),
		PublicAPIURL: viper.GetString("next_public_api_url"),
		ParentsURL:   viper.GetString("parents_url"), PhoenixAuthPassword: viper.GetString("phoenix_auth_password"),
		DatabaseDSN: viper.GetString("db_dsn"), TestDatabaseDSN: viper.GetString("test_db_dsn"),
		SentryDSN: viper.GetString("sentry_dsn"), SentryPyrePortalDSN: viper.GetString("sentry_pyreportal_dsn"),
		LogLevel: viper.GetString("log_level"), EnableCORS: viper.GetBool("enable_cors"),
	}
}

func validateServeConfig(config serveConfig) error {
	required := map[string]string{
		"PORT": config.Port, "APP_ENV": config.AppEnv, "LOG_TEXTLOGGING": config.LogTextLoggingRaw,
		"AUTH_JWT_SECRET": config.JWTSecret, "AUTH_JWT_EXPIRY": config.JWTExpiry,
		"AUTH_JWT_REFRESH_EXPIRY": config.JWTRefreshExpiry, "FRONTEND_URL": config.FrontendURL,
		"NEXT_PUBLIC_API_URL": config.PublicAPIURL,
		"PARENTS_URL":         config.ParentsURL, "PHOENIX_AUTH_PASSWORD": config.PhoenixAuthPassword,
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

	// The kiosks report through the backend (#3645), so a backend that
	// reports to Sentry must know where their reports go.
	if strings.TrimSpace(config.SentryDSN) != "" && strings.TrimSpace(config.SentryPyrePortalDSN) == "" {
		missing = append(missing, "SENTRY_PYREPORTAL_DSN")
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// test, dev and local stay possible without a DSN; with one, an unknown
	// APP_ENV would file events under an environment nobody filters for.
	if strings.TrimSpace(config.SentryDSN) != "" && !slices.Contains(sentryEnvironments, appEnv) {
		return fmt.Errorf("APP_ENV=%q is not allowed with SENTRY_DSN set; use one of: %s",
			appEnv, strings.Join(sentryEnvironments, ", "))
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
