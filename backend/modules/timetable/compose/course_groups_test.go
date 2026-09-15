package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCourseGroupQueriesAreBoundedAndTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	group := testpkg.CreateTestActivityGroup(t, db, "Native review course")
	_, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: timetable.WeekdayWednesday})
	require.NoError(t, err)
	var observations []Observation
	query, err := NewCourseGroupQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	rows, err := query.ListCourseGroups(ctx, timetable.CourseGroupFilter{LegacyGroupIDs: []int64{group.ID}, EffectiveOn: "2030-09-01"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, group.ID, rows[0].ID)
	require.Equal(t, []int{timetable.WeekdayWednesday}, rows[0].ScheduledWeekdays)
	require.Equal(t, int64(1), observations[len(observations)-1].Stats.Queries)
	before := len(observations)
	rows, err = query.ListCourseGroups(ctx, timetable.CourseGroupFilter{})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Len(t, observations, before, "an empty reference set never reads the school-wide catalog")
	_, err = query.ListCourseGroups(ctx, timetable.CourseGroupFilter{LegacyGroupIDs: []int64{0}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	require.Zero(t, observations[len(observations)-1].Stats.Queries)
	foreignID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignID)
	foreign := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignID)
	rows, err = query.ListCourseGroups(foreign, timetable.CourseGroupFilter{LegacyGroupIDs: []int64{group.ID}})
	require.NoError(t, err)
	require.Empty(t, rows)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = query.ListCourseGroups(canceled, timetable.CourseGroupFilter{LegacyGroupIDs: []int64{group.ID}})
	require.ErrorIs(t, err, context.Canceled)
	_, err = NewCourseGroupQueries(nil, func(Observation) {})
	require.Error(t, err)
	_, err = NewCourseGroupQueries(db, nil)
	require.Error(t, err)
}
