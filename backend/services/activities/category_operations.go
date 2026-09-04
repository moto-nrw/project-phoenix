package activities

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Operation names for the category Stammdaten flows (#2131).
const (
	opUpdateCategory  = "update category"
	opArchiveCategory = "archive category"
	opRestoreCategory = "restore category"
)

// CategoryInput carries the editable fields of a category. Description and
// Color are optional; an empty Color clears it back to the display default.
type CategoryInput struct {
	Name        string
	Description string
	Color       string
}

// CategoryUsageCounts reports how many activity groups (Aktivitäten and
// Termin-Vorlagen alike) reference each category of the current tenant, keyed
// by category id; unused categories are absent from the map. It drives the
// archive warning in the Stammdaten UI — a category in use must never be
// deleted, only archived. Kept separate from listing because the aggregate is
// an extra tenant-wide scan that only that one screen needs (#2131).
func (s *Service) CategoryUsageCounts(ctx context.Context) (map[int64]int, error) {
	if s.timetable == nil {
		return nil, &ActivityError{Op: "category usage counts", Err: errors.New("category capability is required")}
	}
	counts, err := s.timetable.CountCategoryUsage(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "category usage counts", Err: err}
	}
	return counts, nil
}

// UpdateCategory renames a category and updates its description/color.
// System categories (Schulhof, WC) are auto-provisioned infrastructure and
// stay untouchable; archived ones must be restored before they can be edited.
func (s *Service) UpdateCategory(ctx context.Context, id int64, input CategoryInput) (*activities.Category, error) {
	updated, err := s.timetable.UpdateCategory(ctx, timetable.UpdateCategory{
		ID: id, Name: input.Name, Description: input.Description, Color: input.Color,
	})
	if err != nil {
		return nil, categoryActivityError(opUpdateCategory, err)
	}
	return categoryFromOwner(updated), nil
}

// ArchiveCategory retires a category. Nothing is deleted: existing Termine and
// Aktivitäten keep their category_id and stay valid, the category just stops
// being offered for new assignments. Archiving an already-archived category is
// a no-op so a double click cannot fail.
func (s *Service) ArchiveCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.timetable.ArchiveCategory(ctx, id)
	if err != nil {
		return nil, categoryActivityError(opArchiveCategory, err)
	}
	return categoryFromOwner(category), nil
}

// RestoreCategory brings an archived category back into the pickers. It fails
// when an active category has meanwhile taken the same case-insensitive name —
// the partial unique index on (tenant_id, LOWER(name)) WHERE archived_at IS
// NULL detects that, so the check cannot race a concurrent create.
func (s *Service) RestoreCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.timetable.RestoreCategory(ctx, id)
	if err != nil {
		return nil, categoryActivityError(opRestoreCategory, err)
	}
	return categoryFromOwner(category), nil
}

func categoryFromOwner(category timetable.Category) *activities.Category {
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

func groupsFromOwner(groups []timetable.Group) []*activities.Group {
	result := make([]*activities.Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, groupFromOwner(group))
	}
	return result
}

func groupFromOwner(group timetable.Group) *activities.Group {
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
		result.Category = categoryFromOwner(*group.Category)
	}
	return result
}

func categoryActivityError(operation string, err error) *ActivityError {
	switch {
	case errors.Is(err, timetable.ErrCategoryNotFound):
		err = ErrCategoryNotFound
	case errors.Is(err, timetable.ErrSystemCategoryProtected):
		err = ErrSystemCategoryProtected
	case errors.Is(err, timetable.ErrSystemCategoryName):
		err = ErrSystemCategoryNameReserved
	case errors.Is(err, timetable.ErrCategoryNameExists):
		err = ErrCategoryNameExists
	case errors.Is(err, timetable.ErrCategoryArchived):
		err = ErrCategoryArchived
	case errors.Is(err, timetable.ErrUnknownCategoryIDs):
		err = activities.ErrUnknownCategoryIDs
	}
	return &ActivityError{Op: operation, Err: err}
}
