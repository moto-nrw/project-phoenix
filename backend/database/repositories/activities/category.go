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
		Where("LOWER(name) = LOWER(?)", name)

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
