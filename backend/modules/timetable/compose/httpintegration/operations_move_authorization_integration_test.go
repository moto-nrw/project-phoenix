package httpintegration_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This characterizes option A of #4736, not an authorization equivalence:
// a planned operator may check a child into B without supervising A or B,
// while the ordinary Presence move rejects that exact actor and child.
// Changing this contract awaits the explicit A/B business decision.
func TestTimetableCheckInPlannedOperatorWithoutMoveSupervision(t *testing.T) {
	t.Parallel()
	for _, bulk := range []bool{false, true} {
		name := "single"
		if bulk {
			name = "bulk"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkPlannedOperatorMove(t, bulk)
		})
	}
}

func checkPlannedOperatorMove(t *testing.T, bulk bool) {
	t.Helper()
	ctx := testpkg.OwnCtx(t)
	db := testpkg.SetupTestDB(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Planned", "Operator")
	ctx = testpkg.IdentityContext(ctx, account.ID, testpkg.Tenant(t), "tenant", []string{"schedules:read"})
	module, _ := api.NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	presence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)).Active
	source := testpkg.CreateTestActiveGroup(t, db,
		testpkg.CreateTestActivityGroup(t, db, "Source A").ID,
		testpkg.CreateTestRoom(t, db, "Source A").ID)
	room := testpkg.CreateTestRoom(t, db, "Target B")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "Target B")
	target := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Status: "active", ActivityGroupID: &targetActivity.ID, ActiveGroupID: &target.ID, Title: "Target B",
	})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	child := testpkg.CreateTestStudent(t, db, "Moving", "Child", "3a")
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, child.ID, "expected")
	device := testpkg.CreateTestDevice(t, db, "move-authorization")
	now := time.Now().Add(-time.Minute)
	testpkg.CreateTestAttendance(t, db, child.ID, staff.ID, device.ID, now, nil)
	testpkg.CreateTestVisit(t, db, child.ID, source.ID, now, nil)

	_, err := presence.MoveStudentsToActiveGroupAuthorized(ctx, []int64{child.ID}, target.ID,
		studentpresence.StudentMoveAuthorization{StaffID: staff.ID})
	require.ErrorIs(t, err, studentpresence.ErrStudentMoveForbidden)
	current, err := presence.GetStudentCurrentVisit(ctx, child.ID)
	require.NoError(t, err)
	require.Equal(t, source.ID, current.ActiveGroupID, "Presence refusal preserves the source visit")

	// A different, unassigned actor must still be refused by Timetable.
	_, outsider := testpkg.CreateTestStaffWithAccount(t, db, "Unassigned", "Operator")
	outsiderCtx := testpkg.IdentityContext(testpkg.Ctx(t), outsider.ID, testpkg.Tenant(t), "tenant", []string{"schedules:read"})
	require.NoError(t, testpkg.WithinTenantContext(t, outsiderCtx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := checkInMove(module.TimetableOperations, txCtx, outsider.ID, instance.ID, child.ID, bulk)
		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		return nil
	}))
	current, err = presence.GetStudentCurrentVisit(ctx, child.ID)
	require.NoError(t, err)
	require.Equal(t, source.ID, current.ActiveGroupID, "Timetable refusal preserves the source visit")

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		roster, err := checkInMove(module.TimetableOperations, txCtx, account.ID, instance.ID, child.ID, bulk)
		require.NoError(t, err)
		if !bulk {
			require.NotNil(t, roster.MovedFrom)
		}
		return nil
	}))
	current, err = presence.GetStudentCurrentVisit(ctx, child.ID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, current.ActiveGroupID)
}

func checkInMove(operations timetable.OperationCapability, ctx context.Context, accountID, instanceID, childID int64, bulk bool) (*timetable.OperationRoster, error) {
	if bulk {
		return operations.CheckInStudents(ctx, accountID, false, instanceID, []int64{childID})
	}
	return operations.CheckInStudent(ctx, accountID, false, instanceID, childID)
}
