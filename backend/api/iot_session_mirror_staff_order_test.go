package api

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The kiosk mirror copies every supervisor of a started session onto the
// mirrored block (#6702). Each assignment key-shares the block, a leaf of
// the lock order, and a staff row after it is refused. The mirror therefore
// locks all supervisors ascending before the block, whatever order the
// kiosk sent. Before, only the first supervisor of [higher, lower] reached
// the block; the mirror logged the second as failed.
func TestKioskSessionMirrorCopiesEverySupervisorInAnyOrder(t *testing.T) {
	t.Parallel()
	db, _, _, activityService, _, _ := setupIoTActiveModule(t)
	_, timetable := setupIoTTimetableModule(t)
	mirror, instances, err := timetable.KioskSessionMirror(testpkg.NewDatabase(t, db), activityService)
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, "Mirror order activity")
	room := testpkg.CreateTestRoom(t, db, "Mirror order room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	lower := testpkg.CreateTestStaff(t, db, "Mirror", "Lower")
	higher := testpkg.CreateTestStaff(t, db, "Mirror", "Higher")
	require.Less(t, lower.ID, higher.ID)
	startedAt := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		mirrorKioskSessionAt(ctx, mirror, group.ID, group.RoomID, startedAt, higher.ID, lower.ID)
		instance, err := instances.FindByActiveGroupID(ctx, group.ID)
		require.NoError(t, err)
		require.NotNil(t, instance, "the session is mirrored")
		rows, err := timetable.TimetableData.Data.ListBlockStaff(ctx, instance.ID)
		require.NoError(t, err)
		staff := make([]int64, 0, len(rows))
		for _, row := range rows {
			staff = append(staff, row.StaffID)
		}
		require.ElementsMatch(t, []int64{lower.ID, higher.ID}, staff, "both supervisors reach the mirrored block")
		return nil
	}))
}
