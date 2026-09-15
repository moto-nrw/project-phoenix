package importpkg

import (
	"context"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// ImportConfig defines entity-specific import behavior
type ImportConfig[T any] interface {
	// PreloadReferenceData loads all reference data (groups, rooms) into memory cache
	PreloadReferenceData(ctx context.Context) error

	// Validate validates a single row of import data
	Validate(ctx context.Context, row *T) []importModels.ValidationError

	// FindExisting checks if entity already exists (for duplicate detection)
	FindExisting(ctx context.Context, row T) (*int64, error)

	// Create creates a new entity from import data
	Create(ctx context.Context, row T) (int64, error)

	// Update updates an existing entity
	Update(ctx context.Context, id int64, row T) error

	// EntityName returns the entity type name (for logging/errors)
	EntityName() string
}

// BatchValidator can be implemented by import configs that need validation
// across rows, such as duplicate checks within the uploaded file.
type BatchValidator[T any] interface {
	ValidateBatch(ctx context.Context, rows []T) map[int][]importModels.ValidationError
}

// ProcessingOrderer can be implemented by imports that must create rows in a
// deterministic order. The returned indexes always refer to the original
// upload order, so row numbers in validation errors remain accurate.
//
// It is used only for real imports; previews preserve upload order.
type ProcessingOrderer[T any] interface {
	ProcessingOrder(rows []T) []int
}
