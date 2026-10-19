package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CalendarPeriodOverlapListing serves the date-typed overlap finders of the
// legacy calendar period contract over an owner listing keyed by YYYY-MM-DD.
// An empty period type means every type.
type CalendarPeriodOverlapListing struct {
	Source func(ctx context.Context, periodType, start, end string, excludeID int64) ([]*schedule.CalendarPeriod, error)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlapping(ctx context.Context, start, end calendar.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, "", string(start), string(end), excludeID)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlappingByType(ctx context.Context, periodType string, start, end calendar.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, periodType, string(start), string(end), excludeID)
}

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

func (l UnfilteredListing[T]) List(ctx context.Context, options *domain.ListOptions) ([]T, error) {
	if options != nil && (options.Pagination != nil || options.Sorting != nil || filterHasConditions(options.Filter)) {
		return nil, &domain.StoreError{Op: "list", Err: errors.New("unsupported query options: the owner serves the unfiltered listing only")}
	}
	return l.Source(ctx)
}

func filterHasConditions(filter *domain.ListFilter) bool {
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
