// backend/database/repositories/activities/category.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesCategories          = "activities.categories"
	tableExprActivitiesCategoriesAsCat = `activities.categories AS "category"`
)

// CategoryRepository implements activities.CategoryRepository interface
type CategoryRepository struct {
	*base.Repository[*activities.Category]
	db *bun.DB
}

// NewCategoryRepository creates a new CategoryRepository
func NewCategoryRepository(db *bun.DB) activities.CategoryRepository {
	repo := base.NewRepository[*activities.Category](db, tableActivitiesCategories, "Category")
	repo.TenantScoped = true
	return &CategoryRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByName finds a category by its name
func (r *CategoryRepository) FindByName(ctx context.Context, name string) (*activities.Category, error) {
	category := new(activities.Category)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(category).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat).
		Where("LOWER(name) = LOWER(?)", name).
		Where("archived_at IS NULL")

	query = base.WithTenantFilter(ctx, query, "category")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return category, nil
}

// FindByIDForShare locks a category against concurrent archive updates until
// the current transaction finishes. Callers must use it inside a transaction
// when the lock must span a subsequent referencing write.
func (r *CategoryRepository) FindByIDForShare(ctx context.Context, id int64) (*activities.Category, error) {
	category := new(activities.Category)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(category).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat).
		Where(`"category".id = ?`, id).
		For("SHARE")

	query = base.WithTenantFilter(ctx, query, "category")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find category by id for share", Err: err}
	}
	return category, nil
}

// UpdateIfActive conditionally updates a category while it is still active.
// This domain write cannot use the generic Update helper because the
// archived_at predicate must be part of the same statement as the full-row
// update; a separate read would leave a check-then-write race.
func (r *CategoryRepository) UpdateIfActive(ctx context.Context, category *activities.Category) (bool, error) {
	if category == nil {
		return false, fmt.Errorf("category cannot be nil")
	}
	if err := category.Validate(); err != nil {
		return false, err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(category).
		Where("id = ?", category.ID).
		Where("archived_at IS NULL").
		ModelTableExpr(tableActivitiesCategories)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active category", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active category", Err: err}
	}
	return rows == 1, nil
}

// ListAll returns all categories
func (r *CategoryRepository) ListAll(ctx context.Context) ([]*activities.Category, error) {
	var categories []*activities.Category
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&categories).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat)

	query = base.WithTenantFilter(ctx, query, "category")

	err := query.
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list all",
			Err: err,
		}
	}

	return categories, nil
}

// SetShiftTypeForCategories syncs the Kategorie↔Schichtart mapping for one
// shift type: sets shift_type_id = shiftTypeID for categoryIDs and clears it on
// any category currently pointing at this shift type that is no longer listed.
// Both statements are tenant-scoped; run inside the ambient tenant transaction
// so the sync is atomic (#1837 follow-up).
func (r *CategoryRepository) SetShiftTypeForCategories(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("no tenant in context")
	}
	db := base.GetDB(ctx, r.db)

	// Deduplicate so an id repeated in the request cannot make the existence
	// check below undercount and falsely reject a valid list.
	distinctIDs := dedupeInt64(categoryIDs)

	// Validate that every submitted id is an existing category in THIS tenant
	// before touching anything. Otherwise an unknown or cross-tenant id would
	// pass through the clear step (which wipes every real mapping not in the
	// list) while the set step silently matches nothing — clearing all links
	// yet reporting success (#1837 review).
	if len(distinctIDs) > 0 {
		var found int
		if err := db.NewSelect().
			Table(tableActivitiesCategories).
			ColumnExpr("count(*)").
			Where("tenant_id = ?", tenantID).
			Where("id IN (?)", bun.List(distinctIDs)).
			Scan(ctx, &found); err != nil {
			return &modelBase.DatabaseError{Op: "validate category ids", Err: err}
		}
		if found != len(distinctIDs) {
			return activities.ErrUnknownCategoryIDs
		}
	}

	// Clear the mapping on categories that used to point at this shift type but
	// were de-selected. When categoryIDs is empty every link is cleared.
	clear := db.NewUpdate().
		Table(tableActivitiesCategories).
		Set("shift_type_id = NULL").
		Where("tenant_id = ?", tenantID).
		Where("shift_type_id = ?", shiftTypeID)
	if len(distinctIDs) > 0 {
		clear = clear.Where("id NOT IN (?)", bun.List(distinctIDs))
	}
	if _, err := clear.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear category shift type", Err: err}
	}

	if len(distinctIDs) == 0 {
		return nil
	}

	if _, err := db.NewUpdate().
		Table(tableActivitiesCategories).
		Set("shift_type_id = ?", shiftTypeID).
		Where("tenant_id = ?", tenantID).
		Where("id IN (?)", bun.List(distinctIDs)).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "set category shift type", Err: err}
	}

	return nil
}

// dedupeInt64 returns the input with duplicate ids removed, preserving order.
func dedupeInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return ids
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Update overrides the base Update method to handle validation
func (r *CategoryRepository) Update(ctx context.Context, category *activities.Category) error {
	if category == nil {
		return fmt.Errorf("category cannot be nil")
	}

	// Validate category
	if err := category.Validate(); err != nil {
		return err
	}

	// Get the query builder - GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(category).
		Where("id = ?", category.ID).
		ModelTableExpr(tableActivitiesCategories)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Execute the query
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update category")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *CategoryRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Category, error) {
	return r.ListWithOptions(ctx, options)
}
