// backend/database/repositories/activities/category.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
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
	return &CategoryRepository{
		Repository: base.NewRepository[*activities.Category](db, tableActivitiesCategories, "Category"),
		db:         db,
	}
}

// FindByName finds a category by its name
func (r *CategoryRepository) FindByName(ctx context.Context, name string) (*activities.Category, error) {
	category := new(activities.Category)
	err := r.db.NewSelect().
		Model(category).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat).
		Where("LOWER(name) = LOWER(?)", name).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(&categories).
		ModelTableExpr(tableExprActivitiesCategoriesAsCat).
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

// Create overrides the base Create method to handle validation
func (r *CategoryRepository) Create(ctx context.Context, category *activities.Category) error {
	if category == nil {
		return fmt.Errorf("category cannot be nil")
	}

	// Validate category
	if err := category.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, category)
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

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(category).
		Where("id = ?", category.ID).
		ModelTableExpr(tableActivitiesCategories)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(category).
			Where("id = ?", category.ID).
			ModelTableExpr(tableActivitiesCategories)
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *CategoryRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Category, error) {
	var categories []*activities.Category
	query := r.db.NewSelect().
		Model(&categories).
		ModelTableExpr(`activities.categories AS "category"`)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return categories, nil
}
