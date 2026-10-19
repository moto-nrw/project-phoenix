//go:build testhooks

package fixturetest

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// DateframeOptionsListing serves the dateframe List(ctx, options) contract over the owner listing.
type DateframeOptionsListing struct {
	Source func(context.Context, DateframeListing) ([]*schedule.Dateframe, error)
}

func (l DateframeOptionsListing) List(ctx context.Context, options *QueryOptions) ([]*schedule.Dateframe, error) {
	listing, err := dateframeListingFromOptions(options)
	if err != nil {
		return nil, &domain.StoreError{Op: "list", Err: err}
	}
	return l.Source(ctx, listing)
}

func dateframeListingFromOptions(options *QueryOptions) (DateframeListing, error) {
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
			switch {
			case condition.Field != "name" || !ok:
				return listing, fmt.Errorf("unsupported dateframe filter %q %s", condition.Field, condition.Operator)
			case condition.Operator == ListOpEqual:
				listing.Name = value
			case condition.Operator == ListOpILike:
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
			listing.Sort = append(listing.Sort, DateframeSortField{Field: field.Field, Descending: field.Direction == ListSortDesc})
		}
	}
	return listing, nil
}
