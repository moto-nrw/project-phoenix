package base

import (
	"context"
	"time"
)

// Entity represents models with the conventional identity and timestamps.
// Audit models with domain-specific timestamps deliberately do not implement it.
type Entity interface {
	// GetID returns the entity's ID
	GetID() interface{}

	// GetCreatedAt returns the creation timestamp
	GetCreatedAt() time.Time

	// GetUpdatedAt returns the last update timestamp
	GetUpdatedAt() time.Time
}

// Validator represents entities that can validate themselves
type Validator interface {
	// Validate validates the entity and returns an error if validation fails
	Validate() error
}

// Repository represents a generic repository interface for database operations
type Repository[T any] interface {
	// Create inserts a new entity into the database
	Create(ctx context.Context, entity T) error

	// FindByID retrieves an entity by its ID
	FindByID(ctx context.Context, id interface{}) (T, error)

	// Update updates an existing entity in the database
	Update(ctx context.Context, entity T) error

	// Delete removes an entity from the database
	Delete(ctx context.Context, id interface{}) error

	// List retrieves all entities matching the provided filters
	List(ctx context.Context, options *QueryOptions) ([]T, error)
}

// CRUDRepository is the generic 5-method CRUD contract implemented by the
// concrete database/repositories/base.Repository[T]. Repository interfaces
// embed it instead of re-declaring the block. Its List takes plain equality
// filters, matching the concrete generic repository.
type CRUDRepository[T any] interface {
	// Create inserts a new entity into the database
	Create(ctx context.Context, entity T) error

	// FindByID retrieves an entity by its ID
	FindByID(ctx context.Context, id any) (T, error)

	// Update updates an existing entity in the database
	Update(ctx context.Context, entity T) error

	// Delete removes an entity from the database
	Delete(ctx context.Context, id any) error

	// List retrieves all entities matching the provided equality filters
	List(ctx context.Context, filters map[string]any) ([]T, error)
}

// DatabaseError represents database operation errors
type DatabaseError struct {
	Op  string // Operation that failed (e.g., "create", "update")
	Err error  // Original error
}

// Error returns the error message
func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

// Unwrap returns the original error
func (e *DatabaseError) Unwrap() error {
	return e.Err
}
