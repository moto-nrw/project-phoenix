// Package compose wires the Timetable & Activities module over the shared
// tenant runtime and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	nativeports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	store := postgres.New(databaseRuntime(dependencies.DB))
	observe := func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(store, transaction{}, application.NewStudentDirectory(dependencies.Students), application.NewRoomDirectory(dependencies.Rooms), dependencies.Sessions,
		dependencies.LockStaffAssignment, func() string { return calendar.TodayDate().String() }, observe)
	return timetable.NewModule(application.NewEngine(service, observe, postgres.NewProvenanceStore(store, dependencies.Provenance))), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("timetable postgres: tenant is required: %w", err)
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("timetable postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return nil, 0, errors.New("timetable postgres: transaction is required")
		default:
			return nil, 0, fmt.Errorf("timetable postgres: unsupported transaction %T", transaction)
		}
	}
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, retry bool, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if !retry {
		if _, err := tenant.TenantFromContext(ctx); err != nil {
			return fmt.Errorf("timetable postgres: tenant is required: %w", err)
		}
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return tenant.WithinTenantRetry(ctx, tenantID, callback)
}
