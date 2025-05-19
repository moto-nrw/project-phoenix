package base

import (
	"context"
	"time"

	"github.com/uptrace/bun"
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

// TableNamer is implemented by models to specify their database table name
type TableNamer interface {
	TableName() string
}

// BeforeAppender is implemented by models that need to execute logic before being appended to the database
type BeforeAppender interface {
	BeforeAppend() error
}

// AfterScanner is implemented by models that need to execute logic after being scanned from the database
type AfterScanner interface {
	AfterScan() error
}

// Paginatable provides a standard interface for pagination
type Paginatable interface {
	// Paginate applies pagination to a database query
	Paginate(query *bun.SelectQuery, page, pageSize int) *bun.SelectQuery
}

// DefaultPaginator implements the Paginatable interface with default behavior
type DefaultPaginator struct{}

// Paginate applies standard offset-based pagination to a query
func (p DefaultPaginator) Paginate(query *bun.SelectQuery, page, pageSize int) *bun.SelectQuery {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	return query.Limit(pageSize).Offset(offset)
}

// Service defines a generic service interface
type Service[T Entity] interface {
	// Get retrieves an entity by its ID
	Get(ctx context.Context, id interface{}) (T, error)

	// Create creates a new entity
	Create(ctx context.Context, entity T) error

	// Update updates an existing entity
	Update(ctx context.Context, entity T) error

	// Delete removes an entity
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
