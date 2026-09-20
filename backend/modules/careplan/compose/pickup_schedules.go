package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// PickupScheduleRecords is the owner's storage capability used by pickup commands.
type PickupScheduleRecords interface {
	FindPickupSchedule(context.Context, int64) (careplan.PickupSchedule, error)
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	CreatePickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
	DeletePickupSchedulesByStudent(context.Context, int64) error
	UpsertPickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	DeletePickupException(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	UpdatePickupException(context.Context, careplan.PickupException) error
	FindPickupNote(context.Context, int64) (careplan.PickupNote, error)
	ListPickupNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupNote, error)
	CreatePickupNote(context.Context, careplan.PickupNote) (careplan.PickupNote, error)
	DeletePickupNote(context.Context, int64) error
	DeletePickupNotesByStudent(context.Context, int64) error
	UpdatePickupNote(context.Context, careplan.PickupNote) error
}

type PickupBulkStudents = ports.PickupBulkStudents
type PickupBulkStudent = ports.PickupBulkStudent

func NewPickupSchedules(db *bun.DB, records PickupScheduleRecords, baselines careplan.PickupBaselineReader, auto careplan.PickupAutoExcusal, students PickupBulkStudents, logger *slog.Logger) (careplan.PickupScheduleService, error) {
	if db == nil || records == nil || baselines == nil {
		return nil, errors.New("pickup schedules: database, records, and baselines are required")
	}
	return application.NewPickupSchedules(pickupScheduleRecords{records}, pickupExceptionRecords{records},
		pickupNoteRecords{records}, pickupScheduleTransaction{excusalTransaction{db}}, baselines, auto, students, logger), nil
}

type pickupScheduleTransaction struct{ excusalTransaction }

func (t pickupScheduleTransaction) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	return careplanning.LockStudentAndExceptionDay(ctx, t.db, id, date)
}
func (t pickupScheduleTransaction) IsNotFound(err error) bool {
	return errors.Is(err, careplan.ErrStudentScheduleNotFound) || t.excusalTransaction.IsNotFound(err)
}

func pickupRecordID(id any) (int64, error) {
	value, ok := id.(int64)
	if !ok || value <= 0 {
		return 0, careplan.ErrInvalidStudentSchedule
	}
	return value, nil
}

type pickupScheduleRecords struct{ PickupScheduleRecords }

func (r pickupScheduleRecords) FindByID(ctx context.Context, id any) (*careplan.PickupSchedule, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindPickupSchedule(ctx, value)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r pickupScheduleRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupSchedule, error) {
	rows, err := r.ListPickupSchedules(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.PickupSchedule, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r pickupScheduleRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.PickupSchedule, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r pickupScheduleRecords) Create(ctx context.Context, row *careplan.PickupSchedule) error {
	result, err := r.CreatePickupSchedule(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r pickupScheduleRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeletePickupSchedule(ctx, value)
}
func (r pickupScheduleRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeletePickupSchedulesByStudent(ctx, id)
}
func (r pickupScheduleRecords) UpsertSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	result, err := r.UpsertPickupSchedule(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r pickupScheduleRecords) FindByStudentIDs(ctx context.Context, ids []int64) ([]*careplan.PickupSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.PickupSchedule{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
}
func (r pickupScheduleRecords) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*careplan.PickupSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.PickupSchedule{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
}
func (r pickupScheduleRecords) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*careplan.PickupSchedule, error) {
	rows, err := r.FindByStudentIDsAndWeekday(ctx, []int64{id}, weekday)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

type pickupExceptionRecords struct{ PickupScheduleRecords }

func (r pickupExceptionRecords) FindByID(ctx context.Context, id any) (*careplan.PickupException, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindPickupException(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r pickupExceptionRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupException, error) {
	rows, err := r.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.PickupException, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r pickupExceptionRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.PickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r pickupExceptionRecords) Create(ctx context.Context, row *careplan.PickupException) error {
	result, err := r.CreatePickupException(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r pickupExceptionRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeletePickupException(ctx, value)
}
func (r pickupExceptionRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeletePickupExceptionsByStudent(ctx, id)
}
func (r pickupExceptionRecords) Update(ctx context.Context, row *careplan.PickupException) error {
	return r.UpdatePickupException(ctx, *row)
}
func (r pickupExceptionRecords) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*careplan.PickupException, error) {
	if len(ids) == 0 {
		return []*careplan.PickupException{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r pickupExceptionRecords) FindByIDForUpdate(ctx context.Context, id any) (*careplan.PickupException, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindPickupException(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r pickupExceptionRecords) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*careplan.PickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (r pickupExceptionRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupException, error) {
	rows, err := r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

type pickupNoteRecords struct{ PickupScheduleRecords }

func (r pickupNoteRecords) FindByID(ctx context.Context, id any) (*careplan.PickupNote, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindPickupNote(ctx, value)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r pickupNoteRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupNote, error) {
	rows, err := r.ListPickupNotes(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.PickupNote, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r pickupNoteRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.PickupNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r pickupNoteRecords) Create(ctx context.Context, row *careplan.PickupNote) error {
	result, err := r.CreatePickupNote(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r pickupNoteRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeletePickupNote(ctx, value)
}
func (r pickupNoteRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeletePickupNotesByStudent(ctx, id)
}
func (r pickupNoteRecords) Update(ctx context.Context, row *careplan.PickupNote) error {
	return r.UpdatePickupNote(ctx, *row)
}
func (r pickupNoteRecords) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*careplan.PickupNote, error) {
	if len(ids) == 0 {
		return []*careplan.PickupNote{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r pickupNoteRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) ([]*careplan.PickupNote, error) {
	return r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
}
