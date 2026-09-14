package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type combinationDetailsSource struct {
	StudentPresence
	groups func(context.Context, []int64) ([]studentpresence.LiveGroup, error)
}

func (s combinationDetailsSource) GetCombinedGroup(context.Context, int64) (studentpresence.CombinedGroup, error) {
	return studentpresence.CombinedGroup{ID: 9}, nil
}

func (s combinationDetailsSource) ListGroupMappings(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
	return []studentpresence.GroupMapping{{ActiveGroupID: 2}, {ActiveGroupID: 1}, {ActiveGroupID: 3}}, nil
}

func (s combinationDetailsSource) ListLiveGroups(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	return s.groups(ctx, ids)
}

func TestCombinationDetailsPreserveMappingOrderAndMissingSessions(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: combinationDetailsSource{
		groups: func(got context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
			assert.Same(t, ctx, got)
			assert.Equal(t, []int64{2, 1, 3}, ids)
			return []studentpresence.LiveGroup{{ID: 1, RoomID: 11}, {ID: 2, RoomID: 22}}, nil
		},
	}}}
	result, err := svc.GetCombinedGroupWithGroups(ctx, 9)
	require.NoError(t, err)
	require.Len(t, result.GroupMappings, 3)
	require.Len(t, result.ActiveGroups, 2)
	assert.EqualValues(t, 2, result.ActiveGroups[0].ID)
	assert.EqualValues(t, 22, result.ActiveGroups[0].RoomID)
	assert.EqualValues(t, 1, result.ActiveGroups[1].ID)
	assert.EqualValues(t, 11, result.ActiveGroups[1].RoomID)
	assert.Same(t, result.ActiveGroups[0], result.GroupMappings[0].ActiveGroup)
	assert.Same(t, result.ActiveGroups[1], result.GroupMappings[1].ActiveGroup)
	assert.EqualValues(t, 3, result.GroupMappings[2].ActiveGroupID)
	assert.Nil(t, result.GroupMappings[2].ActiveGroup)
}
