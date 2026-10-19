package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
)

func FailureLogger() func(Observation)                  { return application.FailureLogger() }
func DebugLogger(logger *slog.Logger) func(Observation) { return application.DebugLogger(logger) }
