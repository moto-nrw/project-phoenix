// Package compose wires the Care Plan module over the shared tenant runtime
// and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careplanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation
type AmbientDatabase func(context.Context) bun.IDB
type StatusStudentDirectory = postgres.StatusStudentDirectory
type StatusStudent = postgres.StatusStudent
type StatusSlotDirectory = postgres.StatusSlotDirectory
type StudentDirectory = postgres.StudentDirectory

type Dependencies struct {
	DB               *bun.DB
	Observe          func(Observation)
	AmbientDB        AmbientDatabase
	StatusStudents   StatusStudentDirectory
	StatusSlots      StatusSlotDirectory
	People           careplanPorts.StudentNameFinder
	StudentLock      careplanning.StudentLock
	StudentNotFound  error
	StudentDirectory StudentDirectory
}

func New(dependencies Dependencies) (*careplan.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.AmbientDB == nil || dependencies.StatusStudents == nil || dependencies.StatusSlots == nil ||
		dependencies.People == nil || dependencies.StudentLock == nil || dependencies.StudentNotFound == nil || dependencies.StudentDirectory == nil {
		return nil, errors.New("care plan compose: database, observer, ambient database resolver, people directory, status-student directory, status-slot directory, student lock, and student directory are required")
	}
	bindCareLocks(dependencies)
	store := postgres.New(carePlanDatabase(dependencies.DB))
	queries := application.NewExceptionQueryService(store, dependencies.Observe)
	statusDays := postgres.NewStatusDayStore(store, dependencies.StatusStudents, dependencies.StatusSlots)
	module := careplan.NewModule(application.NewEngine(application.EngineDependencies{
		OfferingBookings: NewOfferingBookings(),
		Exceptions:       queries, Requests: postgres.NewRequestStore(store, dependencies.StudentDirectory), StatusDays: statusDays,
		Withdrawals: postgres.NewDirectoryStore(store, dependencies.StudentDirectory), Observe: dependencies.Observe, People: dependencies.People,
		Runtime: engineRuntime{db: dependencies.DB},
	}))
	return module, nil
}

func bindCareLocks(dependencies Dependencies) {
	careplanning.BindStudentLockForDB(dependencies.DB, dependencies.StudentLock, dependencies.StudentNotFound)
	careplanning.BindExceptionDayLockForDB(dependencies.DB, func(ctx context.Context, studentID int64, date string) error {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return errors.New("care plan: tenant is required for exception-day lock")
		}
		return postgres.LockExceptionDay(ctx, dependencies.AmbientDB(ctx), tenantID, studentID, date)
	})
}

func carePlanDatabase(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return nil, 0, errors.New("care plan: tenant is required")
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("care plan: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return nil, 0, errors.New("care plan: transaction is required")
		default:
			return nil, 0, fmt.Errorf("care plan postgres: unsupported transaction %T", transaction)
		}
	}
}

// engineRuntime binds the engine's tenant commands and locks to the tenant
// runtime and the database.
type engineRuntime struct{ db *bun.DB }

func (r engineRuntime) WithinTenant(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	validatedTenantID, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return errors.New("care plan: tenant is required for commands")
	}
	return tenant.WithinTenant(ctx, validatedTenantID, command)
}

func (r engineRuntime) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	if _, _, err := carePlanDatabase(r.db)(ctx); err != nil {
		return err
	}
	return careplanning.LockStudentAndExceptionDay(ctx, r.db, studentID, date)
}

func (r engineRuntime) LockExceptionDay(ctx context.Context, studentID int64, date string) error {
	return careplanning.LockExceptionDay(ctx, r.db, studentID, date)
}

func (r engineRuntime) LockStudent(ctx context.Context, studentID int64) error {
	return careplanning.LockStudent(ctx, r.db, studentID)
}
