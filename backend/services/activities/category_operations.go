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

// CategoryUsage pairs a category with the number of activity groups
// (Aktivitäten and Termin-Vorlagen alike) that reference it. The count drives
// the archive warning in the Stammdaten UI — a category in use must never be
// deleted, only archived.
type CategoryUsage struct {
	Category   *activities.Category
	UsageCount int
}

// CategoryInput carries the editable fields of a category. Description and
// Color are optional; an empty Color clears it back to the display default.
type CategoryInput struct {
	Name        string
	Description string
	Color       string
}

// ListCategoriesWithUsage returns every category of the current tenant
// together with how many activity groups reference it. Archived categories are
// included — the management UI shows them in an "Archiviert" section so they
// can be restored.
func (s *Service) ListCategoriesWithUsage(ctx context.Context) ([]CategoryUsage, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "list categories with usage", Err: err}
	}

	counts, err := s.groupRepo.CountByCategory(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "list categories with usage", Err: err}
	}

	result := make([]CategoryUsage, 0, len(categories))
	for _, category := range categories {
		result = append(result, CategoryUsage{
			Category:   category,
			UsageCount: counts[category.ID],
		})
	}

	return result, nil
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
	if err := s.categoryRepo.SetArchived(ctx, category.ID, &now); err != nil {
		return nil, &ActivityError{Op: opArchiveCategory, Err: err}
	}
	category.ArchivedAt = &now

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

	if err := s.categoryRepo.SetArchived(ctx, category.ID, nil); err != nil {
		if base.IsUniqueViolation(err) {
			return nil, &ActivityError{Op: opRestoreCategory, Err: ErrCategoryNameExists}
		}
		return nil, &ActivityError{Op: opRestoreCategory, Err: err}
	}
	category.ArchivedAt = nil

	return category, nil
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
