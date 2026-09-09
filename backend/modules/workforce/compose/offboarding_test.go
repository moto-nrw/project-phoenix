package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOffboardingPreviewLocksConcurrentShiftWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Locked", "Offboarding")
	runtime := testpkg.ConfigRuntime(db)
	offboarding, err := NewOffboarding(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(Observation) {}}, func(context.Context, workforce.StaffAbsence, int64) error { return nil })
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := offboarding.Preview(txCtx, staff.ID, "2027-10-02")
		if err != nil {
			return err
		}
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		attempted := false
		err = testpkg.WithinCurrentTenant(probeCtx, func(writerCtx context.Context) error {
			if _, err := runtime.DB(writerCtx).ExecContext(writerCtx, "SET LOCAL lock_timeout = '100ms'"); err != nil {
				return err
			}
			attempted = true
			return runtime.AcquireLock(writerCtx, fmt.Sprintf("staff-shift:%d:%d", testpkg.Tenant(t), staff.ID), false)
		})
		require.True(t, attempted, "the probe must reach the writer lock")
		require.Error(t, err, "a concurrent shift writer must wait for the offboarding transaction")
		require.ErrorContains(t, err, "lock timeout")
		return nil
	}))
}

func TestOffboardingWorkforceCannotDeleteAnotherTenantsPlans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Isolated", "Offboarding")
	module := buildWorkforce(t, db)
	absence, err := module.CreateStaffAbsence(ctx, testAbsence(staff.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusReported, timezone.NewDate(2026, 3, 5)))
	require.NoError(t, err)
	shift, err := module.CreateStaffShift(ctx, testShift(staff.ID, timezone.NewDate(2026, 3, 5), "09:00:00", "12:00:00"))
	require.NoError(t, err)
	runtime := testpkg.ConfigRuntime(db)
	offboarding, err := NewOffboarding(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(Observation) {}}, func(context.Context, workforce.StaffAbsence, int64) error {
		return errors.New("foreign absence must never reach the audit command")
	})
	require.NoError(t, err)
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	otherCtx := testpkg.TenantContext(otherID)
	preview, err := offboarding.Preview(otherCtx, staff.ID, "2026-02-01")
	require.NoError(t, err)
	require.Equal(t, workforce.OffboardingCounts{}, preview.Counts)
	counts, err := offboarding.Execute(otherCtx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, workforce.OffboardingCounts{}, counts)
	_, err = module.FindStaffAbsence(ctx, absence.ID)
	require.NoError(t, err)
	_, err = module.FindStaffShift(ctx, shift.ID)
	require.NoError(t, err)
}

func TestOffboardingWorkforceDetectsDriftAndBlocksHandovers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Handover", "Offboarding")
	module := buildWorkforce(t, db)
	runtime := testpkg.ConfigRuntime(db)
	offboarding, err := NewOffboarding(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(Observation) {}}, func(context.Context, workforce.StaffAbsence, int64) error { return nil })
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2026-02-01")
	require.NoError(t, err)
	planned, err := module.CreateStaffShift(ctx, testShift(staff.ID, timezone.NewDate(2026, 3, 5), "09:00:00", "12:00:00"))
	require.NoError(t, err)
	_, err = offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.ErrorIs(t, err, workforce.ErrOffboardingConflict)
	_, err = module.FindStaffShift(ctx, planned.ID)
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroup(t, db, "Offboarding handover")
	handover, err := module.CreateGroupSubstitution(ctx, workforce.GroupSubstitution{TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: group.ID, SubstituteStaffID: staff.ID, StartDate: "2026-02-01", EndDate: "2026-02-05"})
	require.NoError(t, err)
	preview, err = offboarding.Preview(ctx, staff.ID, "2026-02-01")
	require.NoError(t, err)
	require.True(t, preview.Blocked)
	_, err = offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.ErrorIs(t, err, workforce.ErrOffboardingInUse)
	_, err = module.FindStaffShift(ctx, planned.ID)
	require.NoError(t, err)
	_, err = module.FindGroupSubstitution(ctx, handover.ID)
	require.NoError(t, err)
}

