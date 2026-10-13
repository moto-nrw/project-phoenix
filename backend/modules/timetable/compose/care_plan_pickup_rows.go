package compose

import (
	"context"
	"errors"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
		return r.rows.scheduleError("create", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r CarePickupScheduleRows) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupSchedule, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupSchedule(ctx, id)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
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
	return r.rows.scheduleError("update", r.rows.Records.UpdatePickupSchedule(ctx, pickupScheduleToPublic(row)))
}
func (r CarePickupScheduleRows) List(ctx context.Context, options *userModels.QueryOptions) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, CareScheduleFilter{Options: options})
	if err != nil {
		return nil, r.rows.scheduleError("list with options", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r CarePickupScheduleRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, CareScheduleFilter{StudentIDs: []int64{id}})
	if err != nil {
		return nil, r.rows.scheduleError("find by student id", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r CarePickupScheduleRows) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, CareScheduleFilter{StudentIDs: []int64{id}, Weekday: weekday})
	if err != nil {
		return nil, r.rows.scheduleError("find by student id and weekday", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return pickupScheduleToLegacy(values[0]), nil
}
func (r CarePickupScheduleRows) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.rows.Records.ListPickupSchedules(ctx, CareScheduleFilter{StudentIDs: ids, Weekday: weekday})
	if err != nil {
		return nil, r.rows.scheduleError("find by student ids and weekday", err)
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
		return r.rows.scheduleError("upsert schedule", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r CarePickupScheduleRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.scheduleError("delete by student id", r.rows.Records.DeletePickupSchedulesByStudent(ctx, id))
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
		return r.rows.scheduleError("create", err)
	}
	applyPickupException(row, value)
	return nil
}
func (r CarePickupExceptionRows) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupException, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupException(ctx, id)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
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
	return r.rows.scheduleError("update", r.rows.Records.UpdatePickupException(ctx, pickupExceptionToPublic(row)))
}
func (r CarePickupExceptionRows) List(ctx context.Context, options *userModels.QueryOptions) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.rows.Records.ListPickupExceptions(ctx, CareScheduleFilter{Options: options})
	if err != nil {
		return nil, r.rows.scheduleError("list with options", err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r CarePickupExceptionRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r CarePickupExceptionRows) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: r.rows.Today()}, "find upcoming by student id")
}
func (r CarePickupExceptionRows) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) (*scheduleModels.StudentPickupException, error) {
	values, err := r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}, Date: CareDate(d)}, "find by student id and date")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r CarePickupExceptionRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: ids, Date: CareDate(d)}, "find by student ids and date")
}
func (r CarePickupExceptionRows) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}, From: CareDate(from), To: CareDate(to)}, "find by student id and date range")
}
func (r CarePickupExceptionRows) list(ctx context.Context, filter CareScheduleFilter, op string) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.rows.Records.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, r.rows.scheduleError(op, err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r CarePickupExceptionRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.scheduleError("delete by student id", r.rows.Records.DeletePickupExceptionsByStudent(ctx, id))
}
func (r CarePickupExceptionRows) DeletePastExceptions(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.rows.Records.DeletePickupExceptionsBefore(ctx, CareDate(d))
	return rows, r.rows.scheduleError("delete past exceptions", err)
}

func (r CarePickupNoteRows) Create(ctx context.Context, row *scheduleModels.StudentPickupNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.CreatePickupNote(ctx, pickupNoteToPublic(row))
	if err != nil {
		return r.rows.scheduleError("create", err)
	}
	applyPickupNote(row, value)
	return nil
}
func (r CarePickupNoteRows) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupNote, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupNote(ctx, id)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
	}
	return pickupNoteToLegacy(value), nil
}
func (r CarePickupNoteRows) Update(ctx context.Context, row *scheduleModels.StudentPickupNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.scheduleError("update", r.rows.Records.UpdatePickupNote(ctx, pickupNoteToPublic(row)))
}
func (r CarePickupNoteRows) List(ctx context.Context, options *userModels.QueryOptions) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, CareScheduleFilter{Options: options}, "list with options")
}
func (r CarePickupNoteRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r CarePickupNoteRows) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}, Date: CareDate(d)}, "find by student id and date")
}
func (r CarePickupNoteRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: ids, Date: CareDate(d)}, "find by student ids and date")
}
func (r CarePickupNoteRows) list(ctx context.Context, filter CareScheduleFilter, op string) ([]*scheduleModels.StudentPickupNote, error) {
	values, err := r.rows.Records.ListPickupNotes(ctx, filter)
	if err != nil {
		return nil, r.rows.scheduleError(op, err)
	}
	return mapRows(values, pickupNoteToLegacy), nil
}
func (r CarePickupNoteRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.scheduleError("delete by student id", r.rows.Records.DeletePickupNotesByStudent(ctx, id))
}
func (r CarePickupNoteRows) DeletePastNotes(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.rows.Records.DeletePickupNotesBefore(ctx, CareDate(d))
	return rows, r.rows.scheduleError("delete past notes", err)
}
