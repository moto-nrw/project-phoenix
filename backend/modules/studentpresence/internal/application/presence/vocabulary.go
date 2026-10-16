package presence

import (
	"errors"
	"fmt"
)

// The repository error contract the retained service speaks. Its own tests
// build these values without reaching past the package (#3214). Stores of
// other owners report a missing row with their own sentinel; IsNoRows
// recognises every one of them by the shared RepositoryNotFound marker.

// DatabaseError is a failed store operation: the store's failure, not a
// refusal of the request.
type DatabaseError struct {
	Op  string // operation that failed
	Err error  // underlying error
}

func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return fmt.Sprintf("database error during %s: %v", e.Op, e.Err)
}

func (e *DatabaseError) Unwrap() error { return e.Err }

// StoreFailure marks the error as the store's failure for callers that may
// not name this type.
func (e *DatabaseError) StoreFailure() bool { return true }

// ErrNotFound is the not-found result this package returns itself, for a row
// the caller may not see. It carries the RepositoryNotFound marker the HTTP
// layer maps to 404.
var ErrNotFound error = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string       { return "repository: not found" }
func (notFoundError) RepositoryNotFound() {}

// IsNoRows reports whether err carries a repository not-found result.
func IsNoRows(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
