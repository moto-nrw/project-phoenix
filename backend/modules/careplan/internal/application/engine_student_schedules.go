package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (e *ExceptionQueryService) FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalSchedule, bool, domain.OperationStats, error) {
		return e.statusQueries.FindArrivalSchedule(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_arrival_schedule", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalSchedule, domain.OperationStats, error) {
		return e.statusQueries.ListArrivalSchedules(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_arrival_schedules", started, stats, err)
	return value, err
}
func (e Engine) CreateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalSchedule(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalSchedule(tx, v) }))
}
func (e Engine) UpsertArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.UpsertArrivalSchedule(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalSchedule(tx, id) }))
}
func (e Engine) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalSchedulesByStudent(tx, id) }))
}

func (e *ExceptionQueryService) FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalException, bool, domain.OperationStats, error) {
		return e.statusQueries.FindArrivalException(txCtx, id, lock)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_arrival_exception", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalException, domain.OperationStats, error) {
		return e.statusQueries.ListArrivalExceptions(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_arrival_exceptions", started, stats, err)
	return value, err
}
func (e Engine) CreateArrivalException(ctx context.Context, v careplan.ArrivalException) (result careplan.ArrivalException, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalException(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdateArrivalException(ctx context.Context, v careplan.ArrivalException) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalException(tx, v) }))
}
func (e Engine) DeleteArrivalException(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalException(tx, id) }))
}
func (e Engine) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalExceptionsByStudent(tx, id) }))
}
func (e Engine) DeleteArrivalExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeleteArrivalExceptionsBefore(tx, d); return err })
	return rows, MapError(err)
}

func (e *ExceptionQueryService) FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalNote, bool, domain.OperationStats, error) {
		return e.statusQueries.FindArrivalNote(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_arrival_note", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalNote, domain.OperationStats, error) {
		return e.statusQueries.ListArrivalNotes(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_arrival_notes", started, stats, err)
	return value, err
}
func (e Engine) CreateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (result careplan.ArrivalNote, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreateArrivalNote(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdateArrivalNote(ctx context.Context, v careplan.ArrivalNote) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdateArrivalNote(tx, v) }))
}
func (e Engine) DeleteArrivalNote(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalNote(tx, id) }))
}
func (e Engine) DeleteArrivalNotesByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeleteArrivalNotesByStudent(tx, id) }))
}
func (e Engine) DeleteArrivalNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeleteArrivalNotesBefore(tx, d); return err })
	return rows, MapError(err)
}

func (e *ExceptionQueryService) FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupSchedule, bool, domain.OperationStats, error) {
		return e.statusQueries.FindPickupSchedule(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_pickup_schedule", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupSchedule, domain.OperationStats, error) {
		return e.statusQueries.ListPickupSchedules(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_pickup_schedules", started, stats, err)
	return value, err
}
func (e Engine) CreatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupSchedule(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupSchedule(tx, v) }))
}
func (e Engine) UpsertPickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.UpsertPickupSchedule(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) DeletePickupSchedule(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupSchedule(tx, id) }))
}
func (e Engine) DeletePickupSchedulesByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupSchedulesByStudent(tx, id) }))
}

func (e *ExceptionQueryService) FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupException, bool, domain.OperationStats, error) {
		return e.statusQueries.FindPickupException(txCtx, id, lock)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_pickup_exception", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupException, domain.OperationStats, error) {
		return e.statusQueries.ListPickupExceptions(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_pickup_exceptions", started, stats, err)
	return value, err
}
func (e Engine) CreatePickupException(ctx context.Context, v careplan.PickupException) (result careplan.PickupException, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupException(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdatePickupException(ctx context.Context, v careplan.PickupException) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupException(tx, v) }))
}
func (e Engine) DeletePickupException(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupException(tx, id) }))
}
func (e Engine) DeletePickupExceptionsByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupExceptionsByStudent(tx, id) }))
}
func (e Engine) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeletePickupExceptionsBefore(tx, d); return err })
	return rows, MapError(err)
}

func (e *ExceptionQueryService) FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, error) {
	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupNote, bool, domain.OperationStats, error) {
		return e.statusQueries.FindPickupNote(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	err = MapError(err)
	e.observeRequest("find_pickup_note", started, stats, err)
	return value, err
}
func (e *ExceptionQueryService) ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupNote, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupNote, domain.OperationStats, error) {
		return e.statusQueries.ListPickupNotes(txCtx, f)
	})

	err = MapError(err)
	e.observeRequest("list_pickup_notes", started, stats, err)
	return value, err
}
func (e Engine) CreatePickupNote(ctx context.Context, v careplan.PickupNote) (result careplan.PickupNote, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { result, err = e.service.CreatePickupNote(tx, v); return err })
	return result, MapError(err)
}
func (e Engine) UpdatePickupNote(ctx context.Context, v careplan.PickupNote) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.UpdatePickupNote(tx, v) }))
}
func (e Engine) DeletePickupNote(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupNote(tx, id) }))
}
func (e Engine) ReplaceWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, notes map[int]string) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error {
		// Existing note rows do not provide a lock on a child's first note.
		// Lock the stable owner first so concurrent replacements cannot both
		// attempt the same weekday insert.
		if err := e.runtime.LockStudent(tx, studentID); err != nil {
			return err
		}
		return e.service.ReplaceWeekdayPickupNotes(tx, studentID, createdBy, notes)
	}))
}
func (e Engine) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupNotesByStudent(tx, id) }))
}
func (e Engine) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeletePickupNotesBefore(tx, d); return err })
	return rows, MapError(err)
}

func (e *ExceptionQueryService) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) (int, domain.OperationStats, error) {
		return e.statusQueries.CountStudentScheduleRows(txCtx, studentID)
	})

	err = MapError(err)
	e.observeRequest("count_student_schedule_rows", started, stats, err)
	return value, err
}

func (e Engine) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error {
		rows, err = e.service.EndStudentSchedulesForCareExit(tx, studentIDs, validUntil)
		return err
	})
	return rows, MapError(err)
}

func (e Engine) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error {
		rows, err = e.service.RestoreStudentSchedulesForCareExit(tx, studentIDs)
		return err
	})
	return rows, MapError(err)
}
