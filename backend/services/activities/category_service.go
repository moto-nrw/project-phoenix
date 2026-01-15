package activities

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ======== Category Methods ========

// CreateCategory creates a new activity category
func (s *Service) CreateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: "create category", Err: err}
	}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		return nil, &ActivityError{Op: "create category", Err: err}
	}

	return category, nil
}

// GetCategory retrieves a category by ID
func (s *Service) GetCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetCategory, Err: ErrCategoryNotFound}
		}
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetCategory, Err: ErrCategoryNotFound}
		}
		return nil, &ActivityError{Op: opGetCategory, Err: err}
	}

	return category, nil
}

// UpdateCategory updates an activity category
func (s *Service) UpdateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	if err := s.categoryRepo.Update(ctx, category); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	return category, nil
}

// DeleteCategory deletes a category
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	// Check if the category is in use by any group
	groupsWithCategory, err := s.groupRepo.FindByCategory(ctx, id)
	if err != nil {
		return &ActivityError{Op: "check category usage", Err: err}
	}

	if len(groupsWithCategory) > 0 {
		return &ActivityError{Op: "delete category", Err: errors.New("category is in use by one or more activity groups")}
	}

	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete category", Err: err}
	}

	return nil
}

// ListCategories lists all activity categories
func (s *Service) ListCategories(ctx context.Context) ([]*activities.Category, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "list categories", Err: err}
	}

	return categories, nil
}
