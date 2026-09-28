package compose

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// supportListOptions is the retained repositories' list options, reached
// through this package's alias so the test needs no base model import.
type supportListOptions = RecurrenceRuleQueryOptions

// supportFresh returns a new zero value of the pointer's element type.
func supportFresh[T any](*T) *T { return new(T) }

// newSupportListOptions mirrors the base package's options constructor: an
// empty filter, no pagination, no sorting.
func newSupportListOptions() *supportListOptions {
	options := &supportListOptions{}
	options.Filter = supportFresh(options.Filter)
	return options
}

func TestUnfilteredListingRejectsOptionsTheOwnerCannotServe(t *testing.T) {
	t.Parallel()

	calls := 0
	listing := UnfilteredListing[*schedule.CalendarPeriod]{
		Source: func(context.Context) ([]*schedule.CalendarPeriod, error) {
			calls++
			return []*schedule.CalendarPeriod{{Name: "Schuljahr"}}, nil
		},
	}
	ctx := context.Background()

	rows, err := listing.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = listing.List(ctx, newSupportListOptions())
	require.NoError(t, err, "empty options are the unfiltered listing")
	require.Len(t, rows, 1)
	assert.Equal(t, 2, calls)

	sorting := newSupportListOptions()
	sorting.Filter = nil
	sorting.Sorting = supportFresh(sorting.Sorting)
	sorting.Sorting.AddField("name", "ASC")
	condition := newSupportListOptions()
	condition.Filter.Equal("name", "x")
	composite := newSupportListOptions()
	composite.Filter.Or(*newSupportListOptions().Filter.Equal("name", "x"))
	for name, options := range map[string]*supportListOptions{
		"pagination": newSupportListOptions().WithPagination(1, 10),
		"sorting":    sorting,
		"condition":  condition,
		"composite":  composite,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := listing.List(ctx, options)
			require.Error(t, err, "options the owner cannot serve are an explicit error, never silently ignored")
			assert.True(t, strings.HasPrefix(err.Error(), "database error during list: "), "want the list database error, got %v", err)
		})
	}
	assert.Equal(t, 2, calls, "a rejected listing never reaches the owner")
}

func TestDateframeOptionsListingTranslatesTheSchedulesAPIShape(t *testing.T) {
	t.Parallel()

	var seen DateframeListing
	listing := DateframeOptionsListing{
		Source: func(_ context.Context, request DateframeListing) ([]*schedule.Dateframe, error) {
			seen = request
			return []*schedule.Dateframe{}, nil
		},
	}
	ctx := context.Background()

	options := newSupportListOptions().WithPagination(2, 25)
	options.Filter.ILike("name", "%Ferien%")
	options.Sorting = supportFresh(options.Sorting)
	options.Sorting.AddField("start_date", "DESC")
	_, err := listing.List(ctx, options)
	require.NoError(t, err)
	assert.Equal(t, DateframeListing{
		NamePattern: "%Ferien%", Limit: 25, Offset: 25,
		Sort: []DateframeSortField{{Field: "start_date", Descending: true}},
	}, seen)

	exact := newSupportListOptions()
	exact.Filter.Equal("name", "Projektwoche")
	_, err = listing.List(ctx, exact)
	require.NoError(t, err)
	assert.Equal(t, DateframeListing{Name: "Projektwoche"}, seen)

	_, err = listing.List(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, DateframeListing{}, seen)

	unsupported := newSupportListOptions()
	unsupported.Filter.GreaterThan("start_date", "2030-01-01")
	_, err = listing.List(ctx, unsupported)
	require.Error(t, err, "filters outside the API shape are rejected instead of dropped")
	composite := newSupportListOptions()
	composite.Filter.Or(*newSupportListOptions().Filter.Equal("name", "x"))
	_, err = listing.List(ctx, composite)
	require.Error(t, err)
}
