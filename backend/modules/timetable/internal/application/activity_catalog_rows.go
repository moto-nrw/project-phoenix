package application

import (
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func LegacyServiceCategory(category timetable.Category) *activities.Category {
	result := &activities.Category{
		Name: category.Name, Description: category.Description, Color: category.Color, IsSystem: category.IsSystem,
		ShiftTypeID: category.ShiftTypeID, ArchivedAt: category.ArchivedAt,
	}
	result.ID = category.ID
	result.CreatedAt = category.CreatedAt
	result.UpdatedAt = category.UpdatedAt
	result.SetTenantID(category.TenantID)
	return result
}

func LegacyServiceGroups(groups []timetable.Group) []*activities.Group {
	result := make([]*activities.Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, LegacyServiceGroup(group))
	}
	return result
}

func LegacyServiceGroup(group timetable.Group) *activities.Group {
	result := &activities.Group{
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
