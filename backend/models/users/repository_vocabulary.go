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
	RequestQueueFilters = base.RequestQueueFilters
)

// SortDesc is the descending direction of a QueryOptions sort field.
const SortDesc = base.SortDesc
