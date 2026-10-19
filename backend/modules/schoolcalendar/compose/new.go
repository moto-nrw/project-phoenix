// Package compose wires the School Calendar module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// PersistenceRuntime exposes the compose-owned ambient tenant transaction to
// legacy School Calendar persistence adapters during the migration.
type PersistenceRuntime struct {
	Database func(context.Context) (bun.IDB, error)
	TenantID func(context.Context) int64
}

// PersistenceRuntimeFor builds the migration bridge for legacy calendar
// repositories without leaking tenant-runtime imports into those adapters.
func PersistenceRuntimeFor(db *bun.DB) PersistenceRuntime {
	if db == nil {
		panic("school calendar compose: database is required")
	}
	return PersistenceRuntime{
		Database: func(ctx context.Context) (bun.IDB, error) {
			db, err := tenant.CurrentTransaction[bun.IDB](ctx)
			if err != nil {
				return nil, fmt.Errorf("school calendar repository: tenant is required for the ambient transaction: %w", err)
			}
			return db, nil
		},
		TenantID: tenant.FromContext,
	}
}

// RecordsDependencies are the bindings of the Records: the database and,
// for the tenant holiday reads, the federal state.
type RecordsDependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// FederalState resolves the tenant's federal-state ISO code (the
	// operations.federal_state setting).
	FederalState func(context.Context) (string, error)
}

// NewRecords composes the School Calendar without the period
// administration, for the owners a root builds before the ones the
// administration asks (#4080). NewAdministration adds the administration
// once its gate and guard exist.
func NewRecords(dependencies RecordsDependencies) (*schoolcalendar.Records, error) {
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.FederalState == nil {
		return nil, errors.New("school calendar compose: all dependencies are required")
	}
	// The records serve no period administration, so its gate and guard
	// are never asked.
	service := newService(dependencies.DB, dependencies.Observe, application.Administration{Transaction: writeTransaction, Today: application.BerlinToday})
	return schoolcalendar.NewRecords(application.NewPublicEngine(service, dependencies.FederalState)), nil
}

// AdministrationDependencies are the collaborators of the period
// administration.
type AdministrationDependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// RecurrenceGate serializes period administration against every
	// recurrence-derived write of the tenant: the shared tenant recurrence
	// lock.
	RecurrenceGate func(context.Context) error
	// CareOfferingGuard refuses a period change (replacement set) or
	// removal (replacement nil) a linked care offering still needs, as
	// schoolcalendar.ErrCalendarPeriodRequiredByCareOffering in the chain.
	CareOfferingGuard func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error
	// Today names the current calendar day in YYYY-MM-DD. Nil means the
	// Berlin calendar day, the school's day.
	Today func() string
}

// NewAdministration adds the period administration to the Records: the
// whole School Calendar, which the period editor drives.
func NewAdministration(records *schoolcalendar.Records, dependencies AdministrationDependencies) (*schoolcalendar.Module, error) {
	if records == nil || dependencies.DB == nil || dependencies.Observe == nil ||
		dependencies.RecurrenceGate == nil || dependencies.CareOfferingGuard == nil {
		return nil, errors.New("school calendar compose: all dependencies are required")
	}
	today := dependencies.Today
	if today == nil {
		today = application.BerlinToday
	}
	service := newService(dependencies.DB, dependencies.Observe, application.Administration{
		Transaction: writeTransaction, RecurrenceGate: dependencies.RecurrenceGate,
		CareOfferingGuard: application.BindCareOfferingGuard(dependencies.CareOfferingGuard), Today: today,
	})
	return schoolcalendar.NewAdministeredModule(records, application.NewPublicEngine(service, nil)), nil
}

func newService(db *bun.DB, observe func(Observation), administration application.Administration) *application.Service {
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		return postgres.ResolveDatabase(raw, ok, id.Int64(), scopeErr)
	})
	return application.New(store, func(observation Observation) {
		observation.Err = application.MapPublicError(observation.Err)
		observe(observation)
	}, administration)
}

// writeTransaction joins the ambient tenant transaction (tenant middleware,
// recurrence gate, scheduler loops) and otherwise opens one for the tenant
// in context, so the recurrence gate always holds a transaction-scoped lock.
func writeTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return fmt.Errorf("school calendar: tenant is required: %w", err)
	}
	return tenant.WithinCurrentTenant(ctx, fn)
}
