package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentCarePlanFromOwnerArrivalException(value careplan.ArrivalException) parentportal.ArrivalExceptionSnapshot {
	return parentportal.ArrivalExceptionSnapshot{
		ID:                value.ID,
		TenantID:          value.TenantID,
		CreatedAt:         value.CreatedAt,
		UpdatedAt:         value.UpdatedAt,
		StudentID:         value.StudentID,
		ExceptionDate:     calendar.Date(value.ExceptionDate),
		ExpectedArrival:   value.ExpectedArrival,
		Reason:            value.Reason,
		Source:            value.Source,
		CreatedBy:         value.CreatedBy,
		CreatedByGuardian: value.CreatedByGuardian,
	}
}

func parentCarePlanFromOwnerArrivalSchedule(value careplan.ArrivalSchedule) parentportal.ArrivalScheduleSnapshot {
	return parentportal.ArrivalScheduleSnapshot{
		ID:              value.ID,
		TenantID:        value.TenantID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
		StudentID:       value.StudentID,
		Weekday:         value.Weekday,
		ExpectedArrival: value.ExpectedArrival,
		Notes:           value.Notes,
		CreatedBy:       value.CreatedBy,
		Source:          value.Source,
		SourceClass:     value.SourceClass,
		SourceLabel:     value.SourceLabel,
	}
}

func parentCarePlanFromOwnerDepartureException(value careplan.DepartureException) parentportal.DepartureExceptionSnapshot {
	return parentportal.DepartureExceptionSnapshot{
		ID:            value.ID,
		TenantID:      value.TenantID,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
		StudentID:     value.StudentID,
		ExceptionDate: calendar.Date(value.ExceptionDate),
		DepartureMode: value.DepartureMode,
		Reason:        value.Reason,
		CreatedBy:     value.CreatedBy,
	}
}

func parentCarePlanFromOwnerEffectivePickupTime(value careplan.EffectivePickupTime) parentportal.EffectivePickupTimeSnapshot {
	return parentportal.EffectivePickupTimeSnapshot{
		Date:              value.Date,
		PickupTime:        value.PickupTime,
		WeekdayName:       value.WeekdayName,
		IsException:       value.IsException,
		Notes:             value.Notes,
		DayNotes:          parentCarePlanSlice(value.DayNotes, parentCarePlanFromOwnerNoteData),
		RegularPickupTime: value.RegularPickupTime,
		ChangedAt:         value.ChangedAt,
		DayDepartureMode:  value.DayDepartureMode,
	}
}

func parentCarePlanToOwnerGuardianPickupChange(value parentportal.GuardianPickupChangeSnapshot) careplan.GuardianPickupChange {
	return careplan.GuardianPickupChange{
		TenantID:          value.TenantID,
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Date:              careplan.Date(value.Date),
		PickupTime:        value.PickupTime,
		Reason:            value.Reason,
	}
}

func parentCarePlanFromOwnerNoteData(value careplan.NoteData) parentportal.NoteDataSnapshot {
	return parentportal.NoteDataSnapshot{
		ID:      value.ID,
		Content: value.Content,
	}
}

func parentCarePlanToOwnerPickupException(value parentportal.PickupExceptionSnapshot) careplan.PickupException {
	return careplan.PickupException{
		ID:                    value.ID,
		TenantID:              value.TenantID,
		CreatedAt:             value.CreatedAt,
		UpdatedAt:             value.UpdatedAt,
		StudentID:             value.StudentID,
		ExceptionDate:         careplan.Date(value.ExceptionDate),
		PickupTime:            value.PickupTime,
		Reason:                value.Reason,
		ExcusedFrom:           value.ExcusedFrom,
		ExcusedReason:         value.ExcusedReason,
		ExcusedCreatedBy:      value.ExcusedCreatedBy,
		ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime,
		ExcusedAuto:           value.ExcusedAuto,
		Source:                value.Source,
		CreatedBy:             value.CreatedBy,
		CreatedByGuardian:     value.CreatedByGuardian,
	}
}

func parentCarePlanFromOwnerPickupException(value careplan.PickupException) parentportal.PickupExceptionSnapshot {
	return parentportal.PickupExceptionSnapshot{
		ManualPartialAbsence:  value.HasManualPartialAbsence(),
		ID:                    value.ID,
		TenantID:              value.TenantID,
		CreatedAt:             value.CreatedAt,
		UpdatedAt:             value.UpdatedAt,
		StudentID:             value.StudentID,
		ExceptionDate:         calendar.Date(value.ExceptionDate),
		PickupTime:            value.PickupTime,
		Reason:                value.Reason,
		ExcusedFrom:           value.ExcusedFrom,
		ExcusedReason:         value.ExcusedReason,
		ExcusedCreatedBy:      value.ExcusedCreatedBy,
		ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime,
		ExcusedAuto:           value.ExcusedAuto,
		Source:                value.Source,
		CreatedBy:             value.CreatedBy,
		CreatedByGuardian:     value.CreatedByGuardian,
	}
}

func parentCarePlanFromOwnerPickupSchedule(value careplan.PickupSchedule) parentportal.PickupScheduleSnapshot {
	return parentportal.PickupScheduleSnapshot{
		ID:               value.ID,
		TenantID:         value.TenantID,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
		StudentID:        value.StudentID,
		Weekday:          value.Weekday,
		PickupTime:       value.PickupTime,
		Notes:            value.Notes,
		CreatedBy:        value.CreatedBy,
		Source:           value.Source,
		CareOfferingID:   value.CareOfferingID,
		CareOfferingName: value.CareOfferingName,
	}
}

func parentCarePlanToOwnerStudentScheduleFilter(value parentportal.StudentScheduleFilterSnapshot) careplan.StudentScheduleFilter {
	return careplan.StudentScheduleFilter{
		StudentIDs: value.StudentIDs,
		Date:       careplan.Date(value.Date),
		From:       careplan.Date(value.From),
		To:         careplan.Date(value.To),
	}
}

func parentCarePlanFromOwnerStudentStatusDay(value careplan.StudentStatusDay) parentportal.StudentStatusDaySnapshot {
	return parentportal.StudentStatusDaySnapshot{
		ID:                value.ID,
		TenantID:          value.TenantID,
		CreatedAt:         value.CreatedAt,
		UpdatedAt:         value.UpdatedAt,
		StudentID:         value.StudentID,
		Date:              calendar.Date(value.Date),
		Status:            value.Status,
		ReportedAt:        value.ReportedAt,
		ClearedAt:         value.ClearedAt,
		Source:            value.Source,
		GuardianAccountID: value.GuardianAccountID,
		Note:              value.Note,
	}
}
