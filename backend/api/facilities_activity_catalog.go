package api

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

type activityCatalogAdapter struct {
	service timetable.Catalog
}

func FacilitiesActivityCatalog(service activityContract.ActivityService) facilities.ActivityCatalog {
	return activityCatalogAdapter{service: timetableCompose.NewCatalog(service)}
}

func (a activityCatalogAdapter) ListActivities(ctx context.Context, name string) ([]facilities.SystemActivity, error) {
	groups, err := a.service.ListCatalogGroups(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]facilities.SystemActivity, 0, len(groups))
	for _, group := range groups {
		result = append(result, systemActivity(&group))
	}
	return result, nil
}

func (a activityCatalogAdapter) CreateActivity(ctx context.Context, input facilities.SystemActivity) (facilities.SystemActivity, error) {
	created, err := a.service.CreateCatalogGroup(ctx, timetable.GroupInput{
		Name: input.Name, MaxParticipants: input.MaxParticipants, IsOpen: input.IsOpen,
		CategoryID: input.CategoryID, PlannedRoomID: input.PlannedRoomID,
		IsSystem: input.IsSystem, CreatedBy: input.CreatedBy,
	})
	return systemActivity(&created), err
}

func (a activityCatalogAdapter) ListCategories(ctx context.Context) ([]facilities.SystemCategory, error) {
	categories, err := a.service.ListCatalogCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]facilities.SystemCategory, 0, len(categories))
	for _, category := range categories {
		result = append(result, systemCategory(&category))
	}
	return result, nil
}

func (a activityCatalogAdapter) CreateCategory(ctx context.Context, input facilities.SystemCategory) (facilities.SystemCategory, error) {
	created, err := a.service.CreateCatalogCategory(ctx, timetable.CreateCategory{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	return systemCategory(&created), err
}

func systemActivity(group *timetable.Group) facilities.SystemActivity {
	if group == nil {
		return facilities.SystemActivity{}
	}
	return facilities.SystemActivity{
		ID: group.ID, Name: group.Name, MaxParticipants: group.MaxParticipants, IsOpen: group.IsOpen,
		CategoryID: group.CategoryID, PlannedRoomID: group.PlannedRoomID,
		IsSystem: group.IsSystem, CreatedBy: group.CreatedBy,
	}
}

func systemCategory(category *timetable.Category) facilities.SystemCategory {
	if category == nil {
		return facilities.SystemCategory{}
	}
	return facilities.SystemCategory{
		ID: category.ID, Name: category.Name, Description: category.Description,
		Color: category.Color, IsSystem: category.IsSystem,
	}
}
