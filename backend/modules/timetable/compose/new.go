// Package compose wires the Timetable & Activities module over the shared
// tenant runtime and Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	nativeports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Students nativeports.StudentDirectory
	Rooms    timetable.RoomDirectory
	// Sessions answers which planned rows Student Presence already executed
	// or observed; Timetable never reads that owner's tables itself (#2762).
	Sessions timetable.SessionFacts
	// LockStaffAssignment locks and validates live Membership staff in the
	// ambient transaction before creating or reassigning an operational link.
	LockStaffAssignment func(context.Context, int64) error
	Observe             func(Observation)
	Provenance          postgres.EnrollmentProvenance
}

func New(dependencies Dependencies) (*timetable.Module, error) {
	if dependencies.DB == nil || dependencies.Students == nil || dependencies.Rooms == nil || dependencies.Sessions == nil || dependencies.LockStaffAssignment == nil || dependencies.Observe == nil || dependencies.Provenance == nil {
		return nil, errors.New("timetable compose: all dependencies are required")
	}
	store := postgres.New(postgres.DatabaseRuntime(dependencies.DB))
	observe := application.NewObserver(dependencies.Observe)
	service := application.New(store, transaction{}, application.NewStudentDirectory(dependencies.Students), application.NewRoomDirectory(dependencies.Rooms), dependencies.Sessions,
		dependencies.LockStaffAssignment, application.NewToday(), observe)
	return timetable.NewModule(application.NewEngine(service, observe, postgres.NewProvenanceStore(store, dependencies.Provenance))), nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, retry bool, callback func(context.Context) error) error {
	return postgres.TransactionRunWriteBinding(ctx, retry, callback, tenant.WithinCurrentTenant, tenant.WithinTenantRetry)
}
