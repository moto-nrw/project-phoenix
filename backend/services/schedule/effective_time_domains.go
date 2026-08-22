package schedule

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

type pickupTimeDomain struct{}

func (pickupTimeDomain) Name() string {
	return "pickup"
}

func (pickupTimeDomain) CollisionPolicy() exceptionCollisionPolicy {
	return exceptionCollisionUpdate
}

func (pickupTimeDomain) ScheduleFields(row *schedule.StudentPickupSchedule) effectiveScheduleFields {
	return effectiveScheduleFields{
		StudentID: row.StudentID,
		Weekday:   row.Weekday,
		Time:      row.PickupTime,
		Notes:     row.Notes,
	}
}

func (pickupTimeDomain) SetScheduleStudentID(row *schedule.StudentPickupSchedule, studentID int64) {
	row.StudentID = studentID
}

func (pickupTimeDomain) ExceptionFields(row *schedule.StudentPickupException) effectiveExceptionFields {
	return effectiveExceptionFields{
		ID:                row.ID,
		StudentID:         row.StudentID,
		Date:              row.ExceptionDate,
		Time:              row.PickupTime,
		Reason:            row.Reason,
		Source:            row.Source,
		CreatedBy:         row.CreatedBy,
		CreatedByGuardian: row.CreatedByGuardian,
		CreatedAt:         row.CreatedAt,
		TenantID:          row.TenantID,
		ExcusedFrom:       row.ExcusedFrom,
		ExcusedReason:     row.ExcusedReason,
		ExcusedCreatedBy:  row.ExcusedCreatedBy,
		ExcusedOwnsTime:   row.ExcusedOwnsPickupTime,
		ExcusedAuto:       row.ExcusedAuto,
	}
}

func (pickupTimeDomain) NewException(fields effectiveExceptionFields) *schedule.StudentPickupException {
	// TIME WITHOUT TIME ZONE columns must not rebind driver-specific date or
	// location metadata. The generic update path already WallClock-normalizes
	// PickupTime; do the same for the partial-excusal cutoff here.
	var excusedFrom = fields.ExcusedFrom
	if excusedFrom != nil {
		clock := timezone.WallClock(*excusedFrom)
		excusedFrom = &clock
	}
	row := &schedule.StudentPickupException{
		StudentID:             fields.StudentID,
		ExceptionDate:         fields.Date,
		PickupTime:            fields.Time,
		Reason:                fields.Reason,
		Source:                fields.Source,
		CreatedBy:             fields.CreatedBy,
		CreatedByGuardian:     fields.CreatedByGuardian,
		ExcusedFrom:           excusedFrom,
		ExcusedReason:         fields.ExcusedReason,
		ExcusedCreatedBy:      fields.ExcusedCreatedBy,
		ExcusedOwnsPickupTime: fields.ExcusedOwnsTime && !fields.TimeChanged,
		ExcusedAuto:           fields.ExcusedAuto,
	}
	row.ID = fields.ID
	row.CreatedAt = fields.CreatedAt
	row.SetTenantID(fields.TenantID)
	return row
}

func (pickupTimeDomain) AssignException(
	target *schedule.StudentPickupException,
	source *schedule.StudentPickupException,
) {
	*target = *source
}

func (pickupTimeDomain) NoteFields(row *schedule.StudentPickupNote) effectiveNoteFields {
	return effectiveNoteFields{
		ID:        row.ID,
		StudentID: row.StudentID,
		Content:   row.Content,
	}
}

type arrivalTimeDomain struct{}

func (arrivalTimeDomain) Name() string {
	return "arrival"
}

func (arrivalTimeDomain) CollisionPolicy() exceptionCollisionPolicy {
	return exceptionCollisionReject
}

func (arrivalTimeDomain) ScheduleFields(row *schedule.StudentArrivalSchedule) effectiveScheduleFields {
	return effectiveScheduleFields{
		StudentID: row.StudentID,
		Weekday:   row.Weekday,
		Time:      row.ExpectedArrival,
		Notes:     row.Notes,
	}
}

func (arrivalTimeDomain) SetScheduleStudentID(row *schedule.StudentArrivalSchedule, studentID int64) {
	row.StudentID = studentID
}

func (arrivalTimeDomain) ExceptionFields(row *schedule.StudentArrivalException) effectiveExceptionFields {
	return effectiveExceptionFields{
		ID:                row.ID,
		StudentID:         row.StudentID,
		Date:              row.ExceptionDate,
		Time:              row.ExpectedArrival,
		Reason:            row.Reason,
		Source:            row.Source,
		CreatedBy:         row.CreatedBy,
		CreatedByGuardian: row.CreatedByGuardian,
		CreatedAt:         row.CreatedAt,
		TenantID:          row.TenantID,
	}
}

func (arrivalTimeDomain) NewException(fields effectiveExceptionFields) *schedule.StudentArrivalException {
	row := &schedule.StudentArrivalException{
		StudentID:         fields.StudentID,
		ExceptionDate:     fields.Date,
		ExpectedArrival:   fields.Time,
		Reason:            fields.Reason,
		Source:            fields.Source,
		CreatedBy:         fields.CreatedBy,
		CreatedByGuardian: fields.CreatedByGuardian,
	}
	row.ID = fields.ID
	row.CreatedAt = fields.CreatedAt
	row.SetTenantID(fields.TenantID)
	return row
}

func (arrivalTimeDomain) AssignException(
	target *schedule.StudentArrivalException,
	source *schedule.StudentArrivalException,
) {
	*target = *source
}

func (arrivalTimeDomain) NoteFields(row *schedule.StudentArrivalNote) effectiveNoteFields {
	return effectiveNoteFields{
		ID:        row.ID,
		StudentID: row.StudentID,
		Content:   row.Content,
	}
}
