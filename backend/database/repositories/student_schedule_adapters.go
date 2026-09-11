package repositories

import (
	"context"
	"errors"
	"fmt"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
)

// StudentScheduleRepositories contains the Care Plan compatibility adapters
// still consumed by legacy schedule services.
type StudentScheduleRepositories struct {
	ArrivalSchedule  scheduleModels.StudentArrivalScheduleRepository
	ArrivalException scheduleModels.StudentArrivalExceptionRepository
	ArrivalNote      scheduleModels.StudentArrivalNoteRepository
	PickupSchedule   scheduleModels.StudentPickupScheduleRepository
	PickupException  scheduleModels.StudentPickupExceptionRepository
	PickupNote       scheduleModels.StudentPickupNoteRepository
	StatusDay        activeModels.StudentStatusDayRepository
}

type arrivalScheduleRepository struct{ careplan.Capability }
type arrivalExceptionRepository struct{ careplan.Capability }
type arrivalNoteRepository struct{ careplan.Capability }
type pickupScheduleRepository struct{ careplan.Capability }
type pickupExceptionRepository struct{ careplan.Capability }
type pickupNoteRepository struct{ careplan.Capability }

func NewArrivalScheduleRepository(capability careplan.Capability) scheduleModels.StudentArrivalScheduleRepository {
	return arrivalScheduleRepository{capability}
}
func NewArrivalExceptionRepository(capability careplan.Capability) scheduleModels.StudentArrivalExceptionRepository {
	return arrivalExceptionRepository{capability}
}
func NewArrivalNoteRepository(capability careplan.Capability) scheduleModels.StudentArrivalNoteRepository {
	return arrivalNoteRepository{capability}
}
func NewPickupScheduleRepository(capability careplan.Capability) scheduleModels.StudentPickupScheduleRepository {
	return pickupScheduleRepository{capability}
}
func NewPickupExceptionRepository(capability careplan.Capability) scheduleModels.StudentPickupExceptionRepository {
	return pickupExceptionRepository{capability}
}
func NewPickupNoteRepository(capability careplan.Capability) scheduleModels.StudentPickupNoteRepository {
	return pickupNoteRepository{capability}
}

func invalidLegacyEntity(name string) error {
	return fmt.Errorf("%s cannot be nil or zero value", name)
}
func legacyScheduleID(raw any) (int64, error) { return carePlanLegacy.ScheduleID(raw) }
func legacyScheduleQueryOptions(options *carePlanLegacy.ScheduleQueryOptions) *careplan.StudentScheduleQueryOptions {
	return carePlanLegacy.CarePlanScheduleQueryOptions(options)
}
func legacyScheduleError(op string, err error) error {
	return carePlanLegacy.ScheduleError(op, err)
}
func today() careplan.Date                         { return carePlanLegacy.TodayScheduleDate() }
func date(value scheduleModels.Date) careplan.Date { return careplan.Date(value) }

