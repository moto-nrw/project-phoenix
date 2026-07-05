package base

import (
	"context"
	"time"
)

// Entity represents the basic interface for all model entities
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
type Repository[T Entity] interface {
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
