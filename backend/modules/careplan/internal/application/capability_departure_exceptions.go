package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) ListDepartureExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.DepartureException, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.DepartureException, domain.OperationStats, error) {
		return s.statusQueries.ListDepartureExceptions(txCtx, filter)
	})

	s.observeRequest("list_departure_exceptions", started, stats, err)
	return value, err
}

// The day departure writes (#3945) have no rule of their own in the store:
// the service validates the value, and the store upserts or deletes the one
// row of the child's day inside the tenant transaction.
func (s *Service) SaveDepartureException(ctx context.Context, v careplan.DepartureException) (result careplan.DepartureException, err error) {
	if !validateDepartureException(v) {
		return careplan.DepartureException{}, scheduleInvalid()
	}

	started := time.Now()
	var stats domain.OperationStats
	err = s.withinTenant(ctx, func(tx context.Context) error {
		result, stats, err = s.statusQueries.SaveDepartureException(tx, v)
		return err
	})
	s.observeRequest("save_departure_exception", started, stats, err)
	return result, err
}

func (s *Service) DeleteDepartureException(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	started := time.Now()
	var stats domain.OperationStats
	err := s.withinTenant(ctx, func(tx context.Context) error {
		var deleteErr error
		stats, deleteErr = s.statusQueries.DeleteDepartureException(tx, id)
		return deleteErr
	})
	s.observeRequest("delete_departure_exception", started, stats, err)
	return err
}
