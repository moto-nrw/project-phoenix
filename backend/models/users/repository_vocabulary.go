package users

import "github.com/moto-nrw/project-phoenix/models/base"

// The retained repository contracts take the generic query shapes. The
// compatibility adapters that serve those contracts over an owner capability
// name the shapes through this model boundary instead of the generic
// repository package. Every entry goes with the last retained contract that
// uses it.
type (
	QueryOptions        = base.QueryOptions
	QueryFilter         = base.Filter
	QuerySorting        = base.Sorting
	QuerySortField      = base.SortField
	RequestQueueFilters = base.RequestQueueFilters
)

// SortAsc and SortDesc are the directions of a QueryOptions sort field.
const (
	SortAsc  = base.SortAsc
	SortDesc = base.SortDesc
)

// The retained People Directory repositories (database/repositories/users)
// and the legacy composition that adapts them report failures in the shared
// repository error shape: callers classify results with errors.As on
// DatabaseError, the RepositoryNotFound marker, or the unique-violation
// checks. The repositories name that shape through this model boundary
// instead of the generic model package (#2727).
type DatabaseError = base.DatabaseError

var (
	// ErrRepositoryNotFound is the persistence-neutral not-found sentinel.
	ErrRepositoryNotFound = base.ErrNotFound

	NewQueryOptions     = base.NewQueryOptions
	NewQueryFilter      = base.NewFilter
	IsRepositoryNoRows  = base.IsNoRows
	IsUniqueViolation   = base.IsUniqueViolation
	IsUniqueViolationOn = base.IsUniqueViolationOn
)
