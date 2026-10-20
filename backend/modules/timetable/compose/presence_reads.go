package compose

import (
	"context"
	"time"

	postgresNative "github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PresenceReads is the consumer-owned view used by the root's retained adapters.
type PresenceReads interface {
	application.PresenceReads
	ListPartialAbsenceBlocks(context.Context, int64, string, time.Time, bool, []int64) ([]scheduleModels.PartialAbsenceBlock, error)
}

func NewPresenceReads(projection ports.PresenceProjection) (PresenceReads, error) {
	return application.NewProjectedPresenceReads(projection, tenant.NewTransactionRunner().RunInTx)
}

// PresenceDatabase exposes the existing runtime for root binding, not a projection dependency.
func PresenceDatabase(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return postgresNative.DatabaseRuntime(db)
}
