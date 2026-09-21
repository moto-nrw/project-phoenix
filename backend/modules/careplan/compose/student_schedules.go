package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (e *exceptionQueries) FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindArrivalSchedule(ctx, id)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_arrival_schedule", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListArrivalSchedules(ctx, f)

	err = mapError(err)
	e.observeRequest("list_arrival_schedules", started, stats, err)
	return value, err
}
func (e engine) CreateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalSchedule(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalSchedule(tx, v) }))
}
func (e engine) UpsertArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.UpsertArrivalSchedule(tx, v); return err })
	return result, mapError(err)
}
func (e engine) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalSchedule(tx, id) }))
}
func (e engine) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalSchedulesByStudent(tx, id) }))
}

func (e *exceptionQueries) FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindArrivalException(ctx, id, lock)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_arrival_exception", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListArrivalExceptions(ctx, f)

	err = mapError(err)
	e.observeRequest("list_arrival_exceptions", started, stats, err)
	return value, err
}
func (e engine) CreateArrivalException(ctx context.Context, v careplan.ArrivalException) (result careplan.ArrivalException, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalException(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdateArrivalException(ctx context.Context, v careplan.ArrivalException) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalException(tx, v) }))
}
func (e engine) DeleteArrivalException(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalException(tx, id) }))
}
func (e engine) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalExceptionsByStudent(tx, id) }))
}
func (e engine) DeleteArrivalExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeleteArrivalExceptionsBefore(tx, d); return err })
	return rows, mapError(err)
}

func (e *exceptionQueries) FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindArrivalNote(ctx, id)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_arrival_note", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListArrivalNotes(ctx, f)

	err = mapError(err)
	e.observeRequest("list_arrival_notes", started, stats, err)
	return value, err
}
func (e engine) CreateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (result careplan.ArrivalNote, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalNote(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdateArrivalNote(ctx context.Context, v careplan.ArrivalNote) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalNote(tx, v) }))
}
func (e engine) DeleteArrivalNote(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalNote(tx, id) }))
}
func (e engine) DeleteArrivalNotesByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalNotesByStudent(tx, id) }))
}
func (e engine) DeleteArrivalNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeleteArrivalNotesBefore(tx, d); return err })
	return rows, mapError(err)
}

func (e *exceptionQueries) FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindPickupSchedule(ctx, id)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_pickup_schedule", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListPickupSchedules(ctx, f)

	err = mapError(err)
	e.observeRequest("list_pickup_schedules", started, stats, err)
	return value, err
}
func (e engine) CreatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupSchedule(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupSchedule(tx, v) }))
}
func (e engine) UpsertPickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.UpsertPickupSchedule(tx, v); return err })
	return result, mapError(err)
}
func (e engine) DeletePickupSchedule(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupSchedule(tx, id) }))
}
func (e engine) DeletePickupSchedulesByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupSchedulesByStudent(tx, id) }))
}

func (e *exceptionQueries) FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindPickupException(ctx, id, lock)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_pickup_exception", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListPickupExceptions(ctx, f)

	err = mapError(err)
	e.observeRequest("list_pickup_exceptions", started, stats, err)
	return value, err
}
func (e engine) CreatePickupException(ctx context.Context, v careplan.PickupException) (result careplan.PickupException, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupException(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdatePickupException(ctx context.Context, v careplan.PickupException) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupException(tx, v) }))
}
func (e engine) DeletePickupException(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupException(tx, id) }))
}
func (e engine) DeletePickupExceptionsByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupExceptionsByStudent(tx, id) }))
}
func (e engine) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeletePickupExceptionsBefore(tx, d); return err })
	return rows, mapError(err)
}

func (e *exceptionQueries) FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, error) {
	started := time.Now()
	value, found, stats, err := e.statusQueries.FindPickupNote(ctx, id)
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = mapError(err)
	e.observeRequest("find_pickup_note", started, stats, err)
	return value, err
}
func (e *exceptionQueries) ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupNote, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.ListPickupNotes(ctx, f)

	err = mapError(err)
	e.observeRequest("list_pickup_notes", started, stats, err)
	return value, err
}
func (e engine) CreatePickupNote(ctx context.Context, v careplan.PickupNote) (result careplan.PickupNote, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupNote(tx, v); return err })
	return result, mapError(err)
}
func (e engine) UpdatePickupNote(ctx context.Context, v careplan.PickupNote) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupNote(tx, v) }))
}
func (e engine) DeletePickupNote(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupNote(tx, id) }))
}
func (e engine) ReplaceWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, notes map[int]string) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error {
		// Existing note rows do not provide a lock on a child's first note.
		// Lock the stable owner first so concurrent replacements cannot both
		// attempt the same weekday insert.
		if err := careplanning.LockStudent(tx, e.database, studentID); err != nil {
			return err
		}
		return e.service.ReplaceWeekdayPickupNotes(tx, studentID, createdBy, notes)
	}))
}
func (e engine) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupNotesByStudent(tx, id) }))
}
func (e engine) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeletePickupNotesBefore(tx, d); return err })
	return rows, mapError(err)
}

func (e *exceptionQueries) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	started := time.Now()
	value, stats, err := e.statusQueries.CountStudentScheduleRows(ctx, studentID)

	err = mapError(err)
	e.observeRequest("count_student_schedule_rows", started, stats, err)
	return value, err
}

func (e engine) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error {
		rows, err = e.service.EndStudentSchedulesForCareExit(tx, studentIDs, validUntil)
		return err
	})
	return rows, mapError(err)
}

func (e engine) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error {
		rows, err = e.service.RestoreStudentSchedulesForCareExit(tx, studentIDs)
		return err
	})
	return rows, mapError(err)
}
