package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsPlannedSupervisorLifecycleAndBlockers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Owner")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor owner lifecycle")

	created := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)
	found, err := module.FindPlannedSupervisor(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, staff.ID, found.StaffID)

	weekday := 2
	updated, err := module.UpdatePlannedSupervisor(ctx, created.ID, timetable.PlannedSupervisorInput{
		StaffID: staff.ID, GroupID: group.ID, IsPrimary: true, ValidFrom: "2026-09-01", Weekday: &weekday,
	})
	require.NoError(t, err)
	assert.True(t, updated.IsPrimary)
	assert.Equal(t, &weekday, updated.Weekday)

	blockers, err := module.ListPlannedSupervisionBlockers(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, blockers, 1)
	assert.Equal(t, group.Name, blockers[0].ActivityName)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_planned_supervision_blockers").Stats.Queries)

	require.NoError(t, module.DeletePlannedSupervisor(ctx, created.ID))
	_, err = module.FindPlannedSupervisor(ctx, created.ID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
}

func TestModulePlannedSupervisorsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Owned", "Supervisor")
	group := testpkg.CreateTestActivityGroup(t, db, "Owned supervisor group")
	owned := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Foreign", "Supervisor")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign supervisor group")
	foreign := createOwnedSupervisor(t, module, foreignCtx, foreignStaff.ID, foreignGroup.ID, false)

	_, err := module.FindPlannedSupervisor(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
	listed, err := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: []int64{group.ID, foreignGroup.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, owned.ID, listed[0].ID)

	_, err = module.UpdatePlannedSupervisor(foreignCtx, owned.ID, supervisorInput(foreignStaff.ID, foreignGroup.ID, false))
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
	err = module.SetPrimaryPlannedSupervisor(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
	require.NoError(t, module.DeletePlannedSupervisor(foreignCtx, owned.ID))
	stillOwned, err := module.FindPlannedSupervisor(ctx, owned.ID)
	require.NoError(t, err)
	assert.False(t, stillOwned.IsPrimary)
	_, err = module.FindPlannedSupervisor(ctx, foreign.ID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
}

func TestModulePlannedSupervisorRejectsForeignTenantReferences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Foreign", "Reference")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign reference group")

	_, err := module.CreatePlannedSupervisor(ctx, supervisorInput(foreignStaff.ID, foreignGroup.ID, false))
	require.Error(t, err)
	listed, listErr := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{})
	require.NoError(t, listErr)
	assert.Empty(t, listed)
}

func TestModulePlannedSupervisorReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindPlannedSupervisor(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{})
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListPlannedSupervisionBlockers(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
}

func TestModulePlannedSupervisorLifecycleWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Lifecycle", "Rollback")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor lifecycle rollback")

	var rolledBackID int64
	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		created, err := module.CreatePlannedSupervisor(txCtx, supervisorInput(staff.ID, group.ID, false))
		rolledBackID = created.ID
		return err
	})
	_, err := module.FindPlannedSupervisor(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
	supervisor := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		_, err := module.UpdatePlannedSupervisor(txCtx, supervisor.ID, supervisorInput(staff.ID, group.ID, true))
		return err
	})
	assertSupervisorPrimary(t, module, ctx, supervisor.ID, false)
	_, err = module.UpdatePlannedSupervisor(ctx, supervisor.ID, supervisorInput(staff.ID, group.ID, true))
	require.NoError(t, err)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error { return module.DeletePlannedSupervisor(txCtx, supervisor.ID) })
	_, err = module.FindPlannedSupervisor(ctx, supervisor.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeletePlannedSupervisor(ctx, supervisor.ID))
}

func TestModulePlannedSupervisorSetPrimaryRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Primary", "Rollback")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor primary rollback")
	supervisor := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error { return module.SetPrimaryPlannedSupervisor(txCtx, supervisor.ID) })
	assertSupervisorPrimary(t, module, ctx, supervisor.ID, false)
	require.NoError(t, module.SetPrimaryPlannedSupervisor(ctx, supervisor.ID))
	assertSupervisorPrimary(t, module, ctx, supervisor.ID, true)
}

