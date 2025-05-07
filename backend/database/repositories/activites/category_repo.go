package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CategoryRepository implements activities.CategoryRepository
type CategoryRepository struct {
	db *bun.DB
}

// NewCategoryRepository creates a new category repository
func NewCategoryRepository(db *bun.DB) activities.CategoryRepository {
	return &CategoryRepository{db: db}
}

// Create inserts a new category into the database
func (r *CategoryRepository) Create(ctx context.Context, category *activities.Category) error {
	if err := category.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(category).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a category by its ID
func (r *CategoryRepository) FindByID(ctx context.Context, id interface{}) (*activities.Category, error) {
	category := new(activities.Category)
	err := r.db.NewSelect().Model(category).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return category, nil
}

// FindByName retrieves a category by its name
func (r *CategoryRepository) FindByName(ctx context.Context, name string) (*activities.Category, error) {
	category := new(activities.Category)
	err := r.db.NewSelect().Model(category).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return category, nil
}

// FindByColor retrieves categories by color
func (r *CategoryRepository) FindByColor(ctx context.Context, color string) ([]*activities.Category, error) {
	var categories []*activities.Category
	err := r.db.NewSelect().
		Model(&categories).
		Where("color = ?", color).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_color", Err: err}
	}
	return categories, nil
}

// Update updates an existing category
func (r *CategoryRepository) Update(ctx context.Context, category *activities.Category) error {
	if err := category.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(category).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a category
func (r *CategoryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*activities.Category)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves categories matching the filters
func (r *CategoryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*activities.Category, error) {
	var categories []*activities.Category
	query := r.db.NewSelect().Model(&categories)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return categories, nil
}
