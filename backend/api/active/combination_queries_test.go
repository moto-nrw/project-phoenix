package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresenceCombinationResponseActivity(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	assert.True(t, newPresenceCombinationResponse(studentpresence.CombinedGroup{}).IsActive,
		"nil end time is open-ended and active")
	assert.True(t, newPresenceCombinationResponse(studentpresence.CombinedGroup{EndTime: &future}).IsActive,
		"future end time is still active")
	assert.False(t, newPresenceCombinationResponse(studentpresence.CombinedGroup{EndTime: &past}).IsActive,
		"past end time is not active")
}

type combinationQueryStub struct {
	PresenceQueries
	query func(context.Context, studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, error)
	find  func(context.Context, int64) (studentpresence.CombinedGroup, error)
}

func (s combinationQueryStub) GetCombinedGroup(ctx context.Context, id int64) (studentpresence.CombinedGroup, error) {
	return s.find(ctx, id)
}

func TestCombinationLookupPreservesNotFoundEnvelope(t *testing.T) {
	t.Parallel()
	for _, lookupErr := range []error{studentpresence.ErrCombinedGroupNotFound, errors.New("database unavailable")} {
		ctx, cancel := context.WithCancel(context.Background())
		rs := &Resource{Presence: combinationQueryStub{find: func(got context.Context, id int64) (studentpresence.CombinedGroup, error) {
			assert.Same(t, ctx, got)
			assert.EqualValues(t, 42, id)
			return studentpresence.CombinedGroup{ID: 42}, lookupErr
		}}}
		row, err := rs.presenceCombination(ctx, 42)
		assert.Zero(t, row)
		require.ErrorIs(t, err, studentpresence.ErrCombinedGroupNotFound)
		var activeErr *presenceError
		require.ErrorAs(t, err, &activeErr)
		assert.Equal(t, "GetCombinedGroup", activeErr.Op)
		cancel()
	}
}

func (s combinationQueryStub) ListCombinedGroups(ctx context.Context, filter studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, error) {
	return s.query(ctx, filter)
}

func TestCombinationQueryPreservesFiltersAndDiscardsPartialFailures(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"ListCombinedGroups", "FindActiveCombinedGroups"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			active := false
			filter := studentpresence.CombinedGroupFilter{Active: &active, OpenOnly: operation == "FindActiveCombinedGroups"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rs := &Resource{Presence: combinationQueryStub{query: func(got context.Context, gotFilter studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, error) {
				assert.Same(t, ctx, got)
				assert.Equal(t, filter, gotFilter)
				return []studentpresence.CombinedGroup{{ID: 42}}, errors.New("query failed")
			}}}
			rows, err := rs.listPresenceCombinations(ctx, filter, operation)
			assert.Nil(t, rows)
			require.ErrorIs(t, err, studentpresence.ErrDatabaseOperation)
			var activeErr *presenceError
			require.ErrorAs(t, err, &activeErr)
			assert.Equal(t, operation, activeErr.Op)
		})
	}
}
