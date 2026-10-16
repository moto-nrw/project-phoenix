package application

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func careExitEnrollmentChangesToPublic(value domain.CareExitEnrollmentChanges) timetable.CareExitEnrollmentChanges {
	result := timetable.CareExitEnrollmentChanges{
		Deleted: make([]timetable.CareExitEnrollment, 0, len(value.Deleted)),
		Capped:  make([]timetable.CareExitEnrollmentCap, 0, len(value.Capped)),
	}
	for _, enrollment := range value.Deleted {
		result.Deleted = append(result.Deleted, careExitEnrollmentToPublic(enrollment))
	}
	for _, cap := range value.Capped {
		result.Capped = append(result.Capped, timetable.CareExitEnrollmentCap{
			TenantID: cap.TenantID, StudentID: cap.StudentID, ID: cap.ID, PreviousValidUntil: cap.PreviousValidUntil,
		})
	}
	return result
}

func careExitEnrollmentToPublic(value domain.CareExitEnrollment) timetable.CareExitEnrollment {
	return timetable.CareExitEnrollment{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
		ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
	}
}

func careExitEnrollmentRemovalsToDomain(values []timetable.CareExitEnrollmentRemoval) []domain.CareExitEnrollmentRemoval {
	result := make([]domain.CareExitEnrollmentRemoval, 0, len(values))
	for _, value := range values {
		result = append(result, domain.CareExitEnrollmentRemoval{
			CareExitEnrollment: domain.CareExitEnrollment{
				ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
				ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
				CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
				SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
			},
			WasDeleted: value.WasDeleted, PreviousValidUntil: value.PreviousValidUntil,
		})
	}
	return result
}

func categoryToPublic(value domain.Category) timetable.Category {
	return timetable.Category{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Description: value.Description, Color: value.Color, IsSystem: value.IsSystem,
		ShiftTypeID: value.ShiftTypeID, ArchivedAt: value.ArchivedAt,
	}
}

func groupTargetToPublic(value domain.GroupTarget) timetable.GroupTarget {
	return timetable.GroupTarget{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ActivityGroupID: value.ActivityGroupID,
		TargetGroupType: value.TargetGroupType, TargetGradeLevel: value.TargetGradeLevel,
		TargetSchoolClass: value.TargetSchoolClass, EducationGroupID: value.EducationGroupID,
		EducationGroupName: value.EducationGroupName,
	}
}

func scheduleToPublic(value domain.Schedule) timetable.Schedule {
	return timetable.Schedule{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: value.ValidUntil, ValidFrom: value.ValidFrom,
	}
}

func scheduleFields(value timetable.ScheduleInput) domain.ScheduleFields {
	return domain.ScheduleFields{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: value.ValidUntil, ValidFrom: value.ValidFrom,
	}
}

