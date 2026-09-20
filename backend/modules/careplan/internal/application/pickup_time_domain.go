package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupTimeDomain struct{}

func (PickupTimeDomain) Name() string {
	return "pickup"
}

func (PickupTimeDomain) CollisionPolicy() domain.ExceptionCollisionPolicy {
	return domain.ExceptionCollisionUpdate
}

func (PickupTimeDomain) ScheduleFields(row *careplan.PickupSchedule) domain.EffectiveScheduleFields {
	return domain.EffectiveScheduleFields{
		StudentID: row.StudentID,
		Weekday:   row.Weekday,
		Time:      row.PickupTime,
		Notes:     row.Notes,
	}
}

func (PickupTimeDomain) SetScheduleStudentID(row *careplan.PickupSchedule, studentID int64) {
	row.StudentID = studentID
}

func (PickupTimeDomain) ExceptionFields(row *careplan.PickupException) domain.EffectiveExceptionFields {
	return domain.EffectiveExceptionFields{
		ID:                row.ID,
		StudentID:         row.StudentID,
		Date:              calendar.Date(row.ExceptionDate),
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

func (PickupTimeDomain) NewException(fields domain.EffectiveExceptionFields) *careplan.PickupException {
	// TIME WITHOUT TIME ZONE columns must not rebind driver-specific date or
	// location metadata. The generic update path already WallClock-normalizes
	// PickupTime; do the same for the partial-excusal cutoff here.
	var excusedFrom = fields.ExcusedFrom
	if excusedFrom != nil {
		clock := calendar.NormalizeWallClock(*excusedFrom)
		excusedFrom = &clock
	}
	row := &careplan.PickupException{
		StudentID:             fields.StudentID,
		ExceptionDate:         careplan.Date(fields.Date),
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

func (PickupTimeDomain) AssignException(
	target *careplan.PickupException,
	source *careplan.PickupException,
) {
	*target = *source
}

func (PickupTimeDomain) NoteFields(row *careplan.PickupNote) domain.EffectiveNoteFields {
	return domain.EffectiveNoteFields{
		ID:        row.ID,
		StudentID: row.StudentID,
		Content:   row.Content,
	}
}
