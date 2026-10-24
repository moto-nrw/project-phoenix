package application

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// FailureLogger logs the failed operations of a Student Presence composition
// at warning level through the default logger.
func FailureLogger() func(ports.Observation) {
	return func(observation ports.Observation) {
		if observation.Err != nil {
			slog.Default().Warn("presence operation failed",
				"operation", observation.Operation,
				"error", observation.Err,
			)
		}
	}
}

// DebugLogger logs every operation of a Student Presence composition at
// debug level.
func DebugLogger(logger *slog.Logger) func(ports.Observation) {
	return func(o ports.Observation) {
		logger.Debug("student presence operation",
			"operation", o.Operation,
			"duration", o.Duration,
			"queries", o.Queries,
			"rows", o.Rows,
			"error", o.Err,
		)
	}
}
