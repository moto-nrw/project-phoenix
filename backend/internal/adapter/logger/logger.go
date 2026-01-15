// Package logger provides structured logging with logrus.
// This package is part of the adapter layer in the hexagonal architecture.
package logger

import "github.com/sirupsen/logrus"

// Logger is a configured logrus.Logger.
// Defaults to the standard logger if not explicitly configured.
var Logger *logrus.Logger

func init() {
	// Initialize with standard logger as default
	Logger = logrus.StandardLogger()
}
