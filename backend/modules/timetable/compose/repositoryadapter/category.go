// backend/database/repositories/activities/category.go
package repositoryadapter

import (
	"context"
	"fmt"
	"time"

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
	return r.findByName(ctx, name, false)
}

// FindByNameIncludingArchivedForShare finds and locks the preferred category
// for a name even when only archived rows exist. An active row wins over
// historical archived rows. Callers must keep the ambient transaction open
// through their referencing write so an archive cannot race that write.
func (r *CategoryRepository) FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*activities.Category, error) {
	return r.findByName(ctx, name, true)
}

func (r *CategoryRepository) findByName(ctx context.Context, name string, includeArchived bool) (*activities.Category, error) {
	category := new(activities.Category)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(category).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat).
		Where(`LOWER("category".name) = LOWER(?)`, name)
	if includeArchived {
		query = query.
			OrderExpr(`"category".archived_at ASC NULLS FIRST`).
			OrderExpr(`"category".updated_at DESC`).
			Limit(1).
			For("SHARE")
	} else {
		query = query.Where(`"category".archived_at IS NULL`)
	}

	query = base.WithTenantFilter(ctx, query, "category")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: base.TranslateNotFound(err),
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
		return nil, &modelBase.DatabaseError{Op: "find category by id for share", Err: base.TranslateNotFound(err)}
	}
	return category, nil
}

// UpdateIfActive conditionally updates a category while it is still active.
// This domain write cannot use the generic Update helper because the
// archived_at predicate must be part of the same statement as the editable
// field update; a separate read would leave a check-then-write race.
func (r *CategoryRepository) UpdateIfActive(ctx context.Context, category *activities.Category) (bool, error) {
	if category == nil {
		return false, fmt.Errorf("category cannot be nil")
	}
	if err := category.Validate(); err != nil {
		return false, err
	}

	category.UpdatedAt = time.Now()
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(category).
		Column("name", "description", "color", "updated_at").
		Where("id = ?", category.ID).
		Where("archived_at IS NULL").
		ModelTableExpr(tableActivitiesCategories)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active category", Err: base.TranslateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active category", Err: base.TranslateNotFound(err)}
	}
	return rows == 1, nil
}

// ListAll returns all categories. Empty, not nil, on no rows: the result is
// serialized straight to JSON, where a nil slice becomes null (#2419).
func (r *CategoryRepository) ListAll(ctx context.Context) ([]*activities.Category, error) {
	categories := make([]*activities.Category, 0)
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
			Err: base.TranslateNotFound(err),
		}
	}

	return categories, nil
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
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update category")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *CategoryRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Category, error) {
	return r.ListWithOptions(ctx, options)
}
