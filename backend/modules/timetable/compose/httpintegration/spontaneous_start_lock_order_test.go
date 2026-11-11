package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A spontaneous start runs, in one unit of work, the start's gates and
// catalogue rows (room gate, activity name, the "Spontan" category), the
// block's create and its start (#6702). The create names its people in
// request order and the start takes the block's day. The lock order needs
// the recurrence gate before the catalogue rows, the day before the people
// and the people ascending; before #6702 the create locked the people one by
// one and the start took the day after them, which the ledger refuses.
func TestSpontaneousStartFollowsTheLockOrder(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	higher := testpkg.CreateTestStaff(t, s.db, "Spontan", fmt.Sprintf("Higher-%d", time.Now().UnixNano()))
	require.Less(t, s.staffID, higher.ID)
	day := calendar.NewDate(2026, time.April, 20)
	err := testpkg.WithTenantTx(t, s.ctx, s.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		if err := s.timetableData.RecurrenceLock.LockRecurrenceWritesThenGradeTransitions(ctx); err != nil {
			return err
		}
		if err := s.timetableData.Data.LockSpontaneousStartRoom(ctx, s.roomID); err != nil {
			return err
		}
		groupID, err := s.timetableData.Data.ResolveSpontaneousActivity(ctx, fmt.Sprintf("Spontan %d", time.Now().UnixNano()), nil, s.staffID)
		if err != nil {
			return err
		}
		spontaneous := true
		created, err := s.svc.CreateInstance(ctx, timetable.CreateInstanceInput{
			Date: day, StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), EndTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
			Title: "Spontan", RoomID: s.roomID, ActivityGroupID: groupID, IsSpontaneous: &spontaneous,
			StaffIDs: []int64{higher.ID, s.staffID}, CreatedByStaffID: &s.staffID,
		})
		if err != nil {
			return err
		}
		for _, staffID := range []int64{s.staffID, higher.ID} {
			require.True(t, tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(staffID)), "the create holds staff row %d", staffID)
		}
		require.True(t, tenant.HoldsLock(ctx, tenant.LockScheduleDay, day.String()), "the create holds the block's day")
		_, err = s.svc.Start(ctx, created.ID, s.staffID)
		return err
	})
	require.NoError(t, err)
}
