package application

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type ClassArrivalRecords interface {
	ListClassArrivalPlans(context.Context, []string) ([]timetable.ClassArrivalPlan, domain.OperationStats, error)
	ListClassArrivalExceptionRecords(context.Context, []string, string, string) ([]timetable.ClassArrivalException, domain.OperationStats, error)
	SaveClassArrivalPlan(context.Context, timetable.ClassArrivalPlan) (timetable.ClassArrivalPlan, domain.OperationStats, error)
	SaveClassArrivalException(context.Context, timetable.ClassArrivalException) (timetable.ClassArrivalException, domain.OperationStats, error)
	RemoveClassArrivalException(context.Context, string, string) (bool, domain.OperationStats, error)
	LockClassArrivalPlan(context.Context, string) (domain.OperationStats, error)
}

func NewClassArrivalQueries(store ClassArrivalRecords, tx internalports.Transaction, observe func(internalports.Observation)) timetable.ClassArrivalQuery {
	return classArrivalQueries{store: classArrivalCommands{store, tx}, observe: observe}
}
func NewClassArrivals(store ClassArrivalRecords, tx internalports.Transaction, observe func(internalports.Observation)) timetable.ClassArrivals {
	return classArrivalExceptions{classArrivalQueries{store: classArrivalCommands{store, tx}, observe: observe}}
}

type classArrivalCommands struct {
	ClassArrivalRecords
	tx internalports.Transaction
}

func (c classArrivalCommands) ListClassArrivalPlans(ctx context.Context, classes []string) (result []timetable.ClassArrivalPlan, stats domain.OperationStats, err error) {
	err = c.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		result, stats, err = c.ClassArrivalRecords.ListClassArrivalPlans(txCtx, classes)
		return err
	})
	return
}
func (c classArrivalCommands) ListClassArrivalExceptionRecords(ctx context.Context, classes []string, from, to string) (result []timetable.ClassArrivalException, stats domain.OperationStats, err error) {
	err = c.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		result, stats, err = c.ClassArrivalRecords.ListClassArrivalExceptionRecords(txCtx, classes, from, to)
		return err
	})
	return
}
func (c classArrivalCommands) SaveClassArrivalPlan(ctx context.Context, row timetable.ClassArrivalPlan) (result timetable.ClassArrivalPlan, stats domain.OperationStats, err error) {
	err = c.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		result, stats, err = c.ClassArrivalRecords.SaveClassArrivalPlan(txCtx, row)
		return err
	})
	return
}
func (c classArrivalCommands) SaveClassArrivalException(ctx context.Context, row timetable.ClassArrivalException) (result timetable.ClassArrivalException, stats domain.OperationStats, err error) {
	err = c.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		result, stats, err = c.ClassArrivalRecords.SaveClassArrivalException(txCtx, row)
		return err
	})
	return
}
func (c classArrivalCommands) RemoveClassArrivalException(ctx context.Context, class, date string) (result bool, stats domain.OperationStats, err error) {
	err = c.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		result, stats, err = c.ClassArrivalRecords.RemoveClassArrivalException(txCtx, class, date)
		return err
	})
	return
}

type classArrivalQueries struct {
	store   ClassArrivalRecords
	observe func(internalports.Observation)
}

func (q classArrivalQueries) ListClassArrivalTimes(ctx context.Context, classes []string) (map[string]map[string]string, error) {
	rows, err := q.ListClassArrivalPlans(ctx, classes)
	if err != nil {
		return nil, err
	}
	times := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		times[strings.ToLower(strings.TrimSpace(row.SchoolClass))] = row.ArrivalTimes
	}
	return times, nil
}

