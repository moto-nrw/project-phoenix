package repositories

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/uptrace/bun"
)

func newStudentPresence(db *bun.DB) *studentpresence.Module {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(observation presenceCompose.Observation) {
		if observation.Err != nil {
			slog.Default().Warn("presence operation failed",
				"operation", observation.Operation,
				"error", observation.Err,
			)
		}
	}})
	if err != nil {
		panic(err)
	}
	return module
}
