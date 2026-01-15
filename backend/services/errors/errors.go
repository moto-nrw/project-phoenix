// Package errors provides shared error definitions for service layer.
// Domain-specific errors should remain in their respective service packages.
// Only errors that are used across multiple services belong here.
package errors

import "errors"

// Shared service errors used across multiple domains.
// These are generic errors that don't belong to a specific domain.
var (
	// ErrInvalidData indicates the provided data failed validation.
	ErrInvalidData = errors.New("invalid data provided")

	// ErrDatabaseOperation indicates a database operation failed.
	ErrDatabaseOperation = errors.New("database operation failed")

	// ErrNotFound is a generic not found error for entities.
	ErrNotFound = errors.New("entity not found")

	// ErrConflict indicates a conflict with existing data.
	ErrConflict = errors.New("conflict with existing data")

	// ErrUnauthorized indicates the operation is not permitted.
	ErrUnauthorized = errors.New("operation not authorized")
)