func plannedSupervisorToPublic(value domain.PlannedSupervisor) timetable.PlannedSupervisor {
	return timetable.PlannedSupervisor{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary, ValidFrom: value.ValidFrom,
		ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
}

func plannedSupervisorFields(value timetable.PlannedSupervisorInput) domain.PlannedSupervisorFields {
	return domain.PlannedSupervisorFields{StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
}

func studentEnrollmentToPublic(value domain.StudentEnrollment) timetable.StudentEnrollment {
	return timetable.StudentEnrollment{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
}

func studentEnrollmentFields(value timetable.StudentEnrollmentInput) domain.StudentEnrollmentFields {
	return domain.StudentEnrollmentFields{StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID,
		EnrollmentRequestChildID: value.EnrollmentRequestChildID, SelectedWeekdays: value.SelectedWeekdays,
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
}

func timeframeToPublic(value domain.Timeframe) timetable.Timeframe {
	return timetable.Timeframe{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StartTime: value.StartTime, EndTime: value.EndTime, IsActive: value.IsActive, Description: value.Description}
}

func timeframeFields(value timetable.TimeframeInput) domain.TimeframeFields {
	return domain.TimeframeFields{StartTime: value.StartTime, EndTime: value.EndTime,
		IsActive: value.IsActive, Description: value.Description}
}

func planningTrackToPublic(value domain.PlanningTrack) timetable.PlanningTrack {
	return timetable.PlanningTrack{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
}

func planningTrackFields(value timetable.PlanningTrackInput) domain.PlanningTrackFields {
	return domain.PlanningTrackFields{Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
}

func recurrenceRuleToPublic(value domain.RecurrenceRule) timetable.RecurrenceRule {
	return timetable.RecurrenceRule{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Frequency: value.Frequency, IntervalCount: value.IntervalCount, Weekdays: value.Weekdays, MonthDays: value.MonthDays,
		EndDate: value.EndDate, Count: value.Count}
}

func recurrenceRuleFields(value timetable.RecurrenceRuleInput) domain.RecurrenceRuleFields {
	return domain.RecurrenceRuleFields{Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndDate: value.EndDate, Count: value.Count}
}

func activityExceptionToPublic(value domain.ActivityException) timetable.ActivityException {
	return timetable.ActivityException{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate, ExceptionType: value.ExceptionType,
		StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}

func activityExceptionFields(value timetable.ActivityExceptionInput) domain.ActivityExceptionFields {
	return domain.ActivityExceptionFields{
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate, ExceptionType: value.ExceptionType,
		StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}

func instanceStaffToPublic(value domain.InstanceStaff) timetable.InstanceStaff {
	return timetable.InstanceStaff{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		InstanceID: value.InstanceID, StaffID: value.StaffID, RoomID: value.RoomID, IsPrimary: value.IsPrimary,
		IsSubstitute: value.IsSubstitute, IsAbsent: value.IsAbsent, AbsenceReason: value.AbsenceReason,
		SickAbsenceID: value.SickAbsenceID,
	}
}

func MapError(err error) error {
	if errors.Is(err, domain.ErrOffboardingConflict) {
		return timetable.ErrOffboardingConflict
	}
	if errors.Is(err, domain.ErrInvalidRecurrenceRange) {
		return timetable.ErrInvalidRecurrenceRange
	}
	switch {
	case errors.Is(err, domain.ErrCategoryNotFound):
		return timetable.ErrCategoryNotFound
	case errors.Is(err, domain.ErrCategoryNameConflict):
		return timetable.ErrCategoryNameExists
	case errors.Is(err, domain.ErrUnknownCategoryIDs):
		return timetable.ErrUnknownCategoryIDs
	case errors.Is(err, domain.ErrSystemCategoryProtected):
		return timetable.ErrSystemCategoryProtected
	case errors.Is(err, domain.ErrSystemCategoryName):
		return timetable.ErrSystemCategoryName
	case errors.Is(err, domain.ErrCategoryArchived):
		return timetable.ErrCategoryArchived
	case errors.Is(err, domain.ErrGroupNotFound):
		return timetable.ErrGroupNotFound
	case errors.Is(err, domain.ErrScheduleNotFound):
		return timetable.ErrScheduleNotFound
	case errors.Is(err, domain.ErrPlannedSupervisorNotFound):
		return timetable.ErrPlannedSupervisorNotFound
	case errors.Is(err, domain.ErrStudentEnrollmentNotFound):
		return timetable.ErrStudentEnrollmentNotFound
	case errors.Is(err, domain.ErrTimeframeNotFound):
		return timetable.ErrTimeframeNotFound
	case errors.Is(err, domain.ErrPlanningTrackNotFound):
		return timetable.ErrPlanningTrackNotFound
	case errors.Is(err, domain.ErrPlanningTrackNameConflict):
		return fmt.Errorf("%w: %w", timetable.ErrPlanningTrackNameExists, err)
	case errors.Is(err, domain.ErrRecurrenceRuleNotFound):
		return timetable.ErrRecurrenceRuleNotFound
	case errors.Is(err, domain.ErrActivityExceptionNotFound):
		return timetable.ErrActivityExceptionNotFound
	case errors.Is(err, domain.ErrActivityInstanceNotFound):
		return timetable.ErrActivityInstanceNotFound
	case errors.Is(err, domain.ErrInstanceStaffNotFound):
		return timetable.ErrInstanceStaffNotFound
	case errors.Is(err, domain.ErrInstanceStudentNotFound):
		return timetable.ErrInstanceStudentNotFound
	case errors.Is(err, domain.ErrPickupExtensionNotFound):
		return timetable.ErrPickupExtensionNotFound
	case errors.Is(err, domain.ErrPickupExtensionBlockGone):
		return timetable.ErrPickupExtensionBlockGone
	default:
		return err
	}
}

func activityInstanceToPublic(value domain.ActivityInstance) timetable.ActivityInstance {
	return timetable.ActivityInstance{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Date: value.Date, ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID,
		Title: value.Title, Description: value.Description, StartTime: value.StartTime, EndTime: value.EndTime,
		RoomID: value.RoomID, RequiredStaff: value.RequiredStaff, Status: value.Status,
		ListKind: value.ListKind, IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy,
	}
}

func instanceStudentToPublic(value domain.InstanceStudent) timetable.InstanceStudent {
	return timetable.InstanceStudent{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID}
}
