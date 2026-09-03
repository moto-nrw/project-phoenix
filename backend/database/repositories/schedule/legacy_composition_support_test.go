package schedule_test

import (
	"context"
	"errors"
	"testing"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnfilteredListingRejectsOptionsTheOwnerCannotServe(t *testing.T) {
	t.Parallel()

	calls := 0
	listing := scheduleRepo.UnfilteredListing[*schedule.CalendarPeriod]{
		Source: func(context.Context) ([]*schedule.CalendarPeriod, error) {
			calls++
			return []*schedule.CalendarPeriod{{Name: "Schuljahr"}}, nil
		},
	}
	ctx := context.Background()

	rows, err := listing.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = listing.List(ctx, modelBase.NewQueryOptions())
	require.NoError(t, err, "empty options are the unfiltered listing")
	require.Len(t, rows, 1)
	assert.Equal(t, 2, calls)

	for name, options := range map[string]*modelBase.QueryOptions{
		"pagination": modelBase.NewQueryOptions().WithPagination(1, 10),
		"sorting":    {Sorting: (&modelBase.Sorting{}).AddField("name", modelBase.SortAsc)},
		"condition":  {Filter: modelBase.NewFilter().Equal("name", "x")},
		"composite":  {Filter: modelBase.NewFilter().Or(*modelBase.NewFilter().Equal("name", "x"))},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := listing.List(ctx, options)
			require.Error(t, err, "options the owner cannot serve are an explicit error, never silently ignored")
			var databaseErr *modelBase.DatabaseError
			assert.True(t, errors.As(err, &databaseErr))
		})
	}
	assert.Equal(t, 2, calls, "a rejected listing never reaches the owner")
}

func TestDateframeOptionsListingTranslatesTheSchedulesAPIShape(t *testing.T) {
	t.Parallel()

	var seen scheduleRepo.DateframeListing
	listing := scheduleRepo.DateframeOptionsListing{
		Source: func(_ context.Context, request scheduleRepo.DateframeListing) ([]*schedule.Dateframe, error) {
			seen = request
			return []*schedule.Dateframe{}, nil
		},
	}
	ctx := context.Background()

	options := modelBase.NewQueryOptions().WithPagination(2, 25)
	options.Filter.ILike("name", "%Ferien%")
	options.Sorting = (&modelBase.Sorting{}).AddField("start_date", modelBase.SortDesc)
	_, err := listing.List(ctx, options)
	require.NoError(t, err)
	assert.Equal(t, scheduleRepo.DateframeListing{
		NamePattern: "%Ferien%", Limit: 25, Offset: 25,
		Sort: []scheduleRepo.DateframeSortField{{Field: "start_date", Descending: true}},
	}, seen)

	exact := modelBase.NewQueryOptions()
	exact.Filter.Equal("name", "Projektwoche")
	_, err = listing.List(ctx, exact)
	require.NoError(t, err)
	assert.Equal(t, scheduleRepo.DateframeListing{Name: "Projektwoche"}, seen)

	_, err = listing.List(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, scheduleRepo.DateframeListing{}, seen)

	unsupported := modelBase.NewQueryOptions()
	unsupported.Filter.GreaterThan("start_date", "2030-01-01")
	_, err = listing.List(ctx, unsupported)
	require.Error(t, err, "filters outside the API shape are rejected instead of dropped")
	composite := modelBase.NewQueryOptions()
	composite.Filter.Or(*modelBase.NewFilter().Equal("name", "x"))
	_, err = listing.List(ctx, composite)
	require.Error(t, err)
}
