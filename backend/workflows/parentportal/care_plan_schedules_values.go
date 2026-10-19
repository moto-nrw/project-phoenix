package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

func carePlanToAppArrivalException(value ArrivalExceptionSnapshot) care.ArrivalException {
	return care.ArrivalException{
		ID:                value.ID,
		TenantID:          value.TenantID,
		CreatedAt:         value.CreatedAt,
		UpdatedAt:         value.UpdatedAt,
		StudentID:         value.StudentID,
		ExceptionDate:     value.ExceptionDate,
		ExpectedArrival:   value.ExpectedArrival,
		Reason:            value.Reason,
		Source:            value.Source,
		CreatedBy:         value.CreatedBy,
		CreatedByGuardian: value.CreatedByGuardian,
	}
}

func carePlanToAppArrivalSchedule(value ArrivalScheduleSnapshot) care.ArrivalSchedule {
	return care.ArrivalSchedule{
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

func carePlanToAppDepartureException(value DepartureExceptionSnapshot) care.DepartureException {
	return care.DepartureException{
		ID:            value.ID,
		TenantID:      value.TenantID,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
		StudentID:     value.StudentID,
		ExceptionDate: value.ExceptionDate,
		DepartureMode: value.DepartureMode,
		Reason:        value.Reason,
		CreatedBy:     value.CreatedBy,
	}
}

func carePlanToAppEffectivePickupTime(value EffectivePickupTimeSnapshot) care.EffectivePickupTime {
	return care.EffectivePickupTime{
		Date:              value.Date,
		PickupTime:        value.PickupTime,
		WeekdayName:       value.WeekdayName,
		IsException:       value.IsException,
		Notes:             value.Notes,
		DayNotes:          carePlanValueSlice(value.DayNotes, carePlanToAppNoteData),
		RegularPickupTime: value.RegularPickupTime,
		ChangedAt:         value.ChangedAt,
		DayDepartureMode:  value.DayDepartureMode,
	}
}

func carePlanFromAppGuardianPickupChange(value care.GuardianPickupChange) GuardianPickupChangeSnapshot {
	return GuardianPickupChangeSnapshot{
		TenantID:          value.TenantID,
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Date:              value.Date,
		PickupTime:        value.PickupTime,
		Reason:            value.Reason,
	}
}
func carePlanToAppNoteData(value NoteDataSnapshot) care.NoteData {
	return care.NoteData{
		ID:      value.ID,
		Content: value.Content,
	}
}

func carePlanToAppPickupException(value PickupExceptionSnapshot) care.PickupException {
	return care.PickupException{
		ManualPartialAbsence:  value.ManualPartialAbsence,
		ID:                    value.ID,
		TenantID:              value.TenantID,
		CreatedAt:             value.CreatedAt,
		UpdatedAt:             value.UpdatedAt,
		StudentID:             value.StudentID,
		ExceptionDate:         value.ExceptionDate,
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
func carePlanFromAppPickupException(value care.PickupException) PickupExceptionSnapshot {
	return PickupExceptionSnapshot{
		ManualPartialAbsence:  value.ManualPartialAbsence,
		ID:                    value.ID,
		TenantID:              value.TenantID,
		CreatedAt:             value.CreatedAt,
		UpdatedAt:             value.UpdatedAt,
		StudentID:             value.StudentID,
		ExceptionDate:         value.ExceptionDate,
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
func carePlanToAppPickupSchedule(value PickupScheduleSnapshot) care.PickupSchedule {
	return care.PickupSchedule{
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

func carePlanFromAppStudentScheduleFilter(value care.StudentScheduleFilter) StudentScheduleFilterSnapshot {
	return StudentScheduleFilterSnapshot{
		StudentIDs: value.StudentIDs,
		Date:       value.Date,
		From:       value.From,
		To:         value.To,
	}
}
func carePlanToAppStudentStatusDay(value StudentStatusDaySnapshot) care.StudentStatusDay {
	return care.StudentStatusDay{
		ID:                value.ID,
		TenantID:          value.TenantID,
		CreatedAt:         value.CreatedAt,
		UpdatedAt:         value.UpdatedAt,
		StudentID:         value.StudentID,
		Date:              value.Date,
		Status:            value.Status,
		ReportedAt:        value.ReportedAt,
		ClearedAt:         value.ClearedAt,
		Source:            value.Source,
		GuardianAccountID: value.GuardianAccountID,
		Note:              value.Note,
	}
}
