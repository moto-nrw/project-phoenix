package compose

import (
	"errors"
	"fmt"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// The retained models/schedule student-schedule repositories over the Care
// Plan records (#3351). They perform no persistence of their own: every
// adapter maps the row models onto the records and keeps the error shapes the
// retained callers classify on.

type CareArrivalScheduleRows struct{ rows CarePlanRows }
type CareArrivalExceptionRows struct{ rows CarePlanRows }
type CareArrivalNoteRows struct{ rows CarePlanRows }
type CarePickupScheduleRows struct{ rows CarePlanRows }
type CarePickupExceptionRows struct{ rows CarePlanRows }
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

func NewCareArrivalScheduleRows(rows CarePlanRows) CareArrivalScheduleRows {
	return CareArrivalScheduleRows{rows: rows}
}
func NewCareArrivalExceptionRows(rows CarePlanRows) CareArrivalExceptionRows {
	return CareArrivalExceptionRows{rows: rows}
}
func NewCareArrivalNoteRows(rows CarePlanRows) CareArrivalNoteRows {
	return CareArrivalNoteRows{rows: rows}
}
func NewCarePickupScheduleRows(rows CarePlanRows) CarePickupScheduleRows {
	return CarePickupScheduleRows{rows: rows}
}
func NewCarePickupExceptionRows(rows CarePlanRows) CarePickupExceptionRows {
	return CarePickupExceptionRows{rows: rows}
}
func NewCarePickupNoteRows(rows CarePlanRows) CarePickupNoteRows {
	return CarePickupNoteRows{rows: rows}
}

func invalidLegacyEntity(name string) error {
	return fmt.Errorf("%s cannot be nil or zero value", name)
}
func legacyScheduleID(raw any) (int64, error) {
	switch value := raw.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}

// scheduleError keeps the retained repository error shape: a missing
// schedule row classifies as not found, everything else is wrapped with its
// operation.
func (c CarePlanRows) scheduleError(op string, err error) error {
	if errors.Is(err, ErrCareScheduleRecordNotFound) {
		return c.Errors.MissingRecordError(op)
	}
	return c.Errors.WrapRecordError(op, err)
}

func mapRows[T any, R any](values []T, convert func(T) R) []R {
	result := make([]R, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func arrivalScheduleToPublic(v *scheduleModels.StudentArrivalSchedule) CareArrivalSchedule {
	return CareArrivalSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, ExpectedArrival: v.ExpectedArrival, Notes: v.Notes, CreatedBy: v.CreatedBy}
}
func arrivalScheduleToLegacy(v CareArrivalSchedule) *scheduleModels.StudentArrivalSchedule {
	row := new(scheduleModels.StudentArrivalSchedule)
	applyArrivalSchedule(row, v)
	return row
}
func applyArrivalSchedule(r *scheduleModels.StudentArrivalSchedule, v CareArrivalSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.ExpectedArrival, r.Notes, r.CreatedBy = v.StudentID, v.Weekday, v.ExpectedArrival, v.Notes, v.CreatedBy
}
func arrivalExceptionToPublic(v *scheduleModels.StudentArrivalException) CareArrivalException {
	return CareArrivalException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: CareDate(v.ExceptionDate), ExpectedArrival: v.ExpectedArrival, Reason: v.Reason, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func arrivalExceptionToLegacy(v CareArrivalException) *scheduleModels.StudentArrivalException {
	row := new(scheduleModels.StudentArrivalException)
	applyArrivalException(row, v)
	return row
}
func applyArrivalException(r *scheduleModels.StudentArrivalException, v CareArrivalException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.ExpectedArrival, r.Reason, r.Source, r.CreatedBy, r.CreatedByGuardian = v.StudentID, scheduleModels.Date(v.ExceptionDate), v.ExpectedArrival, v.Reason, v.Source, v.CreatedBy, v.CreatedByGuardian
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
func pickupScheduleToPublic(v *scheduleModels.StudentPickupSchedule) CarePickupSchedule {
	return CarePickupSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, PickupTime: v.PickupTime, Notes: v.Notes, CreatedBy: v.CreatedBy, Source: v.Source, CareOfferingID: v.CareOfferingID}
}
func pickupScheduleToLegacy(v CarePickupSchedule) *scheduleModels.StudentPickupSchedule {
	row := new(scheduleModels.StudentPickupSchedule)
	applyPickupSchedule(row, v)
	return row
}
func applyPickupSchedule(r *scheduleModels.StudentPickupSchedule, v CarePickupSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.PickupTime, r.Notes, r.CreatedBy, r.Source, r.CareOfferingID = v.StudentID, v.Weekday, v.PickupTime, v.Notes, v.CreatedBy, v.Source, v.CareOfferingID
}
func pickupExceptionToPublic(v *scheduleModels.StudentPickupException) CarePickupException {
	return CarePickupException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: CareDate(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupExceptionToLegacy(v CarePickupException) *scheduleModels.StudentPickupException {
	row := new(scheduleModels.StudentPickupException)
	applyPickupException(row, v)
	return row
}
func applyPickupException(r *scheduleModels.StudentPickupException, v CarePickupException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.PickupTime, r.Reason = v.StudentID, scheduleModels.Date(v.ExceptionDate), v.PickupTime, v.Reason
	r.ExcusedFrom, r.ExcusedReason, r.ExcusedCreatedBy, r.ExcusedOwnsPickupTime, r.ExcusedAuto = v.ExcusedFrom, v.ExcusedReason, v.ExcusedCreatedBy, v.ExcusedOwnsPickupTime, v.ExcusedAuto
	r.Source, r.CreatedBy, r.CreatedByGuardian = v.Source, v.CreatedBy, v.CreatedByGuardian
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

var (
	_ scheduleModels.StudentArrivalScheduleRepository  = CareArrivalScheduleRows{}
	_ scheduleModels.StudentArrivalExceptionRepository = CareArrivalExceptionRows{}
	_ scheduleModels.StudentArrivalNoteRepository      = CareArrivalNoteRows{}
	_ scheduleModels.StudentPickupScheduleRepository   = CarePickupScheduleRows{}
	_ scheduleModels.StudentPickupExceptionRepository  = CarePickupExceptionRows{}
	_ scheduleModels.StudentPickupNoteRepository       = CarePickupNoteRows{}
)
