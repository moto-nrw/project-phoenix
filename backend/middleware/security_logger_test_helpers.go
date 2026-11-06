//go:build testhooks

package middleware

import "log/slog"

// NewSecurityLoggerForTest writes the security events to logger, so a parallel
// test can read them without replacing the process-wide default logger.
func NewSecurityLoggerForTest(logger *slog.Logger) *SecurityLogger {
	return &SecurityLogger{logger: logger}
}
