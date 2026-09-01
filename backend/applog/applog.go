package applog

import (
	"io"
	"log/slog"
	"os"
)

// Config holds logger configuration.
type Config struct {
	Level  string // "debug", "info", "warn", "error" (default: "info")
	Format string // "json", "text" (default: "json")
	Env    string // "development", "test", "production"
	Output io.Writer
}

// New creates a configured logger without changing process-global logging.
func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.Env == "production",
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}
	var handler slog.Handler
	if cfg.Format == "text" || cfg.Env == "development" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	logger := slog.New(handler)

	// Add global attributes that appear on every log line
	if cfg.Env != "" {
		logger = logger.With(slog.String("env", cfg.Env))
	}

	return logger
}

// ConfigureDefault installs the application logger at the process composition root.
func ConfigureDefault(logger *slog.Logger) {
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelWarn)
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
