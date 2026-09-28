package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// Group mappings between the public timetable contract and the domain.

func groupToPublic(value domain.Group) timetable.Group {
	result := timetable.Group{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, MaxParticipants: value.MaxParticipants, RequiredStaff: value.RequiredStaff, IsOpen: value.IsOpen,
		CategoryID: value.CategoryID, PlanningTrackID: value.PlanningTrackID, PlannedRoomID: value.PlannedRoomID,
		CreatedBy: value.CreatedBy, Type: value.Type, EducationGroupID: value.EducationGroupID, ListKind: value.ListKind,
		IsTemplate: value.IsTemplate, IsSystem: value.IsSystem, ArchivedAt: value.ArchivedAt, SeriesRootID: value.SeriesRootID,
		CalendarPeriodID: value.CalendarPeriodID, TargetGroupType: value.TargetGroupType,
		TargetGradeLevel: value.TargetGradeLevel, TargetSchoolClass: value.TargetSchoolClass,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses, Notes: value.Notes, IncludeClosingDays: value.IncludeClosingDays,
		SeriesLastDay: value.SeriesLastDay,
	}
	if value.Category != nil {
		category := categoryToPublic(*value.Category)
		result.Category = &category
	}
	return result
}

func groupFields(value timetable.GroupInput) domain.GroupFields {
	return domain.GroupFields{
		Name: value.Name, MaxParticipants: value.MaxParticipants, RequiredStaff: value.RequiredStaff, IsOpen: value.IsOpen,
		CategoryID: value.CategoryID, PlanningTrackID: value.PlanningTrackID, PlannedRoomID: value.PlannedRoomID,
		CreatedBy: value.CreatedBy, Type: value.Type, EducationGroupID: value.EducationGroupID, ListKind: value.ListKind,
		IsTemplate: value.IsTemplate, IsSystem: value.IsSystem, ArchivedAt: value.ArchivedAt, SeriesRootID: value.SeriesRootID,
		CalendarPeriodID: value.CalendarPeriodID, TargetGroupType: value.TargetGroupType,
		TargetGradeLevel: value.TargetGradeLevel, TargetSchoolClass: value.TargetSchoolClass,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses, Notes: value.Notes, IncludeClosingDays: value.IncludeClosingDays,
		SeriesLastDay: value.SeriesLastDay,
	}
}

func templateFields(value timetable.TemplateUpdate) domain.TemplateFields {
	return domain.TemplateFields{
		Name: value.Name, Type: value.Type, CategoryID: value.CategoryID,
		PlanningTrackID: value.PlanningTrackID, PlanningTrackIDProvided: value.PlanningTrackIDProvided,
		RoomID: value.RoomID, EducationGroupID: value.EducationGroupID,
		MaxParticipants: value.MaxParticipants, MaxParticipantsProvided: value.MaxParticipantsProvided,
		RequiredStaff: value.RequiredStaff, CalendarPeriodID: value.CalendarPeriodID,
		TargetGroupType: value.TargetGroupType, TargetGradeLevel: value.TargetGradeLevel,
		TargetSchoolClass: value.TargetSchoolClass, ListKind: value.ListKind, Notes: value.Notes,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses, IncludeClosingDays: value.IncludeClosingDays,
		SeriesLastDay: value.SeriesLastDay, SeriesLastDayProvided: value.SeriesLastDayProvided,
	}
}
