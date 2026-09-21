package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ArrivalTimeDomain struct{}

func (ArrivalTimeDomain) Name() string {
	return "arrival"
}

func (ArrivalTimeDomain) CollisionPolicy() domain.ExceptionCollisionPolicy {
	return domain.ExceptionCollisionReject
}

func (ArrivalTimeDomain) ScheduleFields(row *careplan.ArrivalSchedule) domain.EffectiveScheduleFields {
	return domain.EffectiveScheduleFields{
		StudentID: row.StudentID,
		Weekday:   row.Weekday,
		Time:      row.ExpectedArrival,
		Notes:     row.Notes,
	}
}

func (ArrivalTimeDomain) SetScheduleStudentID(row *careplan.ArrivalSchedule, studentID int64) {
	row.StudentID = studentID
}

func (ArrivalTimeDomain) ExceptionFields(row *careplan.ArrivalException) domain.EffectiveExceptionFields {
	return domain.EffectiveExceptionFields{
		ID:                row.ID,
		StudentID:         row.StudentID,
		Date:              calendar.Date(row.ExceptionDate),
		Time:              row.ExpectedArrival,
		Reason:            row.Reason,
		Source:            row.Source,
		CreatedBy:         row.CreatedBy,
		CreatedByGuardian: row.CreatedByGuardian,
		CreatedAt:         row.CreatedAt,
		TenantID:          row.TenantID,
	}
}

func (ArrivalTimeDomain) NewException(fields domain.EffectiveExceptionFields) *careplan.ArrivalException {
	row := &careplan.ArrivalException{
		StudentID:         fields.StudentID,
		ExceptionDate:     careplan.Date(fields.Date),
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

func (ArrivalTimeDomain) AssignException(
	target *careplan.ArrivalException,
	source *careplan.ArrivalException,
) {
	*target = *source
}

func (ArrivalTimeDomain) NoteFields(row *careplan.ArrivalNote) domain.EffectiveNoteFields {
	return domain.EffectiveNoteFields{
		ID:        row.ID,
		StudentID: row.StudentID,
		Content:   row.Content,
	}
}
