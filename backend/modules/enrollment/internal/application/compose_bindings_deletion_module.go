package application

import (
	slog "log/slog"
)

func WithService(logger *slog.Logger, service string) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With("service", service)
}
