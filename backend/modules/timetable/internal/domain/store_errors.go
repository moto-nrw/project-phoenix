package domain

import "errors"

// StoreError is the error shape of the retained schedule repositories: the
// failed operation and its cause. It replaces models/base.DatabaseError for
// this owner (#4084) and keeps its message, its unwrapping and its store
// failure mark.
type StoreError struct {
	Op  string
	Err error
}

func (e *StoreError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *StoreError) Unwrap() error { return e.Err }

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request, for callers that classify by shape.
func (e *StoreError) StoreFailure() bool { return true }

// ErrRecordNotFound is the retained repositories' not-found result. Callers
// outside this owner recognise it, like every repository not-found sentinel,
// by its RepositoryNotFound marker.
var ErrRecordNotFound error = recordNotFound{}

type recordNotFound struct{}

func (recordNotFound) Error() string       { return "repository: not found" }
func (recordNotFound) RepositoryNotFound() {}

// IsRecordNotFound reports whether err carries a repository not-found
// sentinel, this owner's or another owner's, bare or wrapped.
func IsRecordNotFound(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
