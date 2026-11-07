package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// validated is the pointer form of a record that can check itself before storage.
type validated[T any] interface {
	*T
	Validate() error
}

// RecordFuncs are the value-based operations of one storage capability
// (schedules, exceptions, or notes of either direction). Unset functions
// belong to operations the respective port does not declare.
type RecordFuncs[T any] struct {
	Find            func(context.Context, int64, bool) (T, error)
	List            func(context.Context, careplan.StudentScheduleFilter) ([]T, error)
	Create          func(context.Context, T) (T, error)
	Upsert          func(context.Context, T) (T, error)
	Update          func(context.Context, T) error
	Remove          func(context.Context, int64) error
	RemoveByStudent func(context.Context, int64) error
}

// EffectiveRecords adapts one value-based storage capability to the
// pointer-based repository ports of the effective-time commands.
type EffectiveRecords[T any, P validated[T]] struct {
	find            func(context.Context, int64, bool) (T, error)
	list            func(context.Context, careplan.StudentScheduleFilter) ([]T, error)
	create          func(context.Context, T) (T, error)
	upsert          func(context.Context, T) (T, error)
	update          func(context.Context, T) error
	remove          func(context.Context, int64) error
	removeByStudent func(context.Context, int64) error
}

// NewEffectiveRecords binds the pointer-based record port to funcs.
func NewEffectiveRecords[T any, P validated[T]](funcs RecordFuncs[T]) EffectiveRecords[T, P] {
	return EffectiveRecords[T, P]{
		find: funcs.Find, list: funcs.List, create: funcs.Create, upsert: funcs.Upsert,
		update: funcs.Update, remove: funcs.Remove, removeByStudent: funcs.RemoveByStudent,
	}
}

// NewExceptionRecords binds the exception record port to funcs.
func NewExceptionRecords[T any, P validated[T]](funcs RecordFuncs[T]) ExceptionRecords[T, P] {
	return ExceptionRecords[T, P]{NewEffectiveRecords[T, P](funcs)}
}

// NewNoteRecords binds the note record port to funcs.
func NewNoteRecords[T any, P validated[T]](funcs RecordFuncs[T]) NoteRecords[T, P] {
	return NoteRecords[T, P]{NewEffectiveRecords[T, P](funcs)}
}

// Unlocked lifts a finder without a row-lock variant into the find shape.
func Unlocked[T any](find func(context.Context, int64) (T, error)) func(context.Context, int64, bool) (T, error) {
	return func(ctx context.Context, id int64, _ bool) (T, error) { return find(ctx, id) }
}

func (r EffectiveRecords[T, P]) findByID(ctx context.Context, id int64, lock bool) (*T, error) {
	if id <= 0 {
		return nil, careplan.ErrInvalidStudentSchedule
	}
	row, err := r.find(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r EffectiveRecords[T, P]) rows(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*T, error) {
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

func (r EffectiveRecords[T, P]) FindByID(ctx context.Context, id int64) (*T, error) {
	return r.findByID(ctx, id, false)
}
func (r EffectiveRecords[T, P]) FindByIDForUpdate(ctx context.Context, id int64) (*T, error) {
	return r.findByID(ctx, id, true)
}
func (r EffectiveRecords[T, P]) FindByStudentID(ctx context.Context, id int64) ([]*T, error) {
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r EffectiveRecords[T, P]) FindByStudentIDs(ctx context.Context, ids []int64) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
}
func (r EffectiveRecords[T, P]) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
}
func (r EffectiveRecords[T, P]) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*T, error) {
	rows, err := r.FindByStudentIDsAndWeekday(ctx, []int64{id}, weekday)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}
func (r EffectiveRecords[T, P]) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*T, error) {
	if len(ids) == 0 {
		return []*T{}, nil
	}
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r EffectiveRecords[T, P]) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*T, error) {
	return r.rows(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (r EffectiveRecords[T, P]) Create(ctx context.Context, row *T) error {
	if err := P(row).Validate(); err != nil {
		return err
	}
	result, err := r.create(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r EffectiveRecords[T, P]) UpsertSchedule(ctx context.Context, row *T) error {
	if err := P(row).Validate(); err != nil {
		return err
	}
	result, err := r.upsert(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r EffectiveRecords[T, P]) Update(ctx context.Context, row *T) error {
	if err := P(row).Validate(); err != nil {
		return err
	}
	return r.update(ctx, *row)
}
func (r EffectiveRecords[T, P]) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return careplan.ErrInvalidStudentSchedule
	}
	return r.remove(ctx, id)
}
func (r EffectiveRecords[T, P]) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.removeByStudent(ctx, id)
}

// ExceptionRecords holds at most one exception per student and day.
type ExceptionRecords[T any, P validated[T]] struct{ EffectiveRecords[T, P] }

func (r ExceptionRecords[T, P]) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*T, error) {
	rows, err := r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// NoteRecords may hold several notes per student and day.
type NoteRecords[T any, P validated[T]] struct{ EffectiveRecords[T, P] }

func (r NoteRecords[T, P]) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) ([]*T, error) {
	return r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
}
