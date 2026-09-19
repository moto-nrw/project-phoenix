package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionQueryStub struct {
	PresenceQueries
	query  func(context.Context, []int64) ([]studentpresence.LiveGroup, error)
	list   func(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	visits func(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

func (s sessionQueryStub) QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	return nil, nil
}

func (s sessionQueryStub) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	return s.visits(ctx, filter)
}

func TestSessionVisitsPreserveExistenceGateAndErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                     string
		groups                   []studentpresence.LiveGroup
		groupErr, visitErr, want error
	}{
		{name: "missing", want: studentpresence.ErrGroupNotFound},
		{name: "lookup failure", groupErr: errors.New("lookup failed"), want: studentpresence.ErrDatabaseOperation},
		{name: "visit failure", groups: []studentpresence.LiveGroup{{ID: 42}}, visitErr: errors.New("visits failed"), want: studentpresence.ErrDatabaseOperation},
		{name: "found", groups: []studentpresence.LiveGroup{{ID: 42}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queriedVisits := false
			rs := &Resource{Presence: sessionQueryStub{
				query: func(got context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
					assert.Same(t, ctx, got)
					assert.Equal(t, []int64{42}, ids)
					return tc.groups, tc.groupErr
				},
				visits: func(got context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
					queriedVisits = true
					assert.Same(t, ctx, got)
					assert.Equal(t, studentpresence.VisitFilter{ActiveGroupIDs: []int64{42}}, filter)
					return []studentpresence.Visit{{ID: 7}}, tc.visitErr
				},
			}}
			rows, err := rs.presenceSessionVisits(ctx, 42)
			assert.Equal(t, tc.groupErr == nil && len(tc.groups) > 0, queriedVisits)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				assert.Nil(t, rows)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []studentpresence.Visit{{ID: 7}}, rows)
		})
	}
}

func (s sessionQueryStub) QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
	if s.list != nil {
		return s.list(ctx, filter)
	}
	return nil, nil
}

func (s sessionQueryStub) ListLiveGroups(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	return s.query(ctx, ids)
}

func TestRoomSessionQueryPreservesFilterAndErrorContract(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roomID := int64(42)
	failure := errors.New("query failed")
	for _, queryErr := range []error{nil, failure} {
		rs := &Resource{Presence: sessionQueryStub{list: func(got context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
			assert.Same(t, ctx, got)
			assert.Equal(t, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true}, filter)
			return []studentpresence.LiveGroup{{ID: 7, RoomID: roomID}}, queryErr
		}}}
		rows, err := rs.presenceRoomSessions(ctx, roomID)
		if queryErr == nil {
			require.NoError(t, err)
			assert.Equal(t, []studentpresence.LiveGroup{{ID: 7, RoomID: roomID}}, rows)
			continue
		}
		require.Nil(t, rows)
		require.ErrorIs(t, err, failure)
		var activeErr *presenceError
		require.ErrorAs(t, err, &activeErr)
		assert.Equal(t, "FindActiveGroupsByRoomID", activeErr.Op)
		assert.EqualError(t, activeErr.Err, "find by room: query failed")
	}
}

func TestActivitySessionQueryPreservesFilterAndErrorContract(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, queryErr := range []error{nil, errors.New("query failed")} {
		rs := &Resource{Presence: sessionQueryStub{list: func(got context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
			assert.Same(t, ctx, got)
			assert.Equal(t, studentpresence.LiveGroupFilter{ActivityGroupIDs: []int64{42}, OpenOnly: true}, filter)
			return []studentpresence.LiveGroup{{ID: 7}}, queryErr
		}}}
		rows, err := rs.presenceActivitySessions(ctx, 42)
		if queryErr == nil {
			require.NoError(t, err)
			assert.Equal(t, []studentpresence.LiveGroup{{ID: 7}}, rows)
			continue
		}
		require.Nil(t, rows)
		require.ErrorIs(t, err, studentpresence.ErrDatabaseOperation)
		var activeErr *presenceError
		require.ErrorAs(t, err, &activeErr)
		assert.Equal(t, "FindActiveGroupsByGroupID", activeErr.Op)
	}
}

func TestSessionQueryPreservesLookupErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		rows []studentpresence.LiveGroup
		err  error
		want error
	}{
		{name: "missing", want: studentpresence.ErrGroupNotFound},
		{name: "partial failure", rows: []studentpresence.LiveGroup{{ID: 42}}, err: errors.New("query failed"), want: studentpresence.ErrDatabaseOperation},
		{name: "found", rows: []studentpresence.LiveGroup{{ID: 42, RoomID: 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rs := &Resource{Presence: sessionQueryStub{query: func(got context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
				assert.Same(t, ctx, got)
				assert.Equal(t, []int64{42}, ids)
				return tc.rows, tc.err
			}}}
			row, err := rs.presenceLiveGroup(ctx, 42)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				assert.Nil(t, row)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, row)
			assert.Equal(t, tc.rows[0], *row)
		})
	}
}
