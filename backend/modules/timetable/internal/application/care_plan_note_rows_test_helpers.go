//go:build testhooks

package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CareArrivalNoteRows struct{ rows ports.CarePlanRows }

type CarePickupNoteRows struct{ rows ports.CarePlanRows }

func NewCareArrivalNoteRows(rows ports.CarePlanRows) CareArrivalNoteRows {
	return CareArrivalNoteRows{rows: rows}
}

func NewCarePickupNoteRows(rows ports.CarePlanRows) CarePickupNoteRows {
	return CarePickupNoteRows{rows: rows}
}

func arrivalNoteToPublic(v *scheduleModels.StudentArrivalNote) ports.CareArrivalNote {
	return ports.CareArrivalNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: ports.CareDate(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}

func arrivalNoteToLegacy(v ports.CareArrivalNote) *scheduleModels.StudentArrivalNote {
	row := new(scheduleModels.StudentArrivalNote)
	applyArrivalNote(row, v)
	return row
}

func applyArrivalNote(r *scheduleModels.StudentArrivalNote, v ports.CareArrivalNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Content, r.CreatedBy = v.StudentID, calendar.Date(v.NoteDate), v.Content, v.CreatedBy
}

func pickupNoteToPublic(v *scheduleModels.StudentPickupNote) ports.CarePickupNote {
	return ports.CarePickupNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: ports.CareDate(v.NoteDate), Weekday: v.Weekday, Content: v.Content, CreatedBy: v.CreatedBy}
}

func pickupNoteToLegacy(v ports.CarePickupNote) *scheduleModels.StudentPickupNote {
	row := new(scheduleModels.StudentPickupNote)
	applyPickupNote(row, v)
	return row
}

func applyPickupNote(r *scheduleModels.StudentPickupNote, v ports.CarePickupNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Weekday, r.Content, r.CreatedBy = v.StudentID, calendar.Date(v.NoteDate), v.Weekday, v.Content, v.CreatedBy
}

func (r CareArrivalNoteRows) Create(ctx context.Context, row *scheduleModels.StudentArrivalNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	value, err := r.rows.Records.CreateArrivalNote(ctx, arrivalNoteToPublic(row))
	if err != nil {
		return r.rows.ScheduleError("create", err)
	}
	applyArrivalNote(row, value)
	return nil
}

func (r CareArrivalNoteRows) FindByID(ctx context.Context, nativeRecordID int64) (*scheduleModels.StudentArrivalNote, error) {
	var raw any = nativeRecordID
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindArrivalNote(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	return arrivalNoteToLegacy(value), nil
}

func (r CareArrivalNoteRows) Update(ctx context.Context, row *scheduleModels.StudentArrivalNote) error {
	if row == nil {
		return invalidLegacyEntity("StudentArrivalNote")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return r.rows.ScheduleError("update", r.rows.Records.UpdateArrivalNote(ctx, arrivalNoteToPublic(row)))
}

func (r CareArrivalNoteRows) List(ctx context.Context, options *domain.ListOptions) ([]*scheduleModels.StudentArrivalNote, error) {
	return listWithOptions(ctx, r.rows, options, r.rows.Records.ListArrivalNotes, arrivalNoteToLegacy)
}

func (r CareArrivalNoteRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}

func (r CareArrivalNoteRows) FindByStudentIDAndDate(ctx context.Context, id int64, d calendar.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Date: ports.CareDate(d)}, "find by student id and date")
}

func (r CareArrivalNoteRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d calendar.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: ids, Date: ports.CareDate(d)}, "find by student ids and date")
}

func (r CareArrivalNoteRows) list(ctx context.Context, filter ports.CareScheduleFilter, op string) ([]*scheduleModels.StudentArrivalNote, error) {
	values, err := r.rows.Records.ListArrivalNotes(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError(op, err)
	}
	return mapRows(values, arrivalNoteToLegacy), nil
}

func (r CareArrivalNoteRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeleteArrivalNotesByStudent(ctx, id))
}

func (r CareArrivalNoteRows) DeletePastNotes(ctx context.Context, d calendar.Date) (int64, error) {
	rows, err := r.rows.Records.DeleteArrivalNotesBefore(ctx, ports.CareDate(d))
	return rows, r.rows.ScheduleError("delete past notes", err)
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
		return r.rows.ScheduleError("create", err)
	}
	applyPickupNote(row, value)
	return nil
}

func (r CarePickupNoteRows) FindByID(ctx context.Context, nativeRecordID int64) (*scheduleModels.StudentPickupNote, error) {
	var raw any = nativeRecordID
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindPickupNote(ctx, id)
	if err != nil {
		return nil, r.rows.ScheduleError("find by id", err)
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
	return r.rows.ScheduleError("update", r.rows.Records.UpdatePickupNote(ctx, pickupNoteToPublic(row)))
}

func (r CarePickupNoteRows) List(ctx context.Context, options *domain.ListOptions) ([]*scheduleModels.StudentPickupNote, error) {
	return listWithOptions(ctx, r.rows, options, r.rows.Records.ListPickupNotes, pickupNoteToLegacy)
}

func (r CarePickupNoteRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}

func (r CarePickupNoteRows) FindByStudentIDAndDate(ctx context.Context, id int64, d calendar.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: []int64{id}, Date: ports.CareDate(d)}, "find by student id and date")
}

func (r CarePickupNoteRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d calendar.Date) ([]*scheduleModels.StudentPickupNote, error) {
	return r.list(ctx, ports.CareScheduleFilter{StudentIDs: ids, Date: ports.CareDate(d)}, "find by student ids and date")
}

func (r CarePickupNoteRows) list(ctx context.Context, filter ports.CareScheduleFilter, op string) ([]*scheduleModels.StudentPickupNote, error) {
	values, err := r.rows.Records.ListPickupNotes(ctx, filter)
	if err != nil {
		return nil, r.rows.ScheduleError(op, err)
	}
	return mapRows(values, pickupNoteToLegacy), nil
}

func (r CarePickupNoteRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.ScheduleError("delete by student id", r.rows.Records.DeletePickupNotesByStudent(ctx, id))
}

func (r CarePickupNoteRows) DeletePastNotes(ctx context.Context, d calendar.Date) (int64, error) {
	rows, err := r.rows.Records.DeletePickupNotesBefore(ctx, ports.CareDate(d))
	return rows, r.rows.ScheduleError("delete past notes", err)
}

var (
	_ ports.StudentArrivalNoteRecords = CareArrivalNoteRows{}
	_ ports.StudentPickupNoteRecords  = CarePickupNoteRows{}
)
