package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

func LegacyServiceCategory(category timetable.Category) *activityContract.Category {
	result := &activityContract.Category{
		Name: category.Name, Description: category.Description, Color: category.Color, IsSystem: category.IsSystem,
		ShiftTypeID: category.ShiftTypeID, ArchivedAt: category.ArchivedAt,
	}
	result.ID = category.ID
	result.CreatedAt = category.CreatedAt
	result.UpdatedAt = category.UpdatedAt
	result.SetTenantID(category.TenantID)
	return result
}

func LegacyServiceGroups(groups []timetable.Group) []*activityContract.Group {
	result := make([]*activityContract.Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, LegacyServiceGroup(group))
	}
	return result
}

func LegacyServiceGroup(group timetable.Group) *activityContract.Group {
	result := &activityContract.Group{
		Name:            group.Name,
		MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff, IsOpen: group.IsOpen,
		CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID, PlannedRoomID: group.PlannedRoomID,
		CreatedBy: group.CreatedBy, Type: group.Type, EducationGroupID: group.EducationGroupID, ListKind: group.ListKind,
		IsTemplate: group.IsTemplate, IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt,
		SeriesRootID: group.SeriesRootID, CalendarPeriodID: group.CalendarPeriodID,
		TargetGroupType: group.TargetGroupType, TargetGradeLevel: group.TargetGradeLevel,
		TargetSchoolClass: group.TargetSchoolClass, SourceCareOfferingIDs: group.SourceCareOfferingIDs,
		SourceGradeLevels: group.SourceGradeLevels, SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes,
	}
	result.ID = group.ID
	result.CreatedAt = group.CreatedAt
	result.UpdatedAt = group.UpdatedAt
	result.SetTenantID(group.TenantID)
	if group.Category != nil {
		result.Category = LegacyServiceCategory(*group.Category)
	}
	return result
}