func TestOffboardingWorkforceRollsBackPlansAndCapsSeriesOnRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Planning", "Offboarding")
	module := buildWorkforce(t, db)
	runtime := testpkg.ConfigRuntime(db)
	offboarding, err := NewOffboarding(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(Observation) {}}, func(context.Context, workforce.StaffAbsence, int64) error { return nil })
	require.NoError(t, err)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Offboarding year", timezone.NewDate(2026, 1, 1), timezone.NewDate(2026, 12, 31))
	series, err := module.CreateStaffShiftSeries(ctx, workforce.StaffShiftSeries{StaffID: staff.ID, Weekdays: []int{1}, StartTime: "09:00:00", EndTime: "12:00:00", CalendarPeriodID: period.ID, ValidFrom: "2026-01-01", CreatedBy: staff.ID})
	require.NoError(t, err)
	history, err := module.CreateStaffShift(ctx, testShift(staff.ID, timezone.NewDate(2026, 1, 5), "09:00:00", "12:00:00"))
	require.NoError(t, err)
	planned, err := module.CreateStaffShift(ctx, testShift(staff.ID, timezone.NewDate(2026, 3, 5), "09:00:00", "12:00:00"))
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2026-02-01")
	require.NoError(t, err)
	require.Equal(t, workforce.OffboardingCounts{Shifts: 1, Series: 1}, preview.Counts)
	failure := errors.New("later workflow owner failed")
	err = testpkg.WithinTenantContext(t, ctx, db, staff.TenantID, func(txCtx context.Context) error {
		counts, executeErr := offboarding.Execute(txCtx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
		require.NoError(t, executeErr)
		require.Equal(t, preview.Counts, counts)
		return failure
	})
	require.ErrorIs(t, err, failure)
	_, err = module.FindStaffShift(ctx, planned.ID)
	require.NoError(t, err)
	uncapped, err := module.FindStaffShiftSeries(ctx, series.ID)
	require.NoError(t, err)
	require.Empty(t, uncapped.ValidUntil)
	counts, err := offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, preview.Counts, counts)
	_, err = module.FindStaffShift(ctx, planned.ID)
	require.ErrorIs(t, err, workforce.ErrStaffShiftNotFound)
	_, err = module.FindStaffShift(ctx, history.ID)
	require.NoError(t, err)
	capped, err := module.FindStaffShiftSeries(ctx, series.ID)
	require.NoError(t, err)
	require.Equal(t, "2026-02-01", capped.ValidUntil)
	counts, err = offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, workforce.OffboardingCounts{}, counts)
}

func TestOffboardingWorkforcePreservesHistoryAndRequiresAbsenceAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Workforce", "Offboarding")
	module := buildWorkforce(t, db)
	history, err := module.CreateStaffAbsence(ctx, testAbsence(staff.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusReported, timezone.NewDate(2026, 1, 5)))
	require.NoError(t, err)
	future, err := module.CreateStaffAbsence(ctx, testAbsence(staff.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusReported, timezone.NewDate(2026, 3, 5)))
	require.NoError(t, err)
	runtime := testpkg.ConfigRuntime(db)
	failure := errors.New("absence audit unavailable")
	var archived []workforce.StaffAbsence
	offboarding, err := NewOffboarding(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(Observation) {}}, func(_ context.Context, absence workforce.StaffAbsence, actorID int64) error {
		require.Equal(t, staff.ID, actorID)
		if failure != nil {
			return failure
		}
		archived = append(archived, absence)
		return nil
	})
	require.NoError(t, err)
	preview, err := offboarding.Preview(ctx, staff.ID, "2026-02-01")
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Counts.Absences)
	_, err = offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.ErrorIs(t, err, failure)
	_, err = module.FindStaffAbsence(ctx, future.ID)
	require.NoError(t, err)
	failure = nil
	counts, err := offboarding.Execute(ctx, staff.ID, staff.ID, "2026-02-01", preview.Revision)
	require.NoError(t, err)
	require.Equal(t, preview.Counts, counts)
	require.Len(t, archived, 1)
	require.Equal(t, future, archived[0])
	_, err = module.FindStaffAbsence(ctx, future.ID)
	require.ErrorIs(t, err, workforce.ErrStaffAbsenceNotFound)
	_, err = module.FindStaffAbsence(ctx, history.ID)
	require.NoError(t, err)
}
