package schedule

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// CalendarPeriodOverlapListing serves the date-typed overlap finders of the
// legacy calendar period contract over an owner listing keyed by YYYY-MM-DD.
// An empty period type means every type.
type CalendarPeriodOverlapListing struct {
	Source func(ctx context.Context, periodType, start, end string, excludeID int64) ([]*schedule.CalendarPeriod, error)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlapping(ctx context.Context, start, end timezone.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, "", string(start), string(end), excludeID)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlappingByType(ctx context.Context, periodType string, start, end timezone.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, periodType, string(start), string(end), excludeID)
}

// ClosingDayRangeListing serves the date-typed range finder of the legacy
// closing day contract over an owner listing keyed by YYYY-MM-DD.
type ClosingDayRangeListing struct {
	Source func(ctx context.Context, from, to string) ([]*schedule.ClosingDay, error)
}

func (l ClosingDayRangeListing) FindOverlappingRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ClosingDay, error) {
	return l.Source(ctx, string(from), string(to))
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

func (l UnfilteredListing[T]) List(ctx context.Context, options *modelBase.QueryOptions) ([]T, error) {
	if options != nil && (options.Pagination != nil || options.Sorting != nil || filterHasConditions(options.Filter)) {
		return nil, &modelBase.DatabaseError{Op: "list", Err: errors.New("unsupported query options: the owner serves the unfiltered listing only")}
	}
	return l.Source(ctx)
}

func filterHasConditions(filter *modelBase.Filter) bool {
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

// DateframeOptionsListing serves the dateframe List(ctx, options) contract
// over the owner listing.
type DateframeOptionsListing struct {
	Source func(context.Context, DateframeListing) ([]*schedule.Dateframe, error)
}

func (l DateframeOptionsListing) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.Dateframe, error) {
	listing, err := dateframeListingFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}
	return l.Source(ctx, listing)
}

func dateframeListingFromOptions(options *modelBase.QueryOptions) (DateframeListing, error) {
	listing := DateframeListing{}
	if options == nil {
		return listing, nil
	}
	if options.Filter != nil {
		if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
			return listing, errors.New("unsupported dateframe filter: composite conditions")
		}
		for _, condition := range options.Filter.Conditions() {
			value, ok := condition.Value.(string)
			if condition.Field != "name" || !ok {
				return listing, fmt.Errorf("unsupported dateframe filter %q %s", condition.Field, condition.Operator)
			}
			switch condition.Operator {
			case modelBase.OpEqual:
				listing.Name = value
			case modelBase.OpILike:
				listing.NamePattern = value
			default:
				return listing, fmt.Errorf("unsupported dateframe filter %q %s", condition.Field, condition.Operator)
			}
		}
	}
	if options.Pagination != nil {
		listing.Limit = options.Pagination.PageSize
		listing.Offset = options.Pagination.Offset()
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			listing.Sort = append(listing.Sort, DateframeSortField{Field: field.Field, Descending: field.Direction == modelBase.SortDesc})
		}
	}
	return listing, nil
}
