package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type combinationGroupsStub struct {
	PresenceQueries
	find     func(context.Context, int64) (studentpresence.CombinedGroup, error)
	mappings func(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error)
	groups   func(context.Context, []int64) ([]studentpresence.LiveGroup, error)
}

func (s combinationGroupsStub) GetCombinedGroup(ctx context.Context, id int64) (studentpresence.CombinedGroup, error) {
	return s.find(ctx, id)
}

func (s combinationGroupsStub) ListGroupMappings(ctx context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
	return s.mappings(ctx, filter)
}

func (s combinationGroupsStub) ListLiveGroups(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	return s.groups(ctx, ids)
}

// TestCombinationGroupsPreserveMappingOrderAndSkipMissingSessions keeps the
// contract of the retired GetCombinedGroupWithGroups read: sessions follow the
// mapping order and mapped sessions that no longer exist are skipped.
func TestCombinationGroupsPreserveMappingOrderAndSkipMissingSessions(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rs := &Resource{Presence: combinationGroupsStub{
		find: func(got context.Context, id int64) (studentpresence.CombinedGroup, error) {
			assert.Same(t, ctx, got)
			assert.EqualValues(t, 9, id)
			return studentpresence.CombinedGroup{ID: 9}, nil
		},
		mappings: func(got context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
			assert.Same(t, ctx, got)
			require.NotNil(t, filter.CombinedGroupID)
			assert.EqualValues(t, 9, *filter.CombinedGroupID)
			return []studentpresence.GroupMapping{{ActiveGroupID: 2}, {ActiveGroupID: 1}, {ActiveGroupID: 3}}, nil
		},
		groups: func(got context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
			assert.Same(t, ctx, got)
			assert.Equal(t, []int64{2, 1, 3}, ids)
			return []studentpresence.LiveGroup{{ID: 1, RoomID: 11}, {ID: 2, RoomID: 22}}, nil
		},
	}}
	combination, groups, err := rs.presenceCombinationGroups(ctx, 9)
	require.NoError(t, err)
	assert.EqualValues(t, 9, combination.ID)
	require.Len(t, groups, 2)
	assert.EqualValues(t, 2, groups[0].ID)
	assert.EqualValues(t, 22, groups[0].RoomID)
	assert.EqualValues(t, 1, groups[1].ID)
	assert.EqualValues(t, 11, groups[1].RoomID)
}

func TestCombinationGroupsClassifyEveryFailureAsNotFound(t *testing.T) {
	t.Parallel()
	failure := errors.New("storage unavailable")
	for _, tc := range []struct {
		name                          string
		findErr, mappingErr, groupErr error
	}{
		{name: "missing combination", findErr: studentpresence.ErrCombinedGroupNotFound},
		{name: "combination lookup failure", findErr: failure},
		{name: "mapping failure", mappingErr: failure},
		{name: "session failure", groupErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &Resource{Presence: combinationGroupsStub{
				find: func(context.Context, int64) (studentpresence.CombinedGroup, error) {
					return studentpresence.CombinedGroup{ID: 9}, tc.findErr
				},
				mappings: func(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
					return []studentpresence.GroupMapping{{ActiveGroupID: 1}}, tc.mappingErr
				},
				groups: func(context.Context, []int64) ([]studentpresence.LiveGroup, error) {
					return []studentpresence.LiveGroup{{ID: 1}}, tc.groupErr
				},
			}}
			_, groups, err := rs.presenceCombinationGroups(context.Background(), 9)
			require.ErrorIs(t, err, studentpresence.ErrCombinedGroupNotFound)
			var presenceErr *presenceError
			require.ErrorAs(t, err, &presenceErr)
			assert.Equal(t, "GetCombinedGroupWithGroups", presenceErr.Op)
			assert.Nil(t, groups)
		})
	}
}
