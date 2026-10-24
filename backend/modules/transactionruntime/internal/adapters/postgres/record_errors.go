package postgres

import (
	"database/sql"
	"errors"
)

// WrapRecordError wraps err of operation in the retained repository error
// shape, the DatabaseError the retained callers of other owners' records
// classify on; nil stays nil.
func WrapRecordError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &DatabaseError{Op: operation, Err: err}
}

// MissingRecordError is the retained not-found error of operation: both the
// repository not-found sentinel and the driver's no-rows sentinel.
func MissingRecordError(operation string) error {
	return WrapRecordError(operation, errors.Join(ErrNotFound, sql.ErrNoRows))
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

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request, for callers that may not name this type (#2736).
func (e *DatabaseError) StoreFailure() bool {
	return true
}
