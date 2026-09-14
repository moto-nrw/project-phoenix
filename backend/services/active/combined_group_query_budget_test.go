package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombinedGroupFindWithGroupsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := services.NewGroupsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)
	combined, err := presence.RecordCombination(ctx, time.Now(), nil)
	require.NoError(t, err)

	add := func(count int) {
		for range count {
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			require.NoError(t, presence.AddGroupToCombination(ctx, combined.ID, group.ID))
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	run := func(want int) []string {
		counter.Reset()
		found, err := module.Active.GetCombinedGroupWithGroups(ctx, combined.ID)
		require.NoError(t, err)
		require.Equal(t, combined.ID, found.ID)
		require.Len(t, found.GroupMappings, want)
		require.Len(t, found.ActiveGroups, want)
		for _, mapping := range found.GroupMappings {
			require.NotNil(t, mapping.ActiveGroup)
			require.Equal(t, mapping.ActiveGroupID, mapping.ActiveGroup.ID)
		}
		return counter.Operation("SELECT")
	}
	run(0)
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "repositories.active.combined_group_with_groups.reads", large)
}
