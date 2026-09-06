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

func TestModuleOwnsInstanceStaffLifecycleAndQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-lifecycle")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-01", "08:00:00", "Staff")
	firstStaff := testpkg.CreateTestStaff(t, db, "Owner", "Primary")
	secondStaff := testpkg.CreateTestStaff(t, db, "Owner", "Absent")
	first := createOwnedInstanceStaff(t, module, ctx, instance.ID, firstStaff.ID, true, false)
	second := createOwnedInstanceStaff(t, module, ctx, instance.ID, secondStaff.ID, false, true)

	found, err := module.FindInstanceStaff(ctx, first.ID)
	require.NoError(t, err)
	assert.True(t, found.IsPrimary)
	listed, err := module.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{
		InstanceIDs: []int64{instance.ID}, OrderByCreated: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, instanceStaffIDs(listed))
	counts, err := module.CountNonAbsentInstanceStaff(ctx, []int64{instance.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[instance.ID])
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_instance_staff").Stats.Queries)
}

func TestModuleOwnsInstanceStaffWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-writes")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-02", "08:00:00", "Writes")
	staff := testpkg.CreateTestStaff(t, db, "Owner", "Writer")
	created := createOwnedInstanceStaff(t, module, ctx, instance.ID, staff.ID, false, false)

	input := ownedInstanceStaffInput(instance.ID, staff.ID, true, true)
	updated, err := module.UpdateInstanceStaff(ctx, created.ID, input)
	require.NoError(t, err)
	assert.True(t, updated.IsPrimary)
	rows, err := module.PatchInstanceStaff(ctx, created.ID, timetable.InstanceStaffInput{}, []string{"is_primary"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	stored, err := module.FindInstanceStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, stored.IsPrimary)
	require.NoError(t, module.DeleteInstanceStaffByInstance(ctx, instance.ID))
	listed, err := module.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instance.ID}})
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestModuleInstanceStaffDuplicateIsObserved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-duplicate")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-03", "08:00:00", "Duplicate")
	staff := testpkg.CreateTestStaff(t, db, "Owner", "Duplicate")
	input := ownedInstanceStaffInput(instance.ID, staff.ID, false, false)

	_, err := module.CreateInstanceStaff(ctx, input)
	require.NoError(t, err)
	_, err = module.CreateInstanceStaff(ctx, input)
	require.Error(t, err)
	assert.EqualValues(t, 1, lastObservedOperation(log.seen, "create_instance_staff").Stats.DuplicatePreventionConflicts)
}

func TestModuleInstanceStaffAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-owned")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-04", "08:00:00", "Owned")
	staff := testpkg.CreateTestStaff(t, db, "Owner", "Tenant")
	owned := createOwnedInstanceStaff(t, module, ctx, instance.ID, staff.ID, false, false)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Foreign", "Staff")
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	_, err := module.CreateInstanceStaff(foreignCtx, ownedInstanceStaffInput(instance.ID, foreignStaff.ID, false, false))
	require.Error(t, err, "foreign assignment cannot reference the owned instance")
	_, err = module.FindInstanceStaff(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStaffNotFound)
	require.NoError(t, module.DeleteInstanceStaff(foreignCtx, owned.ID))
	_, err = module.FindInstanceStaff(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModuleDeletesOnlyUpcomingInstanceStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-upcoming")
	staff := testpkg.CreateTestStaff(t, db, "Owner", "Offboard")
	past := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-04", "08:00:00", "Past")
	today := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-05", "08:00:00", "Today")
	future := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-06", "08:00:00", "Future")
	pastRow := createOwnedInstanceStaff(t, module, ctx, past.ID, staff.ID, false, false)
	createOwnedInstanceStaff(t, module, ctx, today.ID, staff.ID, false, false)
	createOwnedInstanceStaff(t, module, ctx, future.ID, staff.ID, false, false)

	rows, err := module.DeleteUpcomingInstanceStaff(ctx, staff.ID, "2027-10-05")
	require.NoError(t, err)
	assert.EqualValues(t, 2, rows)
	_, err = module.FindInstanceStaff(ctx, pastRow.ID)
	require.NoError(t, err)
}

func TestModuleInstanceStaffFailuresAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-rollback")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-07", "08:00:00", "Rollback")
	staff := testpkg.CreateTestStaff(t, db, "Owner", "Rollback")
	wantErr := errors.New("abort instance staff write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateInstanceStaff(txCtx, ownedInstanceStaffInput(instance.ID, staff.ID, false, false))
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindInstanceStaff(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrInstanceStaffNotFound)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.ListInstanceStaff(cancelled, timetable.InstanceStaffFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestInstanceStaffListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "staff-budget")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-08", "08:00:00", "Budget")
	for index := 0; index < 8; index++ {
		staff := testpkg.CreateTestStaff(t, db, "Owner", fmt.Sprintf("Budget-%d", index))
		createOwnedInstanceStaff(t, module, ctx, instance.ID, staff.ID, false, false)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instance.ID}})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.instance_staff.list", counter.Queries())
}

func createOwnedInstanceStaff(t *testing.T, module *timetable.Module, ctx context.Context, instanceID, staffID int64, primary, absent bool) timetable.InstanceStaff {
	t.Helper()
	value, err := module.CreateInstanceStaff(ctx, ownedInstanceStaffInput(instanceID, staffID, primary, absent))
	require.NoError(t, err)
	return value
}

func ownedInstanceStaffInput(instanceID, staffID int64, primary, absent bool) timetable.InstanceStaffInput {
	return timetable.InstanceStaffInput{InstanceID: instanceID, StaffID: staffID, IsPrimary: primary, IsAbsent: absent}
}

func instanceStaffIDs(values []timetable.InstanceStaff) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}