func (r arrivalScheduleRepository) Create(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	created, err := r.CreateArrivalSchedule(ctx, arrivalScheduleToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyArrivalSchedule(row, created)
	return nil
}
func (r arrivalScheduleRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentArrivalSchedule, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	value, err := r.FindArrivalSchedule(ctx, id)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	return arrivalScheduleToLegacy(value), nil
}
func (r arrivalScheduleRepository) Update(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdateArrivalSchedule(ctx, arrivalScheduleToPublic(row)))
}
func (r arrivalScheduleRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeleteArrivalSchedule(ctx, id))
}
func (r arrivalScheduleRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)})
	if err != nil {
		return nil, legacyScheduleError("list with options", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r arrivalScheduleRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	if err != nil {
		return nil, legacyScheduleError("find by student id", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r arrivalScheduleRepository) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Weekday: weekday})
	if err != nil {
		return nil, legacyScheduleError("find by student id and weekday", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return arrivalScheduleToLegacy(values[0]), nil
}
func (r arrivalScheduleRepository) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
	if err != nil {
		return nil, legacyScheduleError("find by student ids and weekday", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r arrivalScheduleRepository) FindByStudentIDs(ctx context.Context, ids []int64) ([]*scheduleModels.StudentArrivalSchedule, error) {
	values, err := r.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, legacyScheduleError("find by student ids", err)
	}
	return mapRows(values, arrivalScheduleToLegacy), nil
}
func (r arrivalScheduleRepository) UpsertSchedule(ctx context.Context, row *scheduleModels.StudentArrivalSchedule) error {
	if row == nil {
		return errors.New("arrival schedule cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.UpsertArrivalSchedule(ctx, arrivalScheduleToPublic(row))
	if err != nil {
		return legacyScheduleError("upsert arrival schedule", err)
	}
	applyArrivalSchedule(row, value)
	return nil
}
func (r arrivalScheduleRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeleteArrivalSchedulesByStudent(ctx, id))
}

func (r arrivalExceptionRepository) Create(ctx context.Context, row *scheduleModels.StudentArrivalException) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.CreateArrivalException(ctx, arrivalExceptionToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyArrivalException(row, value)
	return nil
}
func (r arrivalExceptionRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentArrivalException, error) {
	return r.find(ctx, raw, false, "find by id")
}
func (r arrivalExceptionRepository) FindByIDForUpdate(ctx context.Context, raw any) (*scheduleModels.StudentArrivalException, error) {
	return r.find(ctx, raw, true, "find by id for update")
}
func (r arrivalExceptionRepository) find(ctx context.Context, raw any, lock bool, op string) (*scheduleModels.StudentArrivalException, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	value, err := r.FindArrivalException(ctx, id, lock)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return arrivalExceptionToLegacy(value), nil
}
func (r arrivalExceptionRepository) Update(ctx context.Context, row *scheduleModels.StudentArrivalException) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdateArrivalException(ctx, arrivalExceptionToPublic(row)))
}
func (r arrivalExceptionRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeleteArrivalException(ctx, id))
}
func (r arrivalExceptionRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentArrivalException, error) {
	values, err := r.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)})
	if err != nil {
		return nil, legacyScheduleError("list with options", err)
	}
	return mapRows(values, arrivalExceptionToLegacy), nil
}
func (r arrivalExceptionRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r arrivalExceptionRepository) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: today()}, "find upcoming by student id")
}
func (r arrivalExceptionRepository) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) (*scheduleModels.StudentArrivalException, error) {
	values, err := r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date(d)}, "find by student id and date")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r arrivalExceptionRepository) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: date(d)}, "find by student ids and date")
}
func (r arrivalExceptionRepository) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, From: date(from), To: date(to)}, "find by student id and date range")
}
func (r arrivalExceptionRepository) FindByStudentIDsAndDateRange(ctx context.Context, ids []int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentArrivalException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, From: date(from), To: date(to)}, "find by student ids and date range")
}
func (r arrivalExceptionRepository) list(ctx context.Context, filter careplan.StudentScheduleFilter, op string) ([]*scheduleModels.StudentArrivalException, error) {
	values, err := r.ListArrivalExceptions(ctx, filter)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return mapRows(values, arrivalExceptionToLegacy), nil
}
func (r arrivalExceptionRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeleteArrivalExceptionsByStudent(ctx, id))
}
func (r arrivalExceptionRepository) DeletePastExceptions(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.DeleteArrivalExceptionsBefore(ctx, date(d))
	return rows, legacyScheduleError("delete past exceptions", err)
}

func (r arrivalNoteRepository) Create(ctx context.Context, row *scheduleModels.StudentArrivalNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.CreateArrivalNote(ctx, arrivalNoteToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyArrivalNote(row, value)
	return nil
}
func (r arrivalNoteRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentArrivalNote, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	value, err := r.FindArrivalNote(ctx, id)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	return arrivalNoteToLegacy(value), nil
}
func (r arrivalNoteRepository) Update(ctx context.Context, row *scheduleModels.StudentArrivalNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdateArrivalNote(ctx, arrivalNoteToPublic(row)))
}
func (r arrivalNoteRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeleteArrivalNote(ctx, id))
}
func (r arrivalNoteRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)}, "list with options")
}
func (r arrivalNoteRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r arrivalNoteRepository) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date(d)}, "find by student id and date")
}
func (r arrivalNoteRepository) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: date(d)}, "find by student ids and date")
}
func (r arrivalNoteRepository) list(ctx context.Context, filter careplan.StudentScheduleFilter, op string) ([]*scheduleModels.StudentArrivalNote, error) {
	values, err := r.ListArrivalNotes(ctx, filter)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return mapRows(values, arrivalNoteToLegacy), nil
}
func (r arrivalNoteRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeleteArrivalNotesByStudent(ctx, id))
}
func (r arrivalNoteRepository) DeletePastNotes(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.DeleteArrivalNotesBefore(ctx, date(d))
	return rows, legacyScheduleError("delete past notes", err)
}

