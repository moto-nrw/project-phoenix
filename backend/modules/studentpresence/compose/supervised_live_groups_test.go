package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSupervisedLiveGroupsApplyTheOwnersSupervisionRule(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	var observation compose.Observation
	// 22:30 UTC is 00:30 on 2026-09-15 in Berlin: the school day, not the UTC day.
	now := func() time.Time { return time.Date(2026, time.September, 14, 22, 30, 0, 0, time.UTC) }
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { observation = o }, Now: now})
	require.NoError(t, err)
	tenantID := testpkg.Tenant(t)
	staff := testpkg.CreateTestStaff(t, db, "Supervised", "Reader")
	colleague := testpkg.CreateTestStaff(t, db, "Other", "Supervisor")

	yesterday, todayValue, tomorrow := "2026-09-14", "2026-09-15", "2026-09-16"
	supervise := func(groupID, staffID int64, start string, end *string) {
		t.Helper()
		_, recordErr := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
			GroupID: groupID, StaffID: staffID, Role: "supervisor", StartDate: start, EndDate: end,
		})
		require.NoError(t, recordErr)
	}

	current := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	supervise(current.ID, staff.ID, yesterday, nil)
	// A second, already ended row on the same session must not duplicate it.
	supervise(current.ID, staff.ID, yesterday, &yesterday)
	endingToday := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	supervise(endingToday.ID, staff.ID, yesterday, &todayValue)
	startsTomorrow := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	supervise(startsTomorrow.ID, staff.ID, tomorrow, nil)
	endedSession := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	supervise(endedSession.ID, staff.ID, todayValue, nil)
	require.NoError(t, module.EndGroup(ctx, endedSession.ID, time.Now()))
	colleagues := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	supervise(colleagues.ID, colleague.ID, todayValue, nil)

	rows, err := module.ListSupervisedLiveGroups(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, current.ID, rows[0].ID)
	require.Equal(t, current.RoomID, rows[0].RoomID)
	require.Equal(t, current.GroupID, rows[0].ActivityGroupID)
	require.Equal(t, tenantID, rows[0].TenantID)
	require.True(t, rows[0].IsOpen())
	// One supervision read and one batched session read, however many rows.
	require.Equal(t, 2, observation.Queries)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		foreignRows, readErr := module.ListSupervisedLiveGroups(otherCtx, staff.ID)
		require.Empty(t, foreignRows)
		return readErr
	}))

	_, err = module.ListSupervisedLiveGroups(ctx, 0)
	require.Error(t, err)
	_, err = module.ListSupervisedLiveGroups(context.Background(), staff.ID)
	require.Error(t, err)
}

func TestOpenLiveGroupsForActivitiesExcludeEndedAndForeignSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	tenantID := testpkg.Tenant(t)
	running := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	ended := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	require.NoError(t, module.EndGroup(ctx, ended.ID, time.Now()))
	unrelated := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)

	rows, err := module.ListOpenLiveGroupsForActivities(ctx, []int64{*running.GroupID, *ended.GroupID, *foreign.GroupID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, running.ID, rows[0].ID)
	require.NotEqual(t, unrelated.ID, rows[0].ID)

	// No activity groups means no sessions, never every open session.
	for _, ids := range [][]int64{nil, {}} {
		rows, err = module.ListOpenLiveGroupsForActivities(ctx, ids)
		require.NoError(t, err)
		require.Empty(t, rows)
	}
	_, err = module.ListOpenLiveGroupsForActivities(ctx, []int64{0})
	require.Error(t, err)
}
