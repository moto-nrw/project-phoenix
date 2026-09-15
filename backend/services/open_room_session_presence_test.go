package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runningSessionQuery struct {
	studentpresence.OpenRoomSessionQuery
	query        func(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	visits       func(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	supervisions func(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func (q runningSessionQuery) QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	if q.supervisions == nil {
		return nil, nil
	}
	return q.supervisions(ctx, filter)
}

func (q runningSessionQuery) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	return q.visits(ctx, filter)
}

func TestFacilitiesGroupSupervisionsKeepFutureEndDatesActive(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	future := "2099-12-31"
	past := "2000-01-01"
	groupIDs := []int64{42}
	query := facilitiesGroupSupervisions(runningSessionQuery{supervisions: func(got context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
		require.Same(t, ctx, got)
		require.Equal(t, groupIDs, filter.GroupIDs)
		require.NotNil(t, filter.ActiveOn)
		_, err := time.Parse("2006-01-02", *filter.ActiveOn)
		require.NoError(t, err)
		return []studentpresence.GroupSupervision{
			{ID: 91, GroupID: 42, StaffID: 5, EndDate: &future},
			{ID: 92, GroupID: 42, StaffID: 6, EndDate: &past},
			{ID: 93, GroupID: 42, StaffID: 7},
		}, nil
	}})

	rows, err := query(ctx, groupIDs)

	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.False(t, rows[0].Ended)
	assert.True(t, rows[1].Ended)
	assert.False(t, rows[2].Ended)
}

func TestFacilitiesGroupVisitsKeepOpenAndEndedRows(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ended := time.Now()
	for _, queryErr := range []error{nil, errors.New("query failed")} {
		query := facilitiesGroupVisits(runningSessionQuery{visits: func(got context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
			require.Same(t, ctx, got)
			require.Equal(t, studentpresence.VisitFilter{ActiveGroupIDs: []int64{42}}, filter)
			return []studentpresence.Visit{{ID: 1}, {ID: 2, ExitTime: &ended}}, queryErr
		}})
		rows, err := query(ctx, 42)
		if queryErr != nil {
			require.Error(t, err)
			require.Nil(t, rows)
			assert.Contains(t, err.Error(), "FindVisitsByActiveGroupID")
			assert.NotErrorIs(t, err, queryErr, "the retained boundary masks storage details")
			continue
		}
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Nil(t, rows[0].ExitTime)
		assert.Equal(t, &ended, rows[1].ExitTime)
	}
}

func (q runningSessionQuery) QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
	return q.query(ctx, filter)
}

func TestFacilitiesRoomSessionsPreserveFactsAndErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roomID := int64(42)
	started := time.Now().Add(-48 * time.Hour)
	failure := errors.New("presence query failed")
	for _, queryErr := range []error{nil, failure} {
		query := facilitiesRoomSessions(runningSessionQuery{query: func(got context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
			require.Same(t, ctx, got)
			require.Equal(t, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true}, filter)
			return []studentpresence.LiveGroup{{ID: 11, RoomID: roomID, StartTime: started}}, queryErr
		}})
		rows, err := query(ctx, roomID)
		if queryErr != nil {
			require.Nil(t, rows)
			require.ErrorIs(t, err, failure)
			assert.EqualError(t, err, "active: FindActiveGroupsByRoomID: find by room: presence query failed")
			continue
		}
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, int64(11), rows[0].ID)
		assert.Equal(t, started, rows[0].StartTime)
		assert.Nil(t, rows[0].EndTime)
		assert.False(t, rows[0].IsToday)
	}
}

func TestRunningSessionIDsPreserveSelectionAndErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ended := time.Now()
	failure := errors.New("presence query failed")
	for _, queryErr := range []error{nil, failure} {
		adapter := openRoomSessionPresence{presence: runningSessionQuery{query: func(got context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
			require.Same(t, ctx, got)
			require.Equal(t, studentpresence.LiveGroupFilter{}, filter)
			return []studentpresence.LiveGroup{{ID: 11}, {ID: 14, EndTime: &ended}, {ID: 12}, {ID: 13}}, queryErr
		}}}
		ids, err := adapter.ListRunningSessionIDs(ctx)
		if queryErr == nil {
			require.NoError(t, err)
			assert.Equal(t, []int64{11, 12, 13}, ids)
			continue
		}
		require.Nil(t, ids, "partial rows must not escape a failed query")
		require.ErrorIs(t, err, failure)
		assert.EqualError(t, err, "active: ListActiveGroups: list failed: presence query failed")
	}
}
