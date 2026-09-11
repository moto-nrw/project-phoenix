package legacy

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	activityService "github.com/moto-nrw/project-phoenix/services/activities"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
)

type activityCatalogAdapter struct {
	service activityService.ActivityService
}

func ActivityCatalog(service activityService.ActivityService) facilitiesService.ActivityCatalog {
	return activityCatalogAdapter{service: service}
}

func (a activityCatalogAdapter) ListActivities(ctx context.Context, name string) ([]facilitiesService.SystemActivity, error) {
	groups, err := a.service.ListGroups(ctx, &activityModels.GroupListQuery{Name: name})
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesService.SystemActivity, 0, len(groups))
	for _, group := range groups {
		result = append(result, systemActivity(group))
	}
	return result, nil
}

func (a activityCatalogAdapter) CreateActivity(ctx context.Context, input facilitiesService.SystemActivity) (facilitiesService.SystemActivity, error) {
	created, err := a.service.CreateGroup(ctx, &activityModels.Group{
		Name: input.Name, MaxParticipants: input.MaxParticipants, IsOpen: input.IsOpen,
		CategoryID: input.CategoryID, PlannedRoomID: input.PlannedRoomID,
		IsSystem: input.IsSystem, CreatedBy: input.CreatedBy,
	}, []int64{}, []*activityModels.Schedule{})
	return systemActivity(created), err
}

func (a activityCatalogAdapter) ListCategories(ctx context.Context) ([]facilitiesService.SystemCategory, error) {
	categories, err := a.service.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesService.SystemCategory, 0, len(categories))
	for _, category := range categories {
		result = append(result, systemCategory(category))
	}
	return result, nil
}

func (a activityCatalogAdapter) CreateCategory(ctx context.Context, input facilitiesService.SystemCategory) (facilitiesService.SystemCategory, error) {
	created, err := a.service.CreateCategory(ctx, &activityModels.Category{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	return systemCategory(created), err
}

func systemActivity(group *activityModels.Group) facilitiesService.SystemActivity {
	if group == nil {
		return facilitiesService.SystemActivity{}
	}
	return facilitiesService.SystemActivity{
		ID: group.ID, Name: group.Name, MaxParticipants: group.MaxParticipants, IsOpen: group.IsOpen,
		CategoryID: group.CategoryID, PlannedRoomID: group.PlannedRoomID,
		IsSystem: group.IsSystem, CreatedBy: group.CreatedBy,
	}
}

func systemCategory(category *activityModels.Category) facilitiesService.SystemCategory {
	if category == nil {
		return facilitiesService.SystemCategory{}
	}
	return facilitiesService.SystemCategory{
		ID: category.ID, Name: category.Name, Description: category.Description,
		Color: category.Color, IsSystem: category.IsSystem,
	}
}

type openGroupCatalogAdapter struct{ service activeService.Service }

func OpenGroupCatalog(service activeService.Service) facilitiesService.OpenGroupCatalog {
	return openGroupCatalogAdapter{service: service}
}

func (a openGroupCatalogAdapter) ListByRoom(ctx context.Context, roomID int64) ([]facilitiesService.OpenGroup, error) {
	groups, err := a.service.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	today := timezone.TodayDate()
	result := make([]facilitiesService.OpenGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, facilitiesService.OpenGroup{
			ID: group.ID, StartTime: group.StartTime, EndTime: group.EndTime,
			IsToday: timezone.DateFromTime(group.StartTime) == today,
		})
	}
	return result, nil
}

func (a openGroupCatalogAdapter) ListSupervisors(ctx context.Context, groupIDs []int64) ([]facilitiesService.OpenGroupSupervisor, error) {
	supervisors, err := a.service.FindSupervisorsByActiveGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesService.OpenGroupSupervisor, 0, len(supervisors))
	for _, supervisor := range supervisors {
		row := facilitiesService.OpenGroupSupervisor{
			ID: supervisor.ID, GroupID: supervisor.GroupID,
			StaffID: supervisor.StaffID, Ended: supervisor.EndDate != nil,
		}
		if supervisor.Staff != nil && supervisor.Staff.Person != nil {
			row.FirstName = supervisor.Staff.Person.FirstName
			row.LastName = supervisor.Staff.Person.LastName
		}
		result = append(result, row)
	}
	return result, nil
}

func (a openGroupCatalogAdapter) ListVisits(ctx context.Context, groupID int64) ([]facilitiesService.OpenGroupVisit, error) {
	visits, err := a.service.FindVisitsByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesService.OpenGroupVisit, 0, len(visits))
	for _, visit := range visits {
		result = append(result, facilitiesService.OpenGroupVisit{ExitTime: visit.ExitTime})
	}
	return result, nil
}
