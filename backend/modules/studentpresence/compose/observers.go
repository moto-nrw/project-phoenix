package compose

import "log/slog"

// FailureLogger logs the failed operations of a Student Presence composition
// at warning level through the default logger.
func FailureLogger() func(Observation) {
	return func(observation Observation) {
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
func DebugLogger(logger *slog.Logger) func(Observation) {
	return func(o Observation) {
		logger.Debug("student presence operation",
			"operation", o.Operation,
			"duration", o.Duration,
			"queries", o.Queries,
			"rows", o.Rows,
			"error", o.Err,
		)
	}
}
