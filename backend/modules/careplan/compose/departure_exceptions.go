package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (e *exceptionQueries) ListDepartureExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.DepartureException, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListDepartureExceptions(ctx, f)

	err = mapError(err)
	e.observeRequest("list_departure_exceptions", started, stats, err)
	return value, err
}

// The day departure writes (#3945) have no rule of their own in the store
// service: the Module validated the value, and the store upserts or deletes
// the one row of the child's day inside the tenant transaction.
func (e engine) SaveDepartureException(ctx context.Context, v careplan.DepartureException) (result careplan.DepartureException, err error) {
	started := time.Now()
	var stats domain.OperationStats
	err = e.withinTenant(ctx, func(tx context.Context) error {
		result, stats, err = e.statusQueries.SaveDepartureException(tx, v)
		return err
	})
	err = mapError(err)
	e.observeRequest("save_departure_exception", started, stats, err)
	return result, err
}
func (e engine) DeleteDepartureException(ctx context.Context, id int64) error {
	started := time.Now()
	var stats domain.OperationStats
	err := e.withinTenant(ctx, func(tx context.Context) error {
		var deleteErr error
		stats, deleteErr = e.statusQueries.DeleteDepartureException(tx, id)
		return deleteErr
	})
	err = mapError(err)
	e.observeRequest("delete_departure_exception", started, stats, err)
	return err
}
