package compose

import (
	"context"
	"errors"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// ClosingDayRangeListing serves the date-typed range finder of the legacy
// closing day contract over an owner listing keyed by YYYY-MM-DD.
type ClosingDayRangeListing struct {
	Source func(ctx context.Context, from, to string) ([]*schedule.ClosingDay, error)
}

func (l ClosingDayRangeListing) FindOverlappingRange(ctx context.Context, from, to schedule.Date) ([]*schedule.ClosingDay, error) {
	return l.Source(ctx, string(from), string(to))
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
