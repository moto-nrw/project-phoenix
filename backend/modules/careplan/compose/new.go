// Package compose wires the Care Plan module over the shared tenant runtime
// and Bun database.
package compose

import (
	"context"
	"errors"

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
	postgres.BindCareLocks(dependencies.DB, dependencies.StudentLock, dependencies.StudentNotFound, dependencies.AmbientDB)
	store := postgres.New(postgres.CarePlanDatabase(dependencies.DB))
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

// engineRuntime binds the engine's tenant commands and locks to the tenant
// runtime and the database.
type engineRuntime struct{ db *bun.DB }

func (r engineRuntime) WithinTenant(ctx context.Context, command func(context.Context) error) error {
	return postgres.EngineRuntimeWithinTenantBinding(ctx, command, tenant.WithinTenant)
}

func (r engineRuntime) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	return postgres.LockEngineStudentDay(ctx, r.db, studentID, date)
}

func (r engineRuntime) LockExceptionDay(ctx context.Context, studentID int64, date string) error {
	return careplanning.LockExceptionDay(ctx, r.db, studentID, date)
}

func (r engineRuntime) LockStudent(ctx context.Context, studentID int64) error {
	return careplanning.LockStudent(ctx, r.db, studentID)
}

type AmbientDatabase func(context.Context) (bun.IDB, error)