func (q classArrivalQueries) ListClassArrivalPlans(ctx context.Context, classes []string) ([]timetable.ClassArrivalPlan, error) {
	keys := classArrivalKeys(classes)
	if len(keys) == 0 {
		return []timetable.ClassArrivalPlan{}, nil
	}
	started := time.Now()
	rows, stats, err := q.store.ListClassArrivalPlans(ctx, keys)
	q.observe(internalports.Observation{Operation: "list_class_arrival_times", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, err
	}
	result := make([]timetable.ClassArrivalPlan, len(rows))
	for i, row := range rows {
		result[i] = nativeClassArrivalPlan(row)
	}
	return result, nil
}

func (q classArrivalQueries) ListClassArrivalExceptions(ctx context.Context, classes []string, from, to string) ([]timetable.ClassArrivalException, error) {
	if !timetable.ValidClassArrivalExceptionDates(from, to) {
		return nil, timetable.ErrInvalidClassArrivalExceptionRange
	}
	keys := classArrivalKeys(classes)
	if len(keys) == 0 || to < from {
		return []timetable.ClassArrivalException{}, nil
	}
	started := time.Now()
	rows, stats, err := q.store.ListClassArrivalExceptionRecords(ctx, keys, from, to)
	q.observe(internalports.Observation{Operation: "list_class_arrival_exceptions", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, err
	}
	result := make([]timetable.ClassArrivalException, len(rows))
	for i, row := range rows {
		result[i] = nativeClassArrivalException(row)
	}
	return result, nil
}

func classArrivalKeys(classes []string) []string {
	keys := make([]string, 0, len(classes))
	for _, class := range classes {
		if key := strings.ToLower(strings.TrimSpace(class)); key != "" {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

type classArrivalExceptions struct {
	classArrivalQueries
}

func (s classArrivalExceptions) UpsertClassArrivalException(ctx context.Context, input timetable.ClassArrivalExceptionInput) (timetable.ClassArrivalException, error) {
	if !input.Valid() {
		return timetable.ClassArrivalException{}, timetable.ErrInvalidClassArrivalException
	}
	started := time.Now()
	row, stats, err := s.store.SaveClassArrivalException(ctx, timetable.ClassArrivalException{SchoolClass: input.SchoolClass, Date: input.Date,
		ArrivalTime: input.ArrivalTime, Reason: input.Reason, CreatedBy: input.CreatedBy, Origin: input.Origin})
	s.observe(internalports.Observation{Operation: "upsert_class_arrival_exception", Duration: time.Since(started), Stats: stats, Err: err})
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
	s.observe(internalports.Observation{Operation: "delete_class_arrival_exception", Duration: time.Since(started), Stats: stats, Err: err})
	return deleted, err
}

func nativeClassArrivalException(row timetable.ClassArrivalException) timetable.ClassArrivalException {
	return timetable.ClassArrivalException{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, Date: row.Date, ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin}
}

func (s classArrivalExceptions) LockClassArrivalPlan(ctx context.Context, class string) error {
	if strings.TrimSpace(class) == "" {
		return timetable.ErrInvalidClassArrivalPlan
	}
	started := time.Now()
	stats, err := s.store.LockClassArrivalPlan(ctx, strings.ToLower(strings.TrimSpace(class)))
	s.observe(internalports.Observation{Operation: "lock_class_arrival_plan", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (s classArrivalExceptions) UpsertClassArrivalPlan(ctx context.Context, input timetable.ClassArrivalPlanInput) (timetable.ClassArrivalPlan, error) {
	if !input.Valid() {
		return timetable.ClassArrivalPlan{}, timetable.ErrInvalidClassArrivalPlan
	}
	started := time.Now()
	row, stats, err := s.store.SaveClassArrivalPlan(ctx, timetable.ClassArrivalPlan{SchoolClass: input.SchoolClass, ArrivalTimes: input.ArrivalTimes, UpdatedBy: input.UpdatedBy})
	s.observe(internalports.Observation{Operation: "upsert_class_arrival_plan", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return timetable.ClassArrivalPlan{}, err
	}
	return nativeClassArrivalPlan(row), nil
}

func nativeClassArrivalPlan(row timetable.ClassArrivalPlan) timetable.ClassArrivalPlan {
	return timetable.ClassArrivalPlan{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy}
}
