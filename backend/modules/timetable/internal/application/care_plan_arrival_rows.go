package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (r CareArrivalScheduleRows) Create(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	created, err := r.rows.Records.CreateArrivalSchedule(ctx, arrivalScheduleToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("create", err)
	}
	applyArrivalSchedule(row, created)
	return nil
}
func (r CareArrivalScheduleRows) FindByID(ctx context.Context, raw int64) (*scheduleModels.StudentArrivalSchedule, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindArrivalSchedule(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	return arrivalScheduleToLegacy(value), nil
}
func (r CareArrivalScheduleRows) Update(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.ScheduleError("update", r.rows.Records.UpdateArrivalSchedule(ctx, arrivalScheduleToPublic(row)))
}
func (r CareArrivalScheduleRows) List(ctx context.Context, options *domain.ListOptions) ([]*scheduleModels.StudentArrivalSchedule, error) {
	return listWithOptions(ctx, r.rows, options, r.rows.Records.ListArrivalSchedules, arrivalScheduleToLegacy)
}
func (r CareArrivalScheduleRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.rows.Records.ListArrivalSchedules(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student id", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r CareArrivalScheduleRows) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.rows.Records.ListArrivalSchedules(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Weekday: weekday})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student id and weekday", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return arrivalScheduleToLegacy(values[0]), nil
}
func (r CareArrivalScheduleRows) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.rows.Records.ListArrivalSchedules(ctx, ports.CareScheduleFilter{StudentIDs: ids, Weekday: weekday})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student ids and weekday", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r CareArrivalScheduleRows) UpsertSchedule(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return errors.New("arrival schedule cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.UpsertArrivalSchedule(ctx, arrivalScheduleToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("upsert arrival schedule", err)
	}
	applyArrivalSchedule(row, value)
	return nil
}
func (r CareArrivalScheduleRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeleteArrivalSchedulesByStudent(ctx, id))
}

func (r CareArrivalExceptionRows) Create(ctx context.Context, row *scheduleModels.StudentArrivalException) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.CreateArrivalException(ctx, arrivalExceptionToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("create", err)
	}
	applyArrivalException(row, value)
	return nil
}
func (r CareArrivalExceptionRows) FindByID(ctx context.Context, raw int64) (*scheduleModels.StudentArrivalException, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindArrivalException(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	return arrivalExceptionToLegacy(value), nil
}
func (r CareArrivalExceptionRows) Update(ctx context.Context, row *scheduleModels.StudentArrivalException) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.ScheduleError("update", r.rows.Records.UpdateArrivalException(ctx, arrivalExceptionToPublic(row)))
}
func (r CareArrivalExceptionRows) List(ctx context.Context, options *domain.ListOptions) ([]*scheduleModels.StudentArrivalException, error) {
	return listWithOptions(ctx, r.rows, options, r.rows.Records.ListArrivalExceptions, arrivalExceptionToLegacy)
}
func (r CareArrivalExceptionRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r CareArrivalExceptionRows) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: r.rows.Today()}, "find upcoming by student id")
}
func (r CareArrivalExceptionRows) FindByStudentIDAndDate(ctx context.Context, id int64, d calendar.Date) (*scheduleModels.StudentArrivalException, error) {
	values, err := r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Date: ports.CareDate(d)}, "find by student id and date")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r CareArrivalExceptionRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d calendar.Date) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: ids, Date: ports.CareDate(d)}, "find by student ids and date")
}
func (r CareArrivalExceptionRows) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, From: ports.CareDate(from), To: ports.CareDate(to)}, "find by student id and date range")
}
func (r CareArrivalExceptionRows) list(ctx context.Context, filter ports.CareScheduleFilter, op string) ([]*scheduleModels.StudentArrivalException, error) {
	values, err := r.rows.Records.ListArrivalExceptions(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError(op, err)
	}
	return mapRows(values, arrivalExceptionToLegacy), nil
}
func (r CareArrivalExceptionRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeleteArrivalExceptionsByStudent(ctx, id))
}
func (r CareArrivalExceptionRows) DeletePastExceptions(ctx context.Context, d calendar.Date) (int64, error) {
	rows, err := r.rows.Records.DeleteArrivalExceptionsBefore(ctx, ports.CareDate(d))
	return rows, r.rows.ScheduleError("delete past exceptions", err)
}
