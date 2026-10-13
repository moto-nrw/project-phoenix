package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

// newStatistics builds the Statistik report over the presence database of
// deps with the operation logging of newStudentPresence.
func newStatistics(logger *slog.Logger, deps presenceCompose.StatisticsDependencies) studentpresence.StatisticsReports {
	deps.Observe = studentPresenceObserver(logger)
	reports, err := presenceCompose.NewStatistics(deps)
	if err != nil {
		panic(err)
	}
	return reports
}

func studentPresenceObserver(logger *slog.Logger) func(presenceCompose.Observation) {
	return func(o presenceCompose.Observation) {
		logger.Debug("student presence operation",
			"operation", o.Operation,
			"duration", o.Duration,
			"queries", o.Queries,
			"rows", o.Rows,
			"error", o.Err,
		)
	}
}
