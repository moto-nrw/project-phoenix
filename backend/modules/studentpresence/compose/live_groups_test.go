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

func TestLiveGroupBatchReadIsTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	other := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)
	rows, err := module.ListLiveGroups(ctx, []int64{group.ID, other.ID, group.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, group.ID, rows[0].ID)
	require.Equal(t, group.RoomID, rows[0].RoomID)
	require.Equal(t, group.GroupID, rows[0].ActivityGroupID)
	require.Equal(t, group.TimeoutMinutes, rows[0].TimeoutMinutes)
	require.True(t, group.StartTime.Equal(rows[0].StartTime))
	require.Equal(t, testpkg.Tenant(t), rows[0].TenantID)
	rows, err = module.ListLiveGroups(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = module.ListLiveGroups(context.Background(), nil)
	require.Error(t, err)
	_, err = module.ListLiveGroups(ctx, []int64{0})
	require.Error(t, err)
}

func TestLiveGroupFiltersPreserveScopeAndInclusiveOverlap(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	var observation compose.Observation
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { observation = o }})
	require.NoError(t, err)
	filter := studentpresence.LiveGroupFilter{RoomID: &group.RoomID, OpenOnly: true}
	rows, err := module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, group.ID, rows[0].ID)
	require.Equal(t, 1, observation.Queries)
	filter.ActivityGroupIDs = []int64{*group.GroupID}
	rows, err = module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	filter.ActivityGroupIDs = []int64{}
	rows, err = module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, observation.Queries)
	_, err = module.QueryLiveGroups(context.Background(), filter)
	require.Error(t, err)
	filter.ActivityGroupIDs = nil
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		foreignRows, readErr := module.QueryLiveGroups(otherCtx, filter)
		require.Empty(t, foreignRows)
		return readErr
	}))
	ended := group.StartTime.Add(time.Hour).Truncate(time.Microsecond)
	require.NoError(t, module.EndGroup(ctx, group.ID, ended))
	rows, err = module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, rows)
	filter.OpenOnly = false
	filter.From, filter.Until = &ended, &ended
	rows, err = module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	later := ended.Add(time.Microsecond)
	filter.From, filter.Until = &later, &later
	rows, err = module.QueryLiveGroups(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, rows)
	filter.From, filter.Until = &later, &ended
	_, err = module.QueryLiveGroups(ctx, filter)
	require.Error(t, err)
}

func TestOccupiedActivityGroupsAreTenantScopedAndDistinct(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	other := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)
	var observation compose.Observation
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { observation = o }})
	require.NoError(t, err)
	ids, err := module.OccupiedActivityGroupIDs(ctx, []int64{*group.GroupID, *group.GroupID, *other.GroupID})
	require.NoError(t, err)
	require.Equal(t, []int64{*group.GroupID}, ids)
	require.Equal(t, 1, observation.Queries)
	require.NoError(t, module.EndGroup(ctx, group.ID, time.Now()))
	ids, err = module.OccupiedActivityGroupIDs(ctx, []int64{*group.GroupID})
	require.NoError(t, err)
	require.Empty(t, ids)
	ids, err = module.OccupiedActivityGroupIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.Zero(t, observation.Queries)
	_, err = module.OccupiedActivityGroupIDs(context.Background(), nil)
	require.Error(t, err)
	_, err = module.OccupiedActivityGroupIDs(ctx, []int64{0})
	require.Error(t, err)
}

func TestGroupSupervisionReadPreservesDatesAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "Group", "Supervisor")
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	rows, err := module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, supervisor.ID, rows[0].ID)
	require.Equal(t, supervisor.StartDate.String(), rows[0].StartDate)
	cutoff := supervisor.StartDate.String()
	stale, err := module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{OpenOnly: true, StartedBefore: &cutoff})
	require.NoError(t, err)
	require.Empty(t, stale, "start-date cutoff is exclusive")
	cutoff = supervisor.StartDate.AddDays(1).String()
	stale, err = module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{OpenOnly: true, StartedBefore: &cutoff})
	require.NoError(t, err)
	require.Len(t, stale, 1)
	require.Equal(t, supervisor.ID, stale[0].ID)
	invalid := "invalid-date"
	_, err = module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StartedBefore: &invalid})
	require.Error(t, err)
	require.Nil(t, rows[0].EndDate)
	require.Equal(t, staff.ID, rows[0].StaffID)
	require.Equal(t, testpkg.Tenant(t), rows[0].TenantID)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		foreignRows, err := module.ListGroupSupervisions(otherCtx, group.ID)
		require.Empty(t, foreignRows)
		return err
	}))
	_, err = module.ListGroupSupervisions(context.Background(), group.ID)
	require.Error(t, err)
}

func TestLiveGroupListingFiltersEndedSessionsAndPaginates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	second := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	require.NoError(t, module.EndGroup(ctx, first.ID, time.Now()))
	rows, err := module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{EndedOnly: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first.ID, rows[0].ID)
	rows, err = module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, second.ID, rows[0].ID)
	rows, err = module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, second.ID, rows[0].ID)
	_, err = module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{Limit: -1})
	require.Error(t, err)
}
