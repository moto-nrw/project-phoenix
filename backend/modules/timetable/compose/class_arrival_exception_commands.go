package compose

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func NewClassArrivals(db *bun.DB, observe func(Observation)) (timetable.ClassArrivals, error) {
	if db == nil || observe == nil {
		return nil, errors.New("class arrival exceptions: database and observer are required")
	}
	store := postgres.New(databaseRuntime(db))
	return classArrivalExceptions{
		classArrivalQueries: classArrivalQueries{store: store, observe: observe},
	}, nil
}

type classArrivalExceptions struct {
	classArrivalQueries
}

func (s classArrivalExceptions) UpsertClassArrivalException(ctx context.Context, input timetable.ClassArrivalExceptionInput) (timetable.ClassArrivalException, error) {
	if !input.Valid() {
		return timetable.ClassArrivalException{}, timetable.ErrInvalidClassArrivalException
	}
	started := time.Now()
	row, stats, err := s.store.SaveClassArrivalException(ctx, domain.ClassArrivalException{SchoolClass: input.SchoolClass, Date: input.Date,
		ArrivalTime: input.ArrivalTime, Reason: input.Reason, CreatedBy: input.CreatedBy, Origin: input.Origin})
	s.observe(Observation{Operation: "upsert_class_arrival_exception", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return timetable.ClassArrivalException{}, err
	}
	return nativeClassArrivalException(row), nil
}

func (s classArrivalExceptions) DeleteClassArrivalException(ctx context.Context, class, date string) (bool, error) {
	if !timetable.ValidClassArrivalExceptionDates(date, date) {
		return false, timetable.ErrInvalidClassArrivalException
	}
	key := strings.ToLower(strings.TrimSpace(class))
	if key == "" {
		return false, nil
	}
	started := time.Now()
	deleted, stats, err := s.store.RemoveClassArrivalException(ctx, key, date)
	s.observe(Observation{Operation: "delete_class_arrival_exception", Duration: time.Since(started), Stats: stats, Err: err})
	return deleted, err
}

func nativeClassArrivalException(row domain.ClassArrivalException) timetable.ClassArrivalException {
	return timetable.ClassArrivalException{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, Date: row.Date, ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin}
}
