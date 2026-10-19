package application

import (
	"context"
	"errors"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (r CarePickupScheduleRows) Create(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.CreatePickupSchedule(ctx, pickupScheduleToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("create", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r CarePickupScheduleRows) FindByID(ctx context.Context, raw int64) (*scheduleModels.StudentPickupSchedule, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupSchedule(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	return pickupScheduleToLegacy(value), nil
}
func (r CarePickupScheduleRows) Update(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.ScheduleError("update", r.rows.Records.UpdatePickupSchedule(ctx, pickupScheduleToPublic(row)))
}
func (r CarePickupScheduleRows) ListRows(ctx context.Context, filter ports.CareScheduleFilter) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError("list with options", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r CarePickupScheduleRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student id", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r CarePickupScheduleRows) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Weekday: weekday})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student id and weekday", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return pickupScheduleToLegacy(values[0]), nil
}
func (r CarePickupScheduleRows) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, ports.CareScheduleFilter{StudentIDs: ids, Weekday: weekday})
	if err != nil {
		return nil, r.rows.ScheduleError("find by student ids and weekday", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r CarePickupScheduleRows) UpsertSchedule(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return errors.New("schedule cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.UpsertPickupSchedule(ctx, pickupScheduleToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("upsert schedule", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r CarePickupScheduleRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeletePickupSchedulesByStudent(ctx, id))
}

func (r CarePickupExceptionRows) Create(ctx context.Context, row *scheduleModels.StudentPickupException) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.CreatePickupException(ctx, pickupExceptionToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("create", err)
	}
	applyPickupException(row, value)
	return nil
}
func (r CarePickupExceptionRows) FindByID(ctx context.Context, raw int64) (*scheduleModels.StudentPickupException, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupException(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	return pickupExceptionToLegacy(value), nil
}
func (r CarePickupExceptionRows) Update(ctx context.Context, row *scheduleModels.StudentPickupException) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupException")
	}
	row.NormalizeWallClockTimes()
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.ScheduleError("update", r.rows.Records.UpdatePickupException(ctx, pickupExceptionToPublic(row)))
}
func (r CarePickupExceptionRows) ListRows(ctx context.Context, filter ports.CareScheduleFilter) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.rows.Records.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError("list with options", err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r CarePickupExceptionRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r CarePickupExceptionRows) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: r.rows.Today()}, "find upcoming by student id")
}
func (r CarePickupExceptionRows) FindByStudentIDAndDate(ctx context.Context, id int64, d calendar.Date) (*scheduleModels.StudentPickupException, error) {
	values, err := r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Date: ports.CareDate(d)}, "find by student id and date")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r CarePickupExceptionRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d calendar.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: ids, Date: ports.CareDate(d)}, "find by student ids and date")
}
func (r CarePickupExceptionRows) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, From: ports.CareDate(from), To: ports.CareDate(to)}, "find by student id and date range")
}
func (r CarePickupExceptionRows) list(ctx context.Context, filter ports.CareScheduleFilter, op string) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.rows.Records.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError(op, err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r CarePickupExceptionRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeletePickupExceptionsByStudent(ctx, id))
}
func (r CarePickupExceptionRows) DeletePastExceptions(ctx context.Context, d calendar.Date) (int64, error) {
	rows, err := r.rows.Records.DeletePickupExceptionsBefore(ctx, ports.CareDate(d))
	return rows, r.rows.ScheduleError("delete past exceptions", err)
}
