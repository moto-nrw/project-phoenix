package students

import (
	slog "log/slog"
)

// getLogger returns a nil-safe logger for handler use.
func (rs *Resource) getLogger() *slog.Logger {
	if rs.Logger == nil {
		return slog.Default()
	}
	return rs.Logger
}
