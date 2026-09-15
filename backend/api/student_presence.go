package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/uptrace/bun"
)

func newStudentPresence(db *bun.DB, logger *slog.Logger) *studentpresence.Module {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(o presenceCompose.Observation) {
		logger.Debug("student presence operation",
			"operation", o.Operation,
			"duration", o.Duration,
			"queries", o.Queries,
			"rows", o.Rows,
			"error", o.Err,
		)
	}})
	if err != nil {
		panic(err)
	}
	return module
}
