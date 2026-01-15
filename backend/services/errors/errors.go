// Package errors provides shared error definitions for service layer.
// Domain-specific errors should remain in their respective service packages.
// Only errors that are used across multiple services belong here.
package errors

import (
	"errors"
	"fmt"
)

// Shared service errors used across multiple domains.
// These are generic errors that don't belong to a specific domain.
var (
	// ErrInvalidData indicates the provided data failed validation.
	ErrInvalidData = errors.New("invalid data provided")

	// ErrDatabaseOperation indicates a database operation failed.
	ErrDatabaseOperation = errors.New("database operation failed")
)

// BatchOperationError wraps errors that occur during batch operations.
// Used by services that perform bulk operations (config, feedback, etc.).
type BatchOperationError struct {
	Errors []error
}

// Error returns the error message.
func (e *BatchOperationError) Error() string {
	return fmt.Sprintf("batch operation failed with %d errors", len(e.Errors))
}

// AddError appends an error to the batch.
func (e *BatchOperationError) AddError(err error) {
	e.Errors = append(e.Errors, err)
}

// HasErrors returns true if any errors were recorded.
func (e *BatchOperationError) HasErrors() bool {
	return len(e.Errors) > 0
}
