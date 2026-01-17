// Package logger provides structured logging with logrus.
// This package is part of the adapter layer in the hexagonal architecture.
package logger

import (
	"os"
	"time"

	corelogger "github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// Logger is a configured logrus.Logger.
// Defaults to the standard logger if not explicitly configured.
var Logger *logrus.Logger

func init() {
	// Initialize with standard logger as default
	Logger = logrus.StandardLogger()
	Logger.SetOutput(os.Stdout)
	Logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	corelogger.SetLogger(newLogrusAdapter(Logger))
}

type logrusAdapter struct {
	entry *logrus.Entry
}

func newLogrusAdapter(logger *logrus.Logger) corelogger.StructuredLogger {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return logrusAdapter{entry: logrus.NewEntry(logger)}
}

func (l logrusAdapter) WithField(key string, value any) corelogger.StructuredLogger {
	return logrusAdapter{entry: l.entry.WithField(key, value)}
}

func (l logrusAdapter) WithFields(fields map[string]any) corelogger.StructuredLogger {
	if fields == nil {
		return logrusAdapter{entry: l.entry}
	}
	return logrusAdapter{entry: l.entry.WithFields(logrus.Fields(fields))}
}

func (l logrusAdapter) WithError(err error) corelogger.StructuredLogger {
	return logrusAdapter{entry: l.entry.WithError(err)}
}

func (l logrusAdapter) Debug(args ...any) { l.entry.Debug(args...) }
func (l logrusAdapter) Info(args ...any)  { l.entry.Info(args...) }
func (l logrusAdapter) Warn(args ...any)  { l.entry.Warn(args...) }
func (l logrusAdapter) Error(args ...any) { l.entry.Error(args...) }
