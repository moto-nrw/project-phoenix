package base

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// This file provides an example implementation of how to use the new filters system
// with the Repository interface. This can serve as a template for implementing
// repositories for specific entities.

// ExampleEntity is a sample entity implementing the Entity interface
type ExampleEntity struct {
	ID        int64     `bun:"id,pk,autoincrement"`
	Name      string    `bun:"name,notnull"`
	Email     string    `bun:"email,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

// GetID implements the Entity interface
func (e *ExampleEntity) GetID() interface{} {
	return e.ID
}

// GetCreatedAt implements the Entity interface
func (e *ExampleEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (e *ExampleEntity) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// Validate implements the Validator interface
func (e *ExampleEntity) Validate() error {
	if e.Name == "" {
		return errors.New("name is required")
	}
	if e.Email == "" {
		return errors.New("email is required")
	}
	return nil
}

// TableName implements the TableNamer interface
func (e *ExampleEntity) TableName() string {
	return "examples"
}

// ExampleRepository is a sample repository implementing the Repository interface
// Note: This is for illustration purposes only and doesn't completely implement
// the generic Repository interface as it uses concrete types instead of generics
type ExampleRepository struct {
	db *bun.DB
}

// NewExampleRepository creates a new example repository
func NewExampleRepository(db *bun.DB) *ExampleRepository {
	return &ExampleRepository{db: db}
}

// Create inserts a new entity into the database
func (r *ExampleRepository) Create(ctx context.Context, entity *ExampleEntity) error {
	// Validate the entity first
	if validator, ok := interface{}(entity).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validation error: %w", err)
		}
	}

	// Set timestamps
	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	// Insert into database
	_, err := r.db.NewInsert().Model(entity).Exec(ctx)
	if err != nil {
		return &DatabaseError{
			Op:  "create",
			Err: fmt.Errorf("error creating entity: %w", err),
		}
	}

	return nil
}

// FindByID retrieves an entity by its ID
func (r *ExampleRepository) FindByID(ctx context.Context, id interface{}) (*ExampleEntity, error) {
	entity := new(ExampleEntity)
	err := r.db.NewSelect().Model(entity).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &DatabaseError{
			Op:  "findById",
			Err: fmt.Errorf("error finding entity by ID: %w", err),
		}
	}

	return entity, nil
}

// Update updates an existing entity in the database
func (r *ExampleRepository) Update(ctx context.Context, entity *ExampleEntity) error {
	// Validate the entity first
	if validator, ok := interface{}(entity).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validation error: %w", err)
		}
	}

	// Update timestamp
	entity.UpdatedAt = time.Now()

	// Update in database
	_, err := r.db.NewUpdate().Model(entity).WherePK().Exec(ctx)
	if err != nil {
		return &DatabaseError{
			Op:  "update",
			Err: fmt.Errorf("error updating entity: %w", err),
		}
	}

	return nil
}

// Delete removes an entity from the database
func (r *ExampleRepository) Delete(ctx context.Context, id interface{}) error {
	entity := new(ExampleEntity)
	_, err := r.db.NewDelete().Model(entity).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &DatabaseError{
			Op:  "delete",
			Err: fmt.Errorf("error deleting entity: %w", err),
		}
	}

	return nil
}

// List retrieves all entities matching the provided filters
func (r *ExampleRepository) List(ctx context.Context, options *QueryOptions) ([]*ExampleEntity, error) {
	var entities []*ExampleEntity
	query := r.db.NewSelect().Model(&entities)

	// Apply query options (filters, pagination, sorting)
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &DatabaseError{
			Op:  "list",
			Err: fmt.Errorf("error listing entities: %w", err),
		}
	}

	return entities, nil
}

// Count returns the total count of entities matching the provided filters
func (r *ExampleRepository) Count(ctx context.Context, filter *Filter) (int, error) {
	query := r.db.NewSelect().Model((*ExampleEntity)(nil))

	// Apply filter if provided
	if filter != nil {
		query = filter.ApplyToQuery(query)
	}

	return CountFromQuery(ctx, r.db, query)
}

// Usage examples:

/*
Example of how to use the new filters system:

// Creating a simple filter
filter := base.NewFilter().
    Equal("status", "active").
    GreaterThan("created_at", time.Now().AddDate(0, -1, 0))

// Adding pagination
pagination := base.NewPagination(1, 20)

// Adding sorting
sorting := base.NewSorting(
    base.SortField{Field: "created_at", Direction: base.SortDesc},
)

// Creating query options with all components
options := base.NewQueryOptions().
    WithPagination(1, 20).
    WithSorting(sorting)
options.Filter = filter

// Using the repository
repo := NewExampleRepository(db)
entities, err := repo.List(ctx, options)

// Counting with filters
count, err := repo.Count(ctx, filter)
*/
