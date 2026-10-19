package audit

import (
	"log/slog"
	"time"
)

func NewLoggingObserver(logger *slog.Logger, observe func(string, time.Duration, int, error)) func(AppendObservation) {
	return func(observation AppendObservation) {
		observe(observation.EventType, observation.Duration, observation.Rows, observation.Err)
		log := logger.Debug
		if observation.Err != nil {
			log = logger.Error
		}
		log("audit append",
			"event_type", observation.EventType,
			"duration_ms", observation.Duration.Milliseconds(),
			"rows", observation.Rows,
			"failed", observation.Err != nil,
		)
	}
}
