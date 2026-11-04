// Package compose wires the School Calendar module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/openholidays"
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
		Database: postgres.NewPersistenceDatabase(),
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
func NewRecords(dependencies RecordsDependencies) (schoolcalendar.Days, error) {
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.FederalState == nil {
		return nil, errors.New("school calendar compose: all dependencies are required")
	}
	// The records serve no period administration, so its gate and guard
	// are never asked.
	service := retainedNewService(dependencies.DB, dependencies.Observe, application.Administration{Transaction: writeTransaction, Today: application.BerlinToday, FederalState: dependencies.FederalState})
	return service, nil
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
func NewAdministration(records schoolcalendar.Days, dependencies AdministrationDependencies) (schoolcalendar.Calendar, error) {
	if records == nil || dependencies.DB == nil || dependencies.Observe == nil ||
		dependencies.RecurrenceGate == nil || dependencies.CareOfferingGuard == nil {
		return nil, errors.New("school calendar compose: all dependencies are required")
	}
	today := dependencies.Today
	if today == nil {
		today = application.BerlinToday
	}
	service, err := application.WithAdministration(records, application.Administration{
		Transaction: writeTransaction, RecurrenceGate: dependencies.RecurrenceGate,
		CareOfferingGuard: dependencies.CareOfferingGuard, Today: today,
	}, dependencies.Observe)
	return service, err
}

// writeTransaction joins the ambient tenant transaction (tenant middleware,
// recurrence gate, scheduler loops) and otherwise opens one for the tenant
// in context, so the recurrence gate always holds a transaction-scoped lock.
func writeTransaction(ctx context.Context, fn func(context.Context) error) error {
	return postgres.WriteTransactionBinding(ctx, fn, tenant.WithinCurrentTenant)
}

// NewSchoolVacations composes the school vacations (#5345) over the ambient
// transaction and the OpenHolidays API at sourceURL ("" is the public API).
// Without federalState it serves only the sync; nil today is the Berlin day.
func NewSchoolVacations(federalState func(context.Context) (string, error), sourceURL string, today func() string) (schoolcalendar.SchoolVacationOwner, error) {
	return application.NewSchoolVacations(application.SchoolVacationDependencies{
		Store: postgres.NewSchoolVacationStore(), Source: openholidays.New(nil, sourceURL, application.SchoolVacationRegions()).SchoolVacations,
		FederalState: federalState, Today: today, AdminTransaction: tenant.WithinAdmin,
	})
}
