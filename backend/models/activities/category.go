package activities

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Category represents an activity category
type Category struct {
	base.Model
	Name        string `bun:"name,notnull,unique" json:"name"`
	Description string `bun:"description" json:"description,omitempty"`
	Color       string `bun:"color" json:"color,omitempty"`

	// Relations
	Groups []*Group `bun:"rel:has-many,join:id=category_id" json:"groups,omitempty"`
}

// TableName returns the table name for the Category model
func (c *Category) TableName() string {
	return "activities.categories"
}

// GetID returns the category ID
func (c *Category) GetID() interface{} {
	return c.ID
}

// GetCreatedAt returns the creation timestamp
func (c *Category) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (c *Category) GetUpdatedAt() time.Time {
	return c.UpdatedAt
}

// Validate validates the category fields
func (c *Category) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("category name is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (c *Category) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := c.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.Color = strings.TrimSpace(c.Color)

	return nil
}

// CategoryRepository defines operations for working with activity categories
type CategoryRepository interface {
	base.Repository[*Category]
	FindByName(ctx context.Context, name string) (*Category, error)
	FindByColor(ctx context.Context, color string) ([]*Category, error)
}

// DefaultCategoryRepository is the default implementation of CategoryRepository
type DefaultCategoryRepository struct {
	db *bun.DB
}

// NewCategoryRepository creates a new category repository
func NewCategoryRepository(db *bun.DB) CategoryRepository {
	return &DefaultCategoryRepository{db: db}
}

// Create inserts a new category into the database
func (r *DefaultCategoryRepository) Create(ctx context.Context, category *Category) error {
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
func (r *DefaultCategoryRepository) FindByID(ctx context.Context, id interface{}) (*Category, error) {
	category := new(Category)
	err := r.db.NewSelect().Model(category).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return category, nil
}

// FindByName retrieves a category by its name
func (r *DefaultCategoryRepository) FindByName(ctx context.Context, name string) (*Category, error) {
	category := new(Category)
	err := r.db.NewSelect().Model(category).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return category, nil
}

// FindByColor retrieves categories by color
func (r *DefaultCategoryRepository) FindByColor(ctx context.Context, color string) ([]*Category, error) {
	var categories []*Category
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
func (r *DefaultCategoryRepository) Update(ctx context.Context, category *Category) error {
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
func (r *DefaultCategoryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Category)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves categories matching the filters
func (r *DefaultCategoryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Category, error) {
	var categories []*Category
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
