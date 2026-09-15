package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupQueryBudgetsStayFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Group query budget")
	groupIDs := addBudgetGroups(t, ctx, module, category.ID, 0, 3)
	counter := testpkg.CaptureQueries(t, db)
	small := captureGroupQueries(t, ctx, module, counter, groupIDs)
	groupIDs = append(groupIDs, addBudgetGroups(t, ctx, module, category.ID, 3, 8)...)
	large := captureGroupQueries(t, ctx, module, counter, groupIDs)

	assertQueryCountStable(t, small, large, "modules.timetable.groups.list")
	testpkg.AssertQueryBudget(t, "modules.timetable.groups.list", large["modules.timetable.groups.list"])
	assertQueryCountStable(t, small, large, "modules.timetable.group_targets.list")
	testpkg.AssertQueryBudget(t, "modules.timetable.group_targets.list", large["modules.timetable.group_targets.list"])
	assertQueryCountStable(t, small, large, "modules.timetable.target_students.list")
	testpkg.AssertQueryBudget(t, "modules.timetable.target_students.list", large["modules.timetable.target_students.list"])
}

func assertQueryCountStable(t *testing.T, small, large map[string][]string, scenario string) {
	t.Helper()
	assert.Equal(t, len(small[scenario]), len(large[scenario]), "%s must not grow with group count", scenario)
}

func addBudgetGroups(t *testing.T, ctx context.Context, module *timetable.Module, categoryID int64, from, to int) []int64 {
	t.Helper()
	groupIDs := make([]int64, 0, to-from)
	for i := from; i < to; i++ {
		group, err := module.CreateGroup(ctx, timetable.GroupInput{Name: fmt.Sprintf("Budget group %d", i), CategoryID: categoryID})
		require.NoError(t, err)
		class := fmt.Sprintf("%da", i+1)
		require.NoError(t, module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
			TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class,
		}}))
		groupIDs = append(groupIDs, group.ID)
	}
	return groupIDs
}

func captureGroupQueries(t *testing.T, ctx context.Context, module *timetable.Module, counter *testpkg.QueryCounter, groupIDs []int64) map[string][]string {
	t.Helper()
	result := make(map[string][]string, 3)
	counter.Reset()
	_, err := module.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs})
	require.NoError(t, err)
	result["modules.timetable.groups.list"] = counter.Queries()
	counter.Reset()
	_, err = module.ListGroupTargets(ctx, groupIDs)
	require.NoError(t, err)
	result["modules.timetable.group_targets.list"] = counter.Queries()
	counter.Reset()
	_, err = module.ListTargetStudentIDs(ctx, groupIDs)
	require.NoError(t, err)
	result["modules.timetable.target_students.list"] = counter.Queries()
	return result
}