func (r pickupScheduleRepository) Create(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.CreatePickupSchedule(ctx, pickupScheduleToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r pickupScheduleRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupSchedule, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	value, err := r.FindPickupSchedule(ctx, id)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	return pickupScheduleToLegacy(value), nil
}
func (r pickupScheduleRepository) Update(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupSchedule")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdatePickupSchedule(ctx, pickupScheduleToPublic(row)))
}
func (r pickupScheduleRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeletePickupSchedule(ctx, id))
}
func (r pickupScheduleRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)})
	if err != nil {
		return nil, legacyScheduleError("list with options", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r pickupScheduleRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	if err != nil {
		return nil, legacyScheduleError("find by student id", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r pickupScheduleRepository) FindByStudentIDAndWeekday(ctx context.Context, id int64, weekday int) (*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Weekday: weekday})
	if err != nil {
		return nil, legacyScheduleError("find by student id and weekday", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return pickupScheduleToLegacy(values[0]), nil
}
func (r pickupScheduleRepository) FindByStudentIDsAndWeekday(ctx context.Context, ids []int64, weekday int) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Weekday: weekday})
	if err != nil {
		return nil, legacyScheduleError("find by student ids and weekday", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r pickupScheduleRepository) FindByStudentIDs(ctx context.Context, ids []int64) ([]*scheduleModels.StudentPickupSchedule, error) {
	values, err := r.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, legacyScheduleError("find by student ids", err)
	}
	return mapRows(values, pickupScheduleToLegacy), nil
}
func (r pickupScheduleRepository) UpsertSchedule(ctx context.Context, row *scheduleModels.StudentPickupSchedule) error {
	if row == nil {
		return errors.New("schedule cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.UpsertPickupSchedule(ctx, pickupScheduleToPublic(row))
	if err != nil {
		return legacyScheduleError("upsert schedule", err)
	}
	applyPickupSchedule(row, value)
	return nil
}
func (r pickupScheduleRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeletePickupSchedulesByStudent(ctx, id))
}

func (r pickupExceptionRepository) Create(ctx context.Context, row *scheduleModels.StudentPickupException) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupException")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.CreatePickupException(ctx, pickupExceptionToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyPickupException(row, value)
	return nil
}
func (r pickupExceptionRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupException, error) {
	return r.find(ctx, raw, false, "find by id")
}
func (r pickupExceptionRepository) FindByIDForUpdate(ctx context.Context, raw any) (*scheduleModels.StudentPickupException, error) {
	return r.find(ctx, raw, true, "find by id for update")
}
func (r pickupExceptionRepository) find(ctx context.Context, raw any, lock bool, op string) (*scheduleModels.StudentPickupException, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	value, err := r.FindPickupException(ctx, id, lock)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return pickupExceptionToLegacy(value), nil
}
func (r pickupExceptionRepository) Update(ctx context.Context, row *scheduleModels.StudentPickupException) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupException")
	}
	row.NormalizeWallClockTimes()
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdatePickupException(ctx, pickupExceptionToPublic(row)))
}
func (r pickupExceptionRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeletePickupException(ctx, id))
}
func (r pickupExceptionRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)})
	if err != nil {
		return nil, legacyScheduleError("list with options", err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r pickupExceptionRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r pickupExceptionRepository) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: today()}, "find upcoming by student id")
}
func (r pickupExceptionRepository) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) (*scheduleModels.StudentPickupException, error) {
	values, err := r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date(d)}, "find by student id and date")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r pickupExceptionRepository) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: date(d)}, "find by student ids and date")
}
func (r pickupExceptionRepository) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, From: date(from), To: date(to)}, "find by student id and date range")
}
func (r pickupExceptionRepository) FindByStudentIDsAndDateRange(ctx context.Context, ids []int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, From: date(from), To: date(to)}, "find by student ids and date range")
}
func (r pickupExceptionRepository) list(ctx context.Context, filter careplan.StudentScheduleFilter, op string) ([]*scheduleModels.StudentPickupException, error) {
	values, err := r.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return mapRows(values, pickupExceptionToLegacy), nil
}
func (r pickupExceptionRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeletePickupExceptionsByStudent(ctx, id))
}
func (r pickupExceptionRepository) DeletePastExceptions(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.DeletePickupExceptionsBefore(ctx, date(d))
	return rows, legacyScheduleError("delete past exceptions", err)
}

