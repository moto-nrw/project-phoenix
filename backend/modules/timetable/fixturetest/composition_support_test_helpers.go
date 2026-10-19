//go:build testhooks

package fixturetest

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// Legacy composition support: the generic List(ctx, *QueryOptions) contract
// the schedule repository interfaces inherit, served over an owner listing.
// The calendar owner facades (#2666) answer typed filters; the translations
// below keep the query-options shape callers still build.

// UnfilteredListing serves List(ctx, options) for repositories whose real
// callers only ever list without options. Options carrying a filter,
// pagination or sorting are an explicit error instead of a silently ignored
// one.
type UnfilteredListing[T any] struct {
	Source func(context.Context) ([]T, error)
}

func (l UnfilteredListing[T]) List(ctx context.Context, options *QueryOptions) ([]T, error) {
	if options != nil && (options.Pagination != nil || options.Sorting != nil || filterHasConditions(options.Filter)) {
		return nil, &domain.StoreError{Op: "list", Err: errors.New("unsupported query options: the owner serves the unfiltered listing only")}
	}
	return l.Source(ctx)
}

func filterHasConditions(filter *ListFilter) bool {
	return filter != nil && (len(filter.Conditions()) > 0 || len(filter.OrFilters()) > 0 || len(filter.AndFilters()) > 0)
}

// DateframeListing is the option shape the schedules API and its tests build:
// a name equality or ILIKE pattern, pagination, and sorting.
type DateframeListing struct {
	Name        string
	NamePattern string
	Sort        []DateframeSortField
	Limit       int
	Offset      int
}

type DateframeSortField struct {
	Field      string
	Descending bool
}
