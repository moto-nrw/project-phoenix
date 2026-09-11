package compose_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSupervisedRoomScopeAgreesWithBulkReader documents where the two ways of
// answering "which rooms does this person supervise right now" agree and where
// they deliberately do not.
//
// Compute walks active.group_supervisors -> active.groups and never checks
// whether the session is still open, because the underlying FindByIDs has no
// end_time predicate. ComputeBatch uses ListActiveSupervisedRooms, which does
// check it. A room whose session already ended therefore surfaces on the single
// path and not on the batch path.
//
// The batch is the stricter and more correct side: a closed session holds no
// children, so an activity reminder for its room is noise. The divergence is
// accepted rather than fixed, because aligning it would change Compute's
// production behaviour with no test coverage on either side. This test pins the
// DIRECTION so nobody flips it by accident: whatever the bulk reader returns
// must always be a subset of what the single path returns.
//
// The in-memory equivalence suite cannot see any of this — its fixtures serve
// both paths from one field, so SQL predicate drift is invisible there. That is
// exactly why this test talks to a real database.
func TestSupervisedRoomScopeAgreesWithBulkReader(t *testing.T) {
	t.Parallel()
	day := testpkg.Date(2026, time.September, 4)
	now := day.BerlinMidnight().Add(12 * time.Hour)
	db, module := testutil.SetupRemindersModule(t, func() time.Time { return now })
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.Settings.SetValue(ctx, "reminders.activity_start_enabled", true, nil, nil))

	staff := testpkg.CreateTestStaff(t, db, "RoomScope", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "RoomScopeActivity")

	openRoom := testpkg.CreateTestRoom(t, db, "RoomScopeOpen")
	closedRoom := testpkg.CreateTestRoom(t, db, "RoomScopeClosed")
	expiredRoom := testpkg.CreateTestRoom(t, db, "RoomScopeExpired")

	openGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, openRoom.ID)
	closedGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, closedRoom.ID)
	expiredGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, expiredRoom.ID)

	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, openGroup.ID, "primary")
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, closedGroup.ID, "primary")
	expiredSup := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, expiredGroup.ID, "primary")

	// The session in closedRoom is over, but the supervision row is still open —
	// the state the nightly stale-supervisor cleanup exists to resolve.
	_, err := db.NewUpdate().
		TableExpr("active.groups").
		Set("end_time = ?", time.Now()).
		Where("id = ?", closedGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// The supervision in expiredRoom ended yesterday.
	_, err = db.NewUpdate().
		TableExpr("active.group_supervisors").
		Set("end_date = ?", testpkg.TodayDate().AddDays(-1)).
		Where("id = ?", expiredSup.ID).
		Exec(ctx)
	require.NoError(t, err)

	instanceIDs := make(map[int64]string)
	for _, roomID := range []int64{openRoom.ID, closedRoom.ID, expiredRoom.ID} {
		instance := testpkg.CreateTestActivityInstance(t, db, day, roomID, testpkg.ActivityInstanceOpts{
			StartHHMM: "12:00", EndHHMM: "13:00", Title: "Room scope",
		})
		instanceIDs[roomID] = strconv.FormatInt(instance.ID, 10)
	}
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		scope := reminder.Scope{StaffID: staff.ID}
		single, err := module.Reminders.Compute(ctx, scope)
		require.NoError(t, err)
		batch, err := module.Reminders.ComputeBatch(ctx, []reminder.BatchScope{{Scope: scope}})
		require.NoError(t, err)
		require.Contains(t, batch, staff.ID)

		activityIDs := func(result *reminder.Result) []string {
			ids := make([]string, 0, len(result.Reminders))
			for _, item := range result.Reminders {
				require.NotNil(t, item.ActivityInstanceID)
				ids = append(ids, *item.ActivityInstanceID)
			}
			return ids
		}
		assert.ElementsMatch(t, []string{instanceIDs[openRoom.ID], instanceIDs[closedRoom.ID]}, activityIDs(single),
			"the single query includes the closed session but excludes expired supervision")
		assert.Equal(t, []string{instanceIDs[openRoom.ID]}, activityIDs(batch[staff.ID]),
			"the batch query excludes both closed sessions and expired supervision")
		return nil
	}))
}
