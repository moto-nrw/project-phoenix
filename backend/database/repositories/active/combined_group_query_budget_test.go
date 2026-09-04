package active_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombinedGroupFindWithGroupsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	combined := &active.CombinedGroup{StartTime: time.Now()}
	require.NoError(t, factory.CombinedGroup.Create(ctx, combined))

	add := func(from, to int) {
		for i := from; i < to; i++ {
			activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("combined-budget-%d", i))
			room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Combined Budget %d", i))
			groupID := activityGroup.ID
			group := &active.Group{
				StartTime: time.Now(), LastActivity: time.Now(), TimeoutMinutes: 30,
				GroupID: &groupID, RoomID: room.ID,
			}
			require.NoError(t, factory.ActiveGroup.Create(ctx, group))
			require.NoError(t, factory.GroupMapping.AddGroupToCombination(ctx, combined.ID, group.ID))
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueries(t, db)
	run := func() []string {
		counter.Reset()
		found, err := factory.CombinedGroup.FindWithGroups(ctx, combined.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "repositories.active.combined_group_with_groups.reads", large)
}
