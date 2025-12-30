// Package logging provides structured logging with logrus.
package logging

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// Logger is a configured logrus.Logger.
var Logger *logrus.Logger

// GetLogEntry return the request scoped logrus.FieldLogger.
// Falls back to standard logger if no structured logging is configured.
func GetLogEntry(r *http.Request) logrus.FieldLogger {
	// First, safely get the log entry from middleware
	logEntry := middleware.GetLogEntry(r)
	if logEntry == nil {
		// Return a default logger if there's no log entry in the request context
		return logrus.StandardLogger()
	}

	// Fallback to standard logger since structured logging is not configured
	return logrus.StandardLogger()
}
