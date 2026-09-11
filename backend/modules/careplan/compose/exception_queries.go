package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

// ExceptionQueries can be constructed before the Timetable status-slot
// commands that Care Plan mutations require.
type ExceptionQueries interface {
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	FindStudentStatusDay(context.Context, int64, bool) (careplan.StudentStatusDay, error)
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
}

type exceptionQueries struct {
	service       *application.Service
	statusQueries *postgres.Store
	observe       func(Observation)
}

func NewExceptionQueries(db *bun.DB, observe func(Observation)) (ExceptionQueries, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan exception queries: database and observer are required")
	}
	return newExceptionQueries(postgres.New(carePlanDatabase(db)), observe), nil
}

func newExceptionQueries(store *postgres.Store, observe func(Observation)) *exceptionQueries {
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		observe(observation)
	})
	return &exceptionQueries{service: service, statusQueries: store, observe: observe}
}

func (e *exceptionQueries) observeRequest(operation string, started time.Time, stats RequestStoreStats, err error) {
	e.observe(Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
}
