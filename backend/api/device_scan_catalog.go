package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func DeviceActivityCatalog(service timetableCompose.ActivityService) deviceActivityCatalogPort {
	if service == nil {
		return nil
	}
	return activityCatalog{activities: timetableCompose.NewCatalog(service)}
}

// deviceActivityCatalogPort is the scan consumer's catalog and directory seam.
type deviceActivityCatalogPort interface {
	Find(context.Context, int64) (*devicescan.Activity, error)
	ListByName(context.Context, string) ([]devicescan.Activity, error)
	ListCategories(context.Context) ([]devicescan.Category, error)
	CreateCategory(context.Context, devicescan.NewCategory) (devicescan.Category, error)
	CreateActivity(context.Context, devicescan.NewActivity) (devicescan.Activity, error)
	Activities(context.Context) ([]devicescan.DirectoryActivity, error)
}

// activityCatalog binds the activity provisioning of the special rooms to
// the retained activity service.
type activityCatalog struct{ activities timetable.Catalog }

func activityFromGroup(group *timetable.Group) *devicescan.Activity {
	if group == nil {
		return nil
	}
	return &devicescan.Activity{
		ID: group.ID, Name: group.Name, MaxParticipants: group.MaxParticipants,
		PlannedRoomID: group.PlannedRoomID, IsSystem: group.IsSystem, IsOpen: group.IsOpen,
	}
}

func (c activityCatalog) Find(ctx context.Context, id int64) (*devicescan.Activity, error) {
	group, err := c.activities.FindCatalogGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	return activityFromGroup(group), nil
}

func (c activityCatalog) ListByName(ctx context.Context, name string) ([]devicescan.Activity, error) {
	groups, err := c.activities.ListCatalogGroups(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.Activity, 0, len(groups))
	for _, group := range groups {
		if activity := activityFromGroup(&group); activity != nil {
			result = append(result, *activity)
		}
	}
	return result, nil
}

func (c activityCatalog) ListCategories(ctx context.Context) ([]devicescan.Category, error) {
	categories, err := c.activities.ListCatalogCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.Category, 0, len(categories))
	for _, category := range categories {
		result = append(result, devicescan.Category{ID: category.ID, Name: category.Name})
	}
	return result, nil
}

func (c activityCatalog) CreateCategory(ctx context.Context, input devicescan.NewCategory) (devicescan.Category, error) {
	created, err := c.activities.CreateCatalogCategory(ctx, timetable.CreateCategory{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	if err != nil {
		return devicescan.Category{}, err
	}
	return devicescan.Category{ID: created.ID, Name: created.Name}, nil
}

func (c activityCatalog) CreateActivity(ctx context.Context, input devicescan.NewActivity) (devicescan.Activity, error) {
	// A system activity carries no supervisors and no schedules; created_by
	// stays unset.
	created, err := c.activities.CreateCatalogGroup(ctx, timetable.GroupInput{
		Name: input.Name, MaxParticipants: input.MaxParticipants, IsOpen: input.IsOpen,
		CategoryID: input.CategoryID, PlannedRoomID: input.PlannedRoomID, IsSystem: input.IsSystem,
	})
	if err != nil {
		return devicescan.Activity{}, err
	}
	return *activityFromGroup(&created), nil
}

func (c activityCatalog) Activities(ctx context.Context) ([]devicescan.DirectoryActivity, error) {
	groups, err := c.activities.ListOccupiedGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.DirectoryActivity, 0, len(groups))
	for _, group := range groups {
		category := ""
		if group.Category != nil {
			category = group.Category.Name
		}
		result = append(result, devicescan.DirectoryActivity{
			ID: group.ID, Name: group.Name, Category: category, IsOccupied: group.IsOccupied,
		})
	}
	return result, nil
}
