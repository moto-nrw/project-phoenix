package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOffboardingTimetableKeepsTodaysNonPlannedAssignments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	staff := testpkg.CreateTestStaff(t, db, "SameDay", "Offboarding")
	fixture := newOwnedActivityInstanceFixture(t, db, "offboarding-same-day")
	completed := createOwnedInstanceWithStatus(t, module, ctx, fixture, "2027-10-02", "08:00:00", "Completed", "completed")
	planned := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-02", "10:00:00", "Planned")
	history := createOwnedInstanceStaff(t, module, ctx, completed.ID, staff.ID, true, false)
	future := createOwnedInstanceStaff(t, module, ctx, planned.ID, staff.ID, true, false)
	offboarding, err := NewOffboarding(OffboardingDependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2027-10-02")
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Counts.InstanceAssignments)
	_, err = offboarding.Execute(ctx, staff.ID, "2027-10-02", preview.Revision)
	require.NoError(t, err)
	_, err = module.FindInstanceStaff(ctx, history.ID)
	require.NoError(t, err)
	_, err = module.FindInstanceStaff(ctx, future.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStaffNotFound)
}

func TestOffboardingTimetableDetectsActivityDateDrift(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	staff := testpkg.CreateTestStaff(t, db, "DateDrift", "Offboarding")
	fixture := newOwnedActivityInstanceFixture(t, db, "offboarding-date-drift")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-01", "08:00:00", "Past")
	assignment := createOwnedInstanceStaff(t, module, ctx, instance.ID, staff.ID, true, false)
	offboarding, err := NewOffboarding(OffboardingDependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2027-10-02")
	require.NoError(t, err)
	require.Zero(t, preview.Counts.InstanceAssignments)
	_, err = module.UpdateActivityInstance(ctx, instance.ID, ownedActivityInstanceInput(fixture, "2027-10-03", "08:00:00", "Future"))
	require.NoError(t, err)
	_, err = offboarding.Execute(ctx, staff.ID, "2027-10-02", preview.Revision)
	require.ErrorIs(t, err, timetable.ErrOffboardingConflict)
	_, err = module.FindInstanceStaff(ctx, assignment.ID)
	require.NoError(t, err)
}

func TestOffboardingTimetableRetainsHistoryAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Timetable", "Offboarding")
	fixture := newOwnedActivityInstanceFixture(t, db, "offboarding")
	pastInstance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-01", "08:00:00", "History")
	futureInstance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-03", "08:00:00", "Future")
	history := createOwnedInstanceStaff(t, module, ctx, pastInstance.ID, staff.ID, true, false)
	future := createOwnedInstanceStaff(t, module, ctx, futureInstance.ID, staff.ID, true, false)
	group := testpkg.CreateTestActivityGroup(t, db, "Offboarding planned supervisor")
	supervisor := createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, false)
	offboarding, err := NewOffboarding(OffboardingDependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2027-10-02")
	require.NoError(t, err)
	require.Equal(t, timetable.OffboardingCounts{InstanceAssignments: 1, PlannedSupervisors: 1}, preview.Counts)
	failure := errors.New("later workflow command failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		counts, executeErr := offboarding.Execute(txCtx, staff.ID, "2027-10-02", preview.Revision)
		require.NoError(t, executeErr)
		require.Equal(t, preview.Counts, counts)
		return failure
	})
	require.ErrorIs(t, err, failure)
	_, err = module.FindInstanceStaff(ctx, future.ID)
	require.NoError(t, err)
	_, err = module.FindPlannedSupervisor(ctx, supervisor.ID)
	require.NoError(t, err)
	counts, err := offboarding.Execute(ctx, staff.ID, "2027-10-02", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, preview.Counts, counts)
	_, err = module.FindInstanceStaff(ctx, history.ID)
	require.NoError(t, err)
	_, err = module.FindInstanceStaff(ctx, future.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStaffNotFound)
	_, err = module.FindPlannedSupervisor(ctx, supervisor.ID)
	require.ErrorIs(t, err, timetable.ErrPlannedSupervisorNotFound)
	counts, err = offboarding.Execute(ctx, staff.ID, "2027-10-02", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, timetable.OffboardingCounts{}, counts)
}