func (r pickupNoteRepository) Create(ctx context.Context, row *scheduleModels.StudentPickupNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.CreatePickupNote(ctx, pickupNoteToPublic(row))
	if err != nil {
		return legacyScheduleError("create", err)
	}
	applyPickupNote(row, value)
	return nil
}
func (r pickupNoteRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentPickupNote, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	value, err := r.FindPickupNote(ctx, id)
	if err != nil {
		return nil, legacyScheduleError("find by id", err)
	}
	return pickupNoteToLegacy(value), nil
}
func (r pickupNoteRepository) Update(ctx context.Context, row *scheduleModels.StudentPickupNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentPickupNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return legacyScheduleError("update", r.UpdatePickupNote(ctx, pickupNoteToPublic(row)))
}
func (r pickupNoteRepository) Delete(ctx context.Context, raw any) error {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return legacyScheduleError("delete", err)
	}
	return legacyScheduleError("delete", r.DeletePickupNote(ctx, id))
}
func (r pickupNoteRepository) List(ctx context.Context, options *carePlanLegacy.ScheduleQueryOptions) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{Options: carePlanLegacy.CarePlanScheduleQueryOptions(options)}, "list with options")
}
func (r pickupNoteRepository) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}
func (r pickupNoteRepository) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date(d)}, "find by student id and date")
}
func (r pickupNoteRepository) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, Date: date(d)}, "find by student ids and date")
}
func (r pickupNoteRepository) list(ctx context.Context, filter careplan.StudentScheduleFilter, op string) ([]*scheduleModels.StudentPickupNote, error) {
	values, err := r.ListPickupNotes(ctx, filter)
	if err != nil {
		return nil, legacyScheduleError(op, err)
	}
	return mapRows(values, pickupNoteToLegacy), nil
}
func (r pickupNoteRepository) DeleteByStudentID(ctx context.Context, id int64) error {
	return legacyScheduleError("delete by student id", r.DeletePickupNotesByStudent(ctx, id))
}
func (r pickupNoteRepository) DeletePastNotes(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.DeletePickupNotesBefore(ctx, date(d))
	return rows, legacyScheduleError("delete past notes", err)
}

