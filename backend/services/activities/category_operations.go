package activities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
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
	counts, err := s.groupRepo.CountByCategory(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "category usage counts", Err: err}
	}
	return counts, nil
}

// UpdateCategory renames a category and updates its description/color.
// System categories (Schulhof, WC) are auto-provisioned infrastructure and
// stay untouchable; archived ones must be restored before they can be edited.
func (s *Service) UpdateCategory(ctx context.Context, id int64, input CategoryInput) (*activities.Category, error) {
	category, err := s.loadEditableCategory(ctx, id, opUpdateCategory)
	if err != nil {
		return nil, err
	}
	if category.IsArchived() {
		return nil, &ActivityError{Op: opUpdateCategory, Err: ErrCategoryArchived}
	}

	category.Name = input.Name
	category.Description = input.Description
	category.Color = input.Color

	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: opUpdateCategory, Err: err}
	}

	if err := s.categoryRepo.Update(ctx, category); err != nil {
		if base.IsUniqueViolation(err) {
			return nil, &ActivityError{Op: opUpdateCategory, Err: ErrCategoryNameExists}
		}
		return nil, &ActivityError{Op: opUpdateCategory, Err: err}
	}

	return category, nil
}

// ArchiveCategory retires a category. Nothing is deleted: existing Termine and
// Aktivitäten keep their category_id and stay valid, the category just stops
// being offered for new assignments. Archiving an already-archived category is
// a no-op so a double click cannot fail.
func (s *Service) ArchiveCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.loadEditableCategory(ctx, id, opArchiveCategory)
	if err != nil {
		return nil, err
	}
	if category.IsArchived() {
		return category, nil
	}

	now := time.Now()
	category.ArchivedAt = &now
	if err := s.setCategoryArchivedAt(ctx, category, opArchiveCategory); err != nil {
		category.ArchivedAt = nil
		return nil, err
	}

	return category, nil
}

// RestoreCategory brings an archived category back into the pickers. It fails
// when an active category has meanwhile taken the same name — the partial
// unique index on (tenant_id, name) WHERE archived_at IS NULL is what detects
// that, so the check cannot race a concurrent create.
func (s *Service) RestoreCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.loadEditableCategory(ctx, id, opRestoreCategory)
	if err != nil {
		return nil, err
	}
	if !category.IsArchived() {
		return category, nil
	}

	archivedAt := category.ArchivedAt
	category.ArchivedAt = nil
	if err := s.setCategoryArchivedAt(ctx, category, opRestoreCategory); err != nil {
		category.ArchivedAt = archivedAt
		return nil, err
	}

	return category, nil
}

// setCategoryArchivedAt persists just the archived_at column of category.
// Deliberately a partial write rather than a full entity Update: an
// archive/restore must not clobber a concurrent rename, and a restore has to
// surface the partial unique index violation on (tenant_id, name) as a name
// conflict rather than a generic failure.
func (s *Service) setCategoryArchivedAt(ctx context.Context, category *activities.Category, op string) error {
	updated, err := s.categoryRepo.UpdateColumns(ctx, category, "archived_at")
	if err != nil {
		if base.IsUniqueViolation(err) {
			return &ActivityError{Op: op, Err: ErrCategoryNameExists}
		}
		return &ActivityError{Op: op, Err: err}
	}
	if updated == 0 {
		return &ActivityError{Op: op, Err: ErrCategoryNotFound}
	}
	return nil
}

// loadEditableCategory fetches a tenant-scoped category and rejects the
// auto-provisioned system ones, which every write path must refuse.
func (s *Service) loadEditableCategory(ctx context.Context, id int64, op string) (*activities.Category, error) {
	category, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			return nil, &ActivityError{Op: op, Err: ErrCategoryNotFound}
		}
		return nil, &ActivityError{Op: op, Err: err}
	}
	if category.IsSystem {
		return nil, &ActivityError{Op: op, Err: ErrSystemCategoryProtected}
	}
	return category, nil
}
