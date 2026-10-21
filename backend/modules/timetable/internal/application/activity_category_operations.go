package application

import (
	"context"
	"errors"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Operation names for the category Stammdaten flows (#2131).
const (
	opUpdateCategory  = "update category"
	opArchiveCategory = "archive category"
	opRestoreCategory = "restore category"
)

// activityContract.CategoryInput carries the editable fields of a category. Description and
// Color are optional; an empty Color clears it back to the display default.

// CategoryUsageCounts reports how many activity groups (Aktivitäten and
// Termin-Vorlagen alike) reference each category of the current tenant, keyed
// by category id; unused categories are absent from the map. It drives the
// archive warning in the Stammdaten UI — a category in use must never be
// deleted, only archived. Kept separate from listing because the aggregate is
// an extra tenant-wide scan that only that one screen needs (#2131).
func (s *ActivityService) CategoryUsageCounts(ctx context.Context) (map[int64]int, error) {
	if s.timetable == nil {
		return nil, &activityContract.ActivityError{Op: "category usage counts", Err: errors.New("category capability is required")}
	}
	counts, err := s.timetable.CountCategoryUsage(ctx)
	if err != nil {
		return nil, &activityContract.ActivityError{Op: "category usage counts", Err: err}
	}
	return counts, nil
}

// UpdateCategory renames a category and updates its description/color.
// System categories (Schulhof, WC) are auto-provisioned infrastructure and
// stay untouchable; archived ones must be restored before they can be edited.
func (s *ActivityService) UpdateCategory(ctx context.Context, id int64, input activityContract.CategoryInput) (*activityContract.Category, error) {
	updated, err := s.timetable.UpdateCategory(ctx, timetable.UpdateCategory{
		ID: id, Name: input.Name, Description: input.Description, Color: input.Color,
	})
	if err != nil {
		return nil, categoryActivityError(opUpdateCategory, err)
	}
	return LegacyServiceCategory(updated), nil
}

// ArchiveCategory retires a category. Nothing is deleted: existing Termine and
// Aktivitäten keep their category_id and stay valid, the category just stops
// being offered for new assignments. Archiving an already-archived category is
// a no-op so a double click cannot fail.
func (s *ActivityService) ArchiveCategory(ctx context.Context, id int64) (*activityContract.Category, error) {
	category, err := s.timetable.ArchiveCategory(ctx, id)
	if err != nil {
		return nil, categoryActivityError(opArchiveCategory, err)
	}
	return LegacyServiceCategory(category), nil
}

// RestoreCategory brings an archived category back into the pickers. It fails
// when an active category has meanwhile taken the same case-insensitive name —
// the partial unique index on (tenant_id, LOWER(name)) WHERE archived_at IS
// NULL detects that, so the check cannot race a concurrent create.
func (s *ActivityService) RestoreCategory(ctx context.Context, id int64) (*activityContract.Category, error) {
	category, err := s.timetable.RestoreCategory(ctx, id)
	if err != nil {
		return nil, categoryActivityError(opRestoreCategory, err)
	}
	return LegacyServiceCategory(category), nil
}

func categoryActivityError(operation string, err error) *activityContract.ActivityError {
	switch {
	case errors.Is(err, timetable.ErrCategoryNotFound):
		err = activityContract.ErrCategoryNotFound
	case errors.Is(err, timetable.ErrSystemCategoryProtected):
		err = activityContract.ErrSystemCategoryProtected
	case errors.Is(err, timetable.ErrSystemCategoryName):
		err = activityContract.ErrSystemCategoryNameReserved
	case errors.Is(err, timetable.ErrCategoryNameExists):
		err = activityContract.ErrCategoryNameExists
	case errors.Is(err, timetable.ErrCategoryArchived):
		err = activityContract.ErrCategoryArchived
	case errors.Is(err, timetable.ErrUnknownCategoryIDs):
		err = unknownCategoryIDsError{}
	}
	return &activityContract.ActivityError{Op: operation, Err: err}
}

// unknownCategoryIDsError is a rejected Kategorie↔Schichtart mapping (#1837):
// it reads as the retained error and matches both sentinels, because the
// Dienstplan's shift-type administration classifies on the Timetable owner's
// (#3424).
type unknownCategoryIDsError struct{}

func (unknownCategoryIDsError) Error() string { return activityContract.ErrUnknownCategoryIDs.Error() }

func (unknownCategoryIDsError) Unwrap() []error {
	return []error{activityContract.ErrUnknownCategoryIDs, timetable.ErrUnknownCategoryIDs}
}
