package active

import "log/slog"

// loggerOrDefault keeps bare-constructed services nil-safe.
func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
