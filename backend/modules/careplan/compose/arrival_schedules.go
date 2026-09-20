package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// ArrivalScheduleRecords is the owner's storage capability used by arrival commands.
type ArrivalScheduleRecords interface {
	FindArrivalSchedule(context.Context, int64) (careplan.ArrivalSchedule, error)
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	CreateArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	DeleteArrivalSchedule(context.Context, int64) error
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
	UpsertArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	FindArrivalException(context.Context, int64, bool) (careplan.ArrivalException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	CreateArrivalException(context.Context, careplan.ArrivalException) (careplan.ArrivalException, error)
	DeleteArrivalException(context.Context, int64) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	UpdateArrivalException(context.Context, careplan.ArrivalException) error
	FindArrivalNote(context.Context, int64) (careplan.ArrivalNote, error)
	ListArrivalNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error)
	CreateArrivalNote(context.Context, careplan.ArrivalNote) (careplan.ArrivalNote, error)
	DeleteArrivalNote(context.Context, int64) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	UpdateArrivalNote(context.Context, careplan.ArrivalNote) error
}

type ArrivalScheduleRules = ports.ArrivalScheduleRules
type ArrivalRuleStudents = ports.ArrivalRuleStudents
type ArrivalBulkStudents = ports.ArrivalBulkStudents
type ArrivalBulkStudent = ports.ArrivalBulkStudent
type ArrivalClassPlans = ports.ArrivalClassPlans
type ArrivalClassPlan = ports.ArrivalClassPlan

func NewArrivalScheduleRules(students ArrivalRuleStudents, classes ArrivalClassPlans) ArrivalScheduleRules {
	return application.NewArrivalScheduleRules(students, classes)
}

type ArrivalScheduleDependencies struct {
	Students        ArrivalBulkStudents
	Classes         ArrivalClassPlans
	ClassExceptions careplan.ClassArrivalExceptions
	Logger          *slog.Logger
}

func NewArrivalSchedules(db *bun.DB, records ArrivalScheduleRecords, baselines careplan.ArrivalBaselineReader, rules ArrivalScheduleRules, deps ArrivalScheduleDependencies) (careplan.ArrivalScheduleService, error) {
	if db == nil || records == nil {
		return nil, errors.New("arrival schedules: database and records are required")
	}
	return application.NewArrivalSchedules(arrivalScheduleRecords{records}, arrivalExceptionRecords{records}, arrivalNoteRecords{records}, baselines, rules, pickupScheduleTransaction{excusalTransaction{db}}, deps.Students, deps.Classes, deps.ClassExceptions, deps.Logger), nil
}

type arrivalScheduleRecords struct{ ArrivalScheduleRecords }

func (r arrivalScheduleRecords) FindByID(ctx context.Context, id any) (*careplan.ArrivalSchedule, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindArrivalSchedule(ctx, value)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r arrivalScheduleRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.ArrivalSchedule, error) {
	rows, err := r.ListArrivalSchedules(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ArrivalSchedule, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r arrivalScheduleRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalSchedule, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r arrivalScheduleRecords) Create(ctx context.Context, row *careplan.ArrivalSchedule) error {
	result, err := r.CreateArrivalSchedule(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r arrivalScheduleRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeleteArrivalSchedule(ctx, value)
}
func (r arrivalScheduleRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeleteArrivalSchedulesByStudent(ctx, id)
}
func (r arrivalScheduleRecords) UpsertSchedule(ctx context.Context, row *careplan.ArrivalSchedule) error {
	result, err := r.UpsertArrivalSchedule(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r arrivalScheduleRecords) FindByStudentIDs(ctx context.Context, ids []int64) ([]*careplan.ArrivalSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalSchedule{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
}
func (r arrivalScheduleRecords) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*careplan.ArrivalSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalSchedule{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
}
func (r arrivalScheduleRecords) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*careplan.ArrivalSchedule, error) {
	rows, err := r.FindByStudentIDsAndWeekday(ctx, []int64{id}, weekday)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

type arrivalExceptionRecords struct{ ArrivalScheduleRecords }

func (r arrivalExceptionRecords) FindByID(ctx context.Context, id any) (*careplan.ArrivalException, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindArrivalException(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r arrivalExceptionRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.ArrivalException, error) {
	rows, err := r.ListArrivalExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ArrivalException, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r arrivalExceptionRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r arrivalExceptionRecords) Create(ctx context.Context, row *careplan.ArrivalException) error {
	result, err := r.CreateArrivalException(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r arrivalExceptionRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeleteArrivalException(ctx, value)
}
func (r arrivalExceptionRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeleteArrivalExceptionsByStudent(ctx, id)
}
func (r arrivalExceptionRecords) Update(ctx context.Context, row *careplan.ArrivalException) error {
	return r.UpdateArrivalException(ctx, *row)
}
func (r arrivalExceptionRecords) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*careplan.ArrivalException, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalException{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r arrivalExceptionRecords) FindByIDForUpdate(ctx context.Context, id any) (*careplan.ArrivalException, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindArrivalException(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r arrivalExceptionRecords) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (r arrivalExceptionRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*careplan.ArrivalException, error) {
	rows, err := r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

type arrivalNoteRecords struct{ ArrivalScheduleRecords }

func (r arrivalNoteRecords) FindByID(ctx context.Context, id any) (*careplan.ArrivalNote, error) {
	value, err := pickupRecordID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.FindArrivalNote(ctx, value)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r arrivalNoteRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.ArrivalNote, error) {
	rows, err := r.ListArrivalNotes(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ArrivalNote, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, nil
}
func (r arrivalNoteRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r arrivalNoteRecords) Create(ctx context.Context, row *careplan.ArrivalNote) error {
	result, err := r.CreateArrivalNote(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (r arrivalNoteRecords) Delete(ctx context.Context, id any) error {
	value, err := pickupRecordID(id)
	if err != nil {
		return err
	}
	return r.DeleteArrivalNote(ctx, value)
}
func (r arrivalNoteRecords) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.DeleteArrivalNotesByStudent(ctx, id)
}
func (r arrivalNoteRecords) Update(ctx context.Context, row *careplan.ArrivalNote) error {
	return r.UpdateArrivalNote(ctx, *row)
}
func (r arrivalNoteRecords) FindByStudentIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*careplan.ArrivalNote, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalNote{}, nil
	}
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: careplan.Date(date)})
}
func (r arrivalNoteRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) ([]*careplan.ArrivalNote, error) {
	return r.FindByStudentIDsAndDate(ctx, []int64{id}, date)
}
