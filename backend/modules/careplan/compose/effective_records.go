package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// effectiveRecords adapts one value-based storage capability (schedules,
// exceptions, or notes of either direction) to the pointer-based repository
// ports of the effective-time commands. Unset functions belong to operations
// the respective port does not declare.
type effectiveRecords[T any] struct {
	find            func(context.Context, int64, bool) (T, error)
	list            func(context.Context, careplan.StudentScheduleFilter) ([]T, error)
	create          func(context.Context, T) (T, error)
	upsert          func(context.Context, T) (T, error)
	update          func(context.Context, T) error
	remove          func(context.Context, int64) error
	removeByStudent func(context.Context, int64) error
}

// unlocked lifts a finder without a row-lock variant into the find shape.
func unlocked[T any](find func(context.Context, int64) (T, error)) func(context.Context, int64, bool) (T, error) {
	return func(ctx context.Context, id int64, _ bool) (T, error) { return find(ctx, id) }
}

func (r effectiveRecords[T]) findByID(ctx context.Context, id int64, lock bool) (*T, error) {
	if id <= 0 {
		return nil, careplan.ErrInvalidStudentSchedule
	}
	row, err := r.find(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r effectiveRecords[T]) rows(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*T, error) {
	rows, err := r.list(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*T, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}

func (r effectiveRecords[T]) FindByID(ctx context.Context, id int64) (*T, error) {
	return r.findByID(ctx, id, false)
}
func (r effectiveRecords[T]) FindByIDForUpdate(ctx context.Context, id int64) (*T, error) {
	return r.findByID(ctx, id, true)
}
func (r effectiveRecords[T]) FindByStudentID(ctx context.Context, id int64) ([]*T, error) {
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r effectiveRecords[T]) FindByStudentIDs(ctx context.Context, ids []int64) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
}
func (r effectiveRecords[T]) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
}
func (r effectiveRecords[T]) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*T, error) {
	rows, err := r.FindByStudentIDsAndWeekday(ctx, []int64{id}, weekday)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}
func (r effectiveRecords[T]) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r effectiveRecords[T]) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*T, error) {
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (r effectiveRecords[T]) Create(ctx context.Context, row *T) error {
	result, err := r.create(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r effectiveRecords[T]) UpsertSchedule(ctx context.Context, row *T) error {
	result, err := r.upsert(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r effectiveRecords[T]) Update(ctx context.Context, row *T) error {
	return r.update(ctx, *row)
}
func (r effectiveRecords[T]) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return careplan.ErrInvalidStudentSchedule
	}
	return r.remove(ctx, id)
}
func (r effectiveRecords[T]) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.removeByStudent(ctx, id)
}

// exceptionRecords holds at most one exception per student and day.
type exceptionRecords[T any] struct{ effectiveRecords[T] }

func (r exceptionRecords[T]) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*T, error) {
	rows, err := r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// noteRecords may hold several notes per student and day.
type noteRecords[T any] struct{ effectiveRecords[T] }

func (r noteRecords[T]) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) ([]*T, error) {
	return r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
}
