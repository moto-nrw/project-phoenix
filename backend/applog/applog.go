package applog

import (
	"log/slog"
	"os"
)

// Config holds logger configuration.
type Config struct {
	Level  string // "debug", "info", "warn", "error" (default: "info")
	Format string // "json", "text" (default: "json")
	Env    string // "development", "test", "production"
}

// New creates a configured *slog.Logger, sets it as the global default,
// and returns it for explicit dependency injection.
//
// After calling New, all existing log.Printf/log.Println calls automatically
// route through the configured slog handler (via slog.SetDefault).
func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.Env == "production",
	}

	var handler slog.Handler
	if cfg.Format == "text" || cfg.Env == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)

	// Add global attributes that appear on every log line
	if cfg.Env != "" {
		logger = logger.With(slog.String("env", cfg.Env))
	}

	// Set as global default — captures existing log.Printf calls.
	// After this, all log.Printf/log.Println calls route through slog.
	// They are logged at LevelInfo by default (configurable via SetLogLoggerLevel).
	// See: https://pkg.go.dev/log/slog#SetDefault
	slog.SetDefault(logger)

	// Route stdlib log.Printf calls as WARN (not INFO) so they stand out
	// as "not yet migrated" during the transition period.
	slog.SetLogLoggerLevel(slog.LevelWarn)

	return logger
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
