package compose

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type CareArrivalNoteRows struct{ rows CarePlanRows }

type CarePickupNoteRows struct{ rows CarePlanRows }

// CareScheduleRepositories are the retained schedule repositories served by
// the Care Plan owner.
type CareScheduleRepositories struct {
	StudentArrivalSchedule    scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException   scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote        scheduleModels.StudentArrivalNoteRepository
	StudentPickupSchedule     scheduleModels.StudentPickupScheduleRepository
	StudentPickupException    scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote         scheduleModels.StudentPickupNoteRepository
	CareScheduleChangeRequest scheduleModels.CareScheduleChangeRequestRepository
}

// NewCareScheduleRepositories builds the retained schedule repositories over
// the Care Plan records.
func NewCareScheduleRepositories(rows CarePlanRows) CareScheduleRepositories {
	return CareScheduleRepositories{
		StudentArrivalSchedule:    NewCareArrivalScheduleRows(rows),
		StudentArrivalException:   NewCareArrivalExceptionRows(rows),
		StudentArrivalNote:        NewCareArrivalNoteRows(rows),
		StudentPickupSchedule:     NewCarePickupScheduleRows(rows),
		StudentPickupException:    NewCarePickupExceptionRows(rows),
		StudentPickupNote:         NewCarePickupNoteRows(rows),
		CareScheduleChangeRequest: NewCareScheduleRequestRows(rows),
	}
}

// EnrollmentPickupSchedules binds Enrollment's weekly pickup port to the
// pickup schedules.
func (r CareScheduleRepositories) EnrollmentPickupSchedules() EnrollmentPickupSchedules {
	return NewEnrollmentPickupSchedules(r.StudentPickupSchedule)
}

// EnrollmentArrivalSchedules binds Enrollment's weekly arrival port to the
// arrival schedules.
func (r CareScheduleRepositories) EnrollmentArrivalSchedules() EnrollmentArrivalSchedules {
	return NewEnrollmentArrivalSchedules(r.StudentArrivalSchedule)
}

func NewCareArrivalNoteRows(rows CarePlanRows) CareArrivalNoteRows {
	return CareArrivalNoteRows{rows: rows}
}

func NewCarePickupNoteRows(rows CarePlanRows) CarePickupNoteRows {
	return CarePickupNoteRows{rows: rows}
}

func arrivalNoteToPublic(v *scheduleModels.StudentArrivalNote) CareArrivalNote {
	return CareArrivalNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: CareDate(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}

func arrivalNoteToLegacy(v CareArrivalNote) *scheduleModels.StudentArrivalNote {
	row := new(scheduleModels.StudentArrivalNote)
	applyArrivalNote(row, v)
	return row
}

func applyArrivalNote(r *scheduleModels.StudentArrivalNote, v CareArrivalNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Content, r.CreatedBy = v.StudentID, scheduleModels.Date(v.NoteDate), v.Content, v.CreatedBy
}

func pickupNoteToPublic(v *scheduleModels.StudentPickupNote) CarePickupNote {
	return CarePickupNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: CareDate(v.NoteDate), Weekday: v.Weekday, Content: v.Content, CreatedBy: v.CreatedBy}
}

func pickupNoteToLegacy(v CarePickupNote) *scheduleModels.StudentPickupNote {
	row := new(scheduleModels.StudentPickupNote)
	applyPickupNote(row, v)
	return row
}

func applyPickupNote(r *scheduleModels.StudentPickupNote, v CarePickupNote) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.NoteDate, r.Weekday, r.Content, r.CreatedBy = v.StudentID, scheduleModels.Date(v.NoteDate), v.Weekday, v.Content, v.CreatedBy
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
		return r.rows.scheduleError("create", err)
	}
	applyArrivalNote(row, value)
	return nil
}

func (r CareArrivalNoteRows) FindByID(ctx context.Context, raw any) (*scheduleModels.StudentArrivalNote, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
	}
	value, err := r.rows.Records.FindArrivalNote(ctx, id)
	if err != nil {
		return nil, r.rows.scheduleError("find by id", err)
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
	return r.rows.scheduleError("update", r.rows.Records.UpdateArrivalNote(ctx, arrivalNoteToPublic(row)))
}

func (r CareArrivalNoteRows) List(ctx context.Context, options *userModels.QueryOptions) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, CareScheduleFilter{Options: options}, "list with options")
}

func (r CareArrivalNoteRows) FindByStudentID(ctx context.Context, id int64) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}}, "find by student id")
}

func (r CareArrivalNoteRows) FindByStudentIDAndDate(ctx context.Context, id int64, d scheduleModels.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: []int64{id}, Date: CareDate(d)}, "find by student id and date")
}

func (r CareArrivalNoteRows) FindByStudentIDsAndDate(ctx context.Context, ids []int64, d scheduleModels.Date) ([]*scheduleModels.StudentArrivalNote, error) {
	return r.list(ctx, CareScheduleFilter{StudentIDs: ids, Date: CareDate(d)}, "find by student ids and date")
}

func (r CareArrivalNoteRows) list(ctx context.Context, filter CareScheduleFilter, op string) ([]*scheduleModels.StudentArrivalNote, error) {
	values, err := r.rows.Records.ListArrivalNotes(ctx, filter)
	if err != nil {
		return nil, r.rows.scheduleError(op, err)
	}
	return mapRows(values, arrivalNoteToLegacy), nil
}

func (r CareArrivalNoteRows) DeleteByStudentID(ctx context.Context, id int64) error {
	return r.rows.scheduleError("delete by student id", r.rows.Records.DeleteArrivalNotesByStudent(ctx, id))
}

func (r CareArrivalNoteRows) DeletePastNotes(ctx context.Context, d scheduleModels.Date) (int64, error) {
	rows, err := r.rows.Records.DeleteArrivalNotesBefore(ctx, CareDate(d))
	return rows, r.rows.scheduleError("delete past notes", err)
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

var (
	_ scheduleModels.StudentArrivalNoteRepository = CareArrivalNoteRows{}
	_ scheduleModels.StudentPickupNoteRepository  = CarePickupNoteRows{}
)
