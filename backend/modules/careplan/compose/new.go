// Package compose wires the Care Plan module over the shared tenant runtime
// and Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/internal/careplanning"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	careplanPorts "github.com/moto-ogs/project-phoenix/modules/careplan/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
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

func New(dependencies Dependencies) (careplan.Owner, error) {
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.AmbientDB == nil || dependencies.StatusStudents == nil || dependencies.StatusSlots == nil ||
		dependencies.People == nil || dependencies.StudentLock == nil || dependencies.StudentNotFound == nil || dependencies.StudentDirectory == nil {
		return nil, errors.New("care plan compose: database, observer, ambient database resolver, people directory, status-student directory, status-slot directory, student lock, and student directory are required")
	}
	postgres.BindCareLocks(dependencies.DB, dependencies.StudentLock, dependencies.StudentNotFound, dependencies.AmbientDB)
	store := postgres.New(postgres.CarePlanDatabase(dependencies.DB))
	return application.NewService(application.Dependencies{
		OfferingBookings: NewOfferingBookings(),
		Store:            store, Requests: postgres.NewRequestStore(store, dependencies.StudentDirectory),
		StatusDays:  postgres.NewStatusDayStore(store, dependencies.StatusStudents, dependencies.StatusSlots),
		Withdrawals: postgres.NewDirectoryStore(store, dependencies.StudentDirectory), Observe: dependencies.Observe, People: dependencies.People,
		Runtime: engineRuntime{EngineLocks: postgres.EngineLocks{DB: dependencies.DB}},
	}), nil
}

type AmbientDatabase func(context.Context) (bun.IDB, error)

// engineRuntime binds the registered transaction boundary to the database locks.
type engineRuntime struct{ postgres.EngineLocks }

func (r engineRuntime) WithinTenant(ctx context.Context, command func(context.Context) error) error {
	return postgres.EngineRuntimeWithinTenantBinding(ctx, command, tenant.WithinTenant)
}

// NewExceptionBatch builds a participant over the caller's tenant transaction.
func NewExceptionBatch(observe func(Observation)) careplan.ExceptionBatchCommands {
	if observe == nil {
		panic("care plan exception batch: observer is required")
	}
	return application.NewExceptionQueryService(postgres.New(postgres.AmbientCarePlanDatabase), observe)
}