func TestModulePlannedSupervisorDeleteByStaffRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Bulk", "Delete")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor bulk delete rollback")
	createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		rows, err := module.DeletePlannedSupervisorsByStaff(txCtx, staff.ID)
		assert.EqualValues(t, 1, rows)
		return err
	})
	assertSupervisorCount(t, module, ctx, staff.ID, 1)
	rows, err := module.DeletePlannedSupervisorsByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	assertSupervisorCount(t, module, ctx, staff.ID, 0)
}

func TestModulePlannedSupervisorValidityWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Validity", "Rollback")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor validity rollback")
	first := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		return module.SetPlannedSupervisorValidUntil(txCtx, first.ID, "2026-09-10")
	})
	assertSupervisorValidUntil(t, module, ctx, first.ID, nil)
	require.NoError(t, module.SetPlannedSupervisorValidUntil(ctx, first.ID, "2026-09-10"))
	want := "2026-09-10"
	assertSupervisorValidUntil(t, module, ctx, first.ID, &want)

	second := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)
	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		return module.CloseOpenPlannedSupervisors(txCtx, group.ID, nil, "2026-09-11")
	})
	assertSupervisorValidUntil(t, module, ctx, second.ID, nil)
	require.NoError(t, module.CloseOpenPlannedSupervisors(ctx, group.ID, nil, "2026-09-11"))
	want = "2026-09-11"
	assertSupervisorValidUntil(t, module, ctx, second.ID, &want)
}

func TestModuleCapActivePlannedSupervisorsRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staffA := testpkg.CreateTestStaff(t, db, "Cap", "Primary")
	staffB := testpkg.CreateTestStaff(t, db, "Cap", "Secondary")
	group := testpkg.CreateTestActivityGroup(t, db, "Supervisor cap rollback")
	createOwnedSupervisor(t, module, ctx, staffA.ID, group.ID, true)
	createOwnedSupervisor(t, module, ctx, staffB.ID, group.ID, false)

	rollbackSupervisorWrite(t, ctx, func(txCtx context.Context) error {
		rows, err := module.CapActivePlannedSupervisors(txCtx, group.ID, "2026-09-10")
		assert.EqualValues(t, 2, rows)
		return err
	})
	assertGroupSupervisorValidUntil(t, module, ctx, group.ID, nil)
	rows, err := module.CapActivePlannedSupervisors(ctx, group.ID, "2026-09-10")
	require.NoError(t, err)
	assert.EqualValues(t, 2, rows)
	want := "2026-09-10"
	assertGroupSupervisorValidUntil(t, module, ctx, group.ID, &want)
}

func TestPlannedSupervisorListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Budget")
	for i := 0; i < 8; i++ {
		group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("supervisor-owner-budget-%d", i))
		createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, i == 0)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staff.ID})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.supervisors.list", counter.Queries())
}

func createOwnedSupervisor(t *testing.T, module *timetable.Module, ctx context.Context, staffID, groupID int64, primary bool) timetable.PlannedSupervisor {
	t.Helper()
	value, err := module.CreatePlannedSupervisor(ctx, supervisorInput(staffID, groupID, primary))
	require.NoError(t, err)
	return value
}

func supervisorInput(staffID, groupID int64, primary bool) timetable.PlannedSupervisorInput {
	return timetable.PlannedSupervisorInput{StaffID: staffID, GroupID: groupID, IsPrimary: primary, ValidFrom: "2026-09-01"}
}

func rollbackSupervisorWrite(t *testing.T, ctx context.Context, write func(context.Context) error) {
	t.Helper()
	wantErr := errors.New("abort planned supervisor write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

func assertSupervisorPrimary(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want bool) {
	t.Helper()
	value, err := module.FindPlannedSupervisor(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.IsPrimary)
}

func assertSupervisorCount(t *testing.T, module *timetable.Module, ctx context.Context, staffID int64, want int) {
	t.Helper()
	values, err := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staffID})
	require.NoError(t, err)
	assert.Len(t, values, want)
}

func assertSupervisorValidUntil(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want *string) {
	t.Helper()
	value, err := module.FindPlannedSupervisor(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.ValidUntil)
}

func assertGroupSupervisorValidUntil(t *testing.T, module *timetable.Module, ctx context.Context, groupID int64, want *string) {
	t.Helper()
	values, err := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: []int64{groupID}})
	require.NoError(t, err)
	require.NotEmpty(t, values)
	for _, value := range values {
		assert.Equal(t, want, value.ValidUntil)
	}
}
