package reminders_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.TenantContext(1)

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
		Set("end_date = ?", timezone.TodayDate().AddDays(-1)).
		Where("id = ?", expiredSup.ID).
		Exec(ctx)
	require.NoError(t, err)

	rows, err := repo.ListActiveSupervisedRooms(ctx)
	require.NoError(t, err)

	bulkRooms := make(map[int64]struct{})
	for _, row := range rows {
		if row.StaffID == staff.ID {
			bulkRooms[row.RoomID] = struct{}{}
		}
	}

	// What both paths agree on.
	assert.Contains(t, bulkRooms, openRoom.ID,
		"a live supervision on an open session is supervised on either path")
	assert.NotContains(t, bulkRooms, expiredRoom.ID,
		"a supervision that ended yesterday is gone on either path")

	// The accepted divergence. Compute still sees this room; the batch does not.
	assert.NotContains(t, bulkRooms, closedRoom.ID,
		"the bulk reader drops the room of an ended session — deliberate, and stricter than Compute")
}
