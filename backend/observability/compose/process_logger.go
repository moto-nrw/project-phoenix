package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/observability/internal/adapters/applog"
)

// InstallProcessLogger builds the process logger from the configured level, text
// format flag and environment, and installs it as the process default. The
// roots call it once at start (#3300).
func InstallProcessLogger(level string, textLogging bool, env string) *slog.Logger {
	logger := newProcessLogger(level, textLogging, env)
	applog.ConfigureDefault(logger)
	return logger
}

func newProcessLogger(level string, textLogging bool, env string) *slog.Logger {
	format := "json"
	if textLogging {
		format = "text"
	}
	return applog.New(applog.Config{Level: level, Format: format, Env: env})
}
