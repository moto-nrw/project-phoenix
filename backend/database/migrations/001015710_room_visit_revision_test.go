package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func roomRevision(t *testing.T, db *bun.DB, schoolID, roomID int64) int64 {
	t.Helper()
	var revision int64
	require.NoError(t, db.NewRaw(`SELECT COALESCE(sum(changes), 0) FROM active.visit_revision_events WHERE tenant_id = ? AND room_id = ?`,
		schoolID, roomID).Scan(t.Context(), &revision))
	return revision
}

// TestSchoolErasureRemovesRoomRevisionEvents pins the erasure path of
// 1.15.710 (#5205): the school's open session with an open visit gives the
// erasure a room event and a session-delete trigger, the residual check
// still finds nothing, and the other school keeps its room events.
func TestSchoolErasureRemovesRoomRevisionEvents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	open := func(schoolID int64) int64 {
		student := testpkg.CreateTestStudentForTenant(t, db, schoolID, "Room", "Erasure", "2b")
		group := testpkg.CreateTestActiveGroupForTenant(t, db, schoolID)
		_, err := db.ExecContext(ctx, `INSERT INTO active.visits (tenant_id, student_id, active_group_id, entry_time) VALUES (?, ?, ?, ?)`,
			schoolID, student.ID, group.ID, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE active.groups SET end_time = NOW() WHERE id = ?`, group.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE active.groups SET end_time = NULL WHERE id = ?`, group.ID)
		require.NoError(t, err)
		return group.RoomID
	}
	school := testpkg.UniqueTestTenantID(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	testpkg.EnsureTestTenant(t, db, other)
	schoolRoom := open(school)
	otherRoom := open(other)
	require.Equal(t, int64(3), roomRevision(t, db, school, schoolRoom), "the check-in, the end and the reopening")

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))

	require.Equal(t, "db_done", result.Status, "preflight, deletion and residual checks pass: %v", result.ErrorCode)
	assert.Zero(t, countRevisionEvents(t, db, school))
	assert.Equal(t, int64(3), roomRevision(t, db, other, otherRoom))
}

// TestRoomVisitRevisionMigrationRoundTrip pins that 1.15.710 runs twice,
// rolls back to the session-only events of 1.15.705 and applies again.
func TestRoomVisitRevisionMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	hasRoomScope := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'active' AND table_name = 'visit_revision_events' AND column_name = 'room_id')
			AND to_regprocedure('active.record_room_revision()') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Room", "RoundTrip", "2b")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, school)
	_, err := db.ExecContext(ctx, `INSERT INTO active.visits (tenant_id, student_id, active_group_id, entry_time) VALUES (?, ?, ?, ?)`,
		school, student.ID, group.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE active.groups SET end_time = NOW() WHERE id = ?`, group.ID)
	require.NoError(t, err)
	require.Equal(t, 2, countRevisionEvents(t, db, school), "one session event and one room event")

	require.NoError(t, roomVisitRevisionUp(ctx, db), "the migration is idempotent")
	require.NoError(t, roomVisitRevisionDown(ctx, db))
	assert.False(t, hasRoomScope())
	assert.Equal(t, 1, countRevisionEvents(t, db, school), "the session event stays, the room event goes")
	_, err = db.ExecContext(ctx, `UPDATE active.groups SET end_time = NULL WHERE id = ?`, group.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE active.visits SET exit_time = NOW() WHERE active_group_id = ?`, group.ID)
	require.NoError(t, err, "the restored 1.15.705 trigger writes session events")
	assert.Equal(t, 2, countRevisionEvents(t, db, school))
	require.NoError(t, roomVisitRevisionUp(ctx, db))
	assert.True(t, hasRoomScope())
}
