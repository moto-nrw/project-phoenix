package postgres

import (
	base "github.com/moto-nrw/project-phoenix/models/base"
)

// ErrNotFound is the persistence-neutral repository result for a missing row.
//
// It is a typed sentinel: packages that must not import this one (the HTTP
// layer, see api/common.IsNotFound) recognise it through the
// RepositoryNotFound marker method via errors.As, so the match follows the
// error chain like errors.Is does instead of comparing message text.
var ErrNotFound error = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found sentinel for packages that match it
// by shape rather than by identity.
func (notFoundError) RepositoryNotFound() {}

func IsUniqueViolation(err error) bool { return base.IsUniqueViolation(err) }
func IsUniqueViolationOn(err error, constraint string) bool {
	return base.IsUniqueViolationOn(err, constraint)
}
