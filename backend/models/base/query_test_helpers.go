package base

import "errors"

// The constructors and the not-found check below serve only the tests of
// the retained repositories since #4084 removed their last production
// callers.

// NewFilter creates a new filter
func NewFilter() *Filter {
	return &Filter{
		conditions: make([]FilterCondition, 0),
		or:         make([]Filter, 0),
		and:        make([]Filter, 0),
		tableAlias: "",
	}
}

// NewQueryOptions creates a new QueryOptions instance
func NewQueryOptions() *QueryOptions {
	return &QueryOptions{
		Filter: NewFilter(),
	}
}

// IsNoRows reports whether err wraps the persistence-neutral repository
// not-found sentinel.
func IsNoRows(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
