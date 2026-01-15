// Package logger provides structured logging for core services.
// This keeps core packages independent from adapter-level logger wiring.
package logger

import "github.com/sirupsen/logrus"

// Logger is a configured logrus.Logger.
// Defaults to the standard logger if not explicitly configured.
var Logger *logrus.Logger

func init() {
	Logger = logrus.StandardLogger()
}
