package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (e engine) FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, error) {
	value, err := e.service.FindArrivalSchedule(ctx, id)
	return value, mapError(err)
}
func (e engine) ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	values, err := e.service.ListArrivalSchedules(ctx, f)
	return values, mapError(err)
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

func (e engine) FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, error) {
	value, err := e.service.FindArrivalException(ctx, id, lock)
	return value, mapError(err)
}
func (e engine) ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error) {
	values, err := e.service.ListArrivalExceptions(ctx, f)
	return values, mapError(err)
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

func (e engine) FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, error) {
	value, err := e.service.FindArrivalNote(ctx, id)
	return value, mapError(err)
}
func (e engine) ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error) {
	values, err := e.service.ListArrivalNotes(ctx, f)
	return values, mapError(err)
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

func (e engine) FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, error) {
	value, err := e.service.FindPickupSchedule(ctx, id)
	return value, mapError(err)
}
func (e engine) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	values, err := e.service.ListPickupSchedules(ctx, f)
	return values, mapError(err)
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

func (e engine) FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, error) {
	value, err := e.service.FindPickupException(ctx, id, lock)
	return value, mapError(err)
}
func (e engine) ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	values, err := e.service.ListPickupExceptions(ctx, f)
	return values, mapError(err)
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

func (e engine) FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, error) {
	value, err := e.service.FindPickupNote(ctx, id)
	return value, mapError(err)
}
func (e engine) ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupNote, error) {
	values, err := e.service.ListPickupNotes(ctx, f)
	return values, mapError(err)
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
func (e engine) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(tx context.Context) error { return e.service.DeletePickupNotesByStudent(tx, id) }))
}
func (e engine) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = e.withinTenant(ctx, func(tx context.Context) error { rows, err = e.service.DeletePickupNotesBefore(tx, d); return err })
	return rows, mapError(err)
}

func (e engine) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	value, err := e.service.CountStudentScheduleRows(ctx, studentID)
	return value, mapError(err)
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
