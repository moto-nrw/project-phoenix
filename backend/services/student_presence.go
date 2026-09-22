package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/uptrace/bun"
)

func newStudentPresence(db *bun.DB, logger *slog.Logger) *studentpresence.Module {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: studentPresenceObserver(logger)})
	if err != nil {
		panic(err)
	}
	return module
}

// newStatistics builds the Statistik report over the same presence database
// and operation logging as newStudentPresence.
func newStatistics(db *bun.DB, logger *slog.Logger, deps presenceCompose.StatisticsDependencies) studentpresence.StatisticsReports {
	deps.DB = db
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