func mapRows[T any, R any](values []T, convert func(T) R) []R {
	result := make([]R, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func arrivalScheduleToPublic(v *scheduleModels.StudentArrivalSchedule) careplan.ArrivalSchedule {
	return careplan.ArrivalSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, ExpectedArrival: v.ExpectedArrival, Notes: v.Notes, CreatedBy: v.CreatedBy}
}
func arrivalScheduleToLegacy(v careplan.ArrivalSchedule) *scheduleModels.StudentArrivalSchedule {
	row := new(scheduleModels.StudentArrivalSchedule)
	applyArrivalSchedule(row, v)
	return row
}
func applyArrivalSchedule(r *scheduleModels.StudentArrivalSchedule, v careplan.ArrivalSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.ExpectedArrival, r.Notes, r.CreatedBy = v.StudentID, v.Weekday, v.ExpectedArrival, v.Notes, v.CreatedBy
}
func arrivalExceptionToPublic(v *scheduleModels.StudentArrivalException) careplan.ArrivalException {
	return careplan.ArrivalException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: date(v.ExceptionDate), ExpectedArrival: v.ExpectedArrival, Reason: v.Reason, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func arrivalExceptionToLegacy(v careplan.ArrivalException) *scheduleModels.StudentArrivalException {
	row := new(scheduleModels.StudentArrivalException)
	applyArrivalException(row, v)
	return row
}
func applyArrivalException(r *scheduleModels.StudentArrivalException, v careplan.ArrivalException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.ExpectedArrival, r.Reason, r.Source, r.CreatedBy, r.CreatedByGuardian = v.StudentID, scheduleModels.Date(v.ExceptionDate), v.ExpectedArrival, v.Reason, v.Source, v.CreatedBy, v.CreatedByGuardian
}
func arrivalNoteToPublic(v *scheduleModels.StudentArrivalNote) careplan.ArrivalNote {
	return careplan.ArrivalNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: date(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
func arrivalNoteToLegacy(v careplan.ArrivalNote) *scheduleModels.StudentArrivalNote {
	row := new(scheduleModels.StudentArrivalNote)
	applyArrivalNote(row, v)
	return row
}
func applyArrivalNote(r *scheduleModels.StudentArrivalNote, v careplan.ArrivalNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Content, r.CreatedBy = v.StudentID, scheduleModels.Date(v.NoteDate), v.Content, v.CreatedBy
}
func pickupScheduleToPublic(v *scheduleModels.StudentPickupSchedule) careplan.PickupSchedule {
	return careplan.PickupSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, PickupTime: v.PickupTime, Notes: v.Notes, CreatedBy: v.CreatedBy, Source: v.Source, CareOfferingID: v.CareOfferingID}
}
func pickupScheduleToLegacy(v careplan.PickupSchedule) *scheduleModels.StudentPickupSchedule {
	row := new(scheduleModels.StudentPickupSchedule)
	applyPickupSchedule(row, v)
	return row
}
func applyPickupSchedule(r *scheduleModels.StudentPickupSchedule, v careplan.PickupSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.PickupTime, r.Notes, r.CreatedBy, r.Source, r.CareOfferingID = v.StudentID, v.Weekday, v.PickupTime, v.Notes, v.CreatedBy, v.Source, v.CareOfferingID
}
func pickupExceptionToPublic(v *scheduleModels.StudentPickupException) careplan.PickupException {
	return careplan.PickupException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: date(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupExceptionToLegacy(v careplan.PickupException) *scheduleModels.StudentPickupException {
	row := new(scheduleModels.StudentPickupException)
	applyPickupException(row, v)
	return row
}
func applyPickupException(r *scheduleModels.StudentPickupException, v careplan.PickupException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.PickupTime, r.Reason = v.StudentID, scheduleModels.Date(v.ExceptionDate), v.PickupTime, v.Reason
	r.ExcusedFrom, r.ExcusedReason, r.ExcusedCreatedBy, r.ExcusedOwnsPickupTime, r.ExcusedAuto = v.ExcusedFrom, v.ExcusedReason, v.ExcusedCreatedBy, v.ExcusedOwnsPickupTime, v.ExcusedAuto
	r.Source, r.CreatedBy, r.CreatedByGuardian = v.Source, v.CreatedBy, v.CreatedByGuardian
}
func pickupNoteToPublic(v *scheduleModels.StudentPickupNote) careplan.PickupNote {
	return careplan.PickupNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: date(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
func pickupNoteToLegacy(v careplan.PickupNote) *scheduleModels.StudentPickupNote {
	row := new(scheduleModels.StudentPickupNote)
	applyPickupNote(row, v)
	return row
}
func applyPickupNote(r *scheduleModels.StudentPickupNote, v careplan.PickupNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Content, r.CreatedBy = v.StudentID, scheduleModels.Date(v.NoteDate), v.Content, v.CreatedBy
}

var (
	_ scheduleModels.StudentArrivalScheduleRepository  = arrivalScheduleRepository{}
	_ scheduleModels.StudentArrivalExceptionRepository = arrivalExceptionRepository{}
	_ scheduleModels.StudentArrivalNoteRepository      = arrivalNoteRepository{}
	_ scheduleModels.StudentPickupScheduleRepository   = pickupScheduleRepository{}
	_ scheduleModels.StudentPickupExceptionRepository  = pickupExceptionRepository{}
	_ scheduleModels.StudentPickupNoteRepository       = pickupNoteRepository{}
)
