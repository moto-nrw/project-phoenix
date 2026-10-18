package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// CatalogSource is the existing activity service, bound by the owner composition.
type CatalogSource interface {
	GetGroup(context.Context, int64) (*activities.Group, error)
	ListGroups(context.Context, *activities.GroupListQuery) ([]*activities.Group, error)
	ListCategories(context.Context) ([]*activities.Category, error)
	CreateCategory(context.Context, *activities.Category) (*activities.Category, error)
	CreateGroup(context.Context, *activities.Group, []int64, []*activities.Schedule) (*activities.Group, error)
}

type Catalog struct {
	Source CatalogSource
	timetable.OccupancyQuery
}

func (c Catalog) FindCatalogGroup(ctx context.Context, id int64) (*timetable.Group, error) {
	row, err := c.Source.GetGroup(ctx, id)
	if row == nil {
		return nil, err
	}
	group := CatalogGroup(row)
	return &group, err
}

func (c Catalog) ListCatalogGroups(ctx context.Context, name string) ([]timetable.Group, error) {
	rows, err := c.Source.ListGroups(ctx, &activities.GroupListQuery{Name: name})
	if err != nil {
		return nil, err
	}
	groups := make([]timetable.Group, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			groups = append(groups, CatalogGroup(row))
		}
	}
	return groups, nil
}

func (c Catalog) ListCatalogCategories(ctx context.Context) ([]timetable.Category, error) {
	rows, err := c.Source.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.Category, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, catalogCategory(row))
		}
	}
	return result, nil
}

func (c Catalog) CreateCatalogCategory(ctx context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	row, err := c.Source.CreateCategory(ctx, &activities.Category{Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem})
	if row == nil {
		return timetable.Category{}, err
	}
	return catalogCategory(row), err
}

func (c Catalog) CreateCatalogGroup(ctx context.Context, input timetable.GroupInput) (timetable.Group, error) {
	// System provisioning deliberately has no roster or schedules, just as before.
	row, err := c.Source.CreateGroup(ctx, catalogGroupInput(input), []int64{}, []*activities.Schedule{})
	if row == nil {
		return timetable.Group{}, err
	}
	return CatalogGroup(row), err
}

func CatalogGroup(row *activities.Group) timetable.Group {
	return timetable.Group{
		ID: row.ID, TenantID: row.GetTenantID(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, MaxParticipants: row.MaxParticipants, RequiredStaff: row.RequiredStaff, IsOpen: row.IsOpen,
		CategoryID: row.CategoryID, PlanningTrackID: row.PlanningTrackID, PlannedRoomID: row.PlannedRoomID, CreatedBy: row.CreatedBy,
		Type: row.Type, EducationGroupID: row.EducationGroupID, ListKind: row.ListKind, IsTemplate: row.IsTemplate, IsSystem: row.IsSystem,
		ArchivedAt: row.ArchivedAt, SeriesRootID: row.SeriesRootID, CalendarPeriodID: row.CalendarPeriodID,
		TargetGroupType: row.TargetGroupType, TargetGradeLevel: row.TargetGradeLevel, TargetSchoolClass: row.TargetSchoolClass,
		SourceCareOfferingIDs: row.SourceCareOfferingIDs, SourceGradeLevels: row.SourceGradeLevels, SourceSchoolClasses: row.SourceSchoolClasses,
		Notes: row.Notes, IncludeClosingDays: row.IncludeClosingDays, SeriesLastDay: activityDateString(row.SeriesLastDay), Category: catalogCategoryPointer(row.Category),
	}
}

func catalogCategoryPointer(row *activities.Category) *timetable.Category {
	if row == nil {
		return nil
	}
	category := catalogCategory(row)
	return &category
}

func catalogCategory(row *activities.Category) timetable.Category {
	return timetable.Category{ID: row.ID, TenantID: row.GetTenantID(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Description: row.Description, Color: row.Color, IsSystem: row.IsSystem, ShiftTypeID: row.ShiftTypeID, ArchivedAt: row.ArchivedAt}
}

func catalogGroupInput(input timetable.GroupInput) *activities.Group {
	return &activities.Group{
		Name: input.Name, MaxParticipants: input.MaxParticipants, RequiredStaff: input.RequiredStaff, IsOpen: input.IsOpen,
		CategoryID: input.CategoryID, PlanningTrackID: input.PlanningTrackID, PlannedRoomID: input.PlannedRoomID,
		CreatedBy: input.CreatedBy, Type: input.Type, EducationGroupID: input.EducationGroupID, ListKind: input.ListKind,
		IsTemplate: input.IsTemplate, IsSystem: input.IsSystem, ArchivedAt: input.ArchivedAt, SeriesRootID: input.SeriesRootID,
		CalendarPeriodID: input.CalendarPeriodID, TargetGroupType: input.TargetGroupType,
		TargetGradeLevel: input.TargetGradeLevel, TargetSchoolClass: input.TargetSchoolClass,
		SourceCareOfferingIDs: input.SourceCareOfferingIDs, SourceGradeLevels: input.SourceGradeLevels,
		SourceSchoolClasses: input.SourceSchoolClasses, Notes: input.Notes, IncludeClosingDays: input.IncludeClosingDays,
		SeriesLastDay: activityDateFromString(input.SeriesLastDay),
	}
}
