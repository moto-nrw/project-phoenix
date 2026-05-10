package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// dropWorkSessionsSourceColumn rolls back the schema portion of migration
// 1.15.49 so a test can re-run the up against a controlled pre-migration
// state. SetupTestDB has already run every migration including 1.15.49, so
// the column exists at the start of every test in this file; the deferred
// cleanup branch puts it back via workSessionsSourceUp.
func dropWorkSessionsSourceColumn(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.work_sessions
		DROP CONSTRAINT IF EXISTS chk_work_sessions_source;
		ALTER TABLE active.work_sessions DROP COLUMN IF EXISTS source;
	`)
	require.NoError(t, err, "drop work_sessions.source column")
}

// insertWorkSession stages a work_sessions row with a specific check_in_time
// so the heuristic in workSessionsSourceUp can be exercised deterministically.
// created_by/updated_by point at the staff member themselves, matching what
// WorkSessionService.CheckIn writes in production.
func insertWorkSession(t *testing.T, db *bun.DB, tenantID, staffID int64, checkInTime time.Time) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO active.work_sessions
		    (tenant_id, staff_id, date, status, check_in_time, created_by, updated_by)
		VALUES (?, ?, ?::date, 'present', ?, ?, ?)
		RETURNING id;
	`, tenantID, staffID, checkInTime, checkInTime, staffID, staffID).Scan(ctx, &id)
	require.NoError(t, err, "insert work_session")
	return id
}

// insertGroupSupervisor stages a supervisor row whose created_at sits at a
// caller-chosen timestamp so the migration's ±2-second window can be
// triggered or missed deterministically. The associated active.groups row
// is created on the fly because group_supervisors.group_id is FK-enforced;
// active.groups.room_id is NOT NULL so a throwaway room is created too.
func insertGroupSupervisor(t *testing.T, db *bun.DB, tenantID, staffID int64, createdAt time.Time) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID,
		fmt.Sprintf("ws-source-test-room-%d", time.Now().UnixNano()))

	var groupID int64
	err := db.NewRaw(`
		INSERT INTO active.groups (tenant_id, room_id, start_time, last_activity, timeout_minutes)
		VALUES (?, ?, ?, ?, 30)
		RETURNING id;
	`, tenantID, room.ID, createdAt, createdAt).Scan(ctx, &groupID)
	require.NoError(t, err, "insert active.group for supervisor")

	var id int64
	err = db.NewRaw(`
		INSERT INTO active.group_supervisors
		    (tenant_id, staff_id, group_id, role, start_date, created_at, updated_at)
		VALUES (?, ?, ?, 'Supervisor', ?::date, ?, ?)
		RETURNING id;
	`, tenantID, staffID, groupID, createdAt, createdAt, createdAt).Scan(ctx, &id)
	require.NoError(t, err, "insert group_supervisor")
	return id
}

func workSessionSource(t *testing.T, db *bun.DB, id int64) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var source string
	err := db.NewRaw(`SELECT source FROM active.work_sessions WHERE id = ?;`, id).Scan(ctx, &source)
	require.NoError(t, err, "read work_session.source for id=%d", id)
	return source
}

func cleanupWorkSessionsSourceTest(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stmts := []string{
		`DELETE FROM active.work_sessions WHERE tenant_id = ?`,
		`DELETE FROM active.group_supervisors WHERE tenant_id = ?`,
		`DELETE FROM active.groups WHERE tenant_id = ?`,
		`DELETE FROM facilities.rooms WHERE tenant_id = ?`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt, tenantID); err != nil {
			t.Logf("work_sessions source migration cleanup: %v", err)
		}
	}
}

// TestWorkSessionsSourceUp_HeuristicLabelsNFC verifies the core behaviour:
// a work_session whose check_in_time sits inside the (gs.created_at, +2s]
// window for the same staff/tenant must be labelled 'nfc'. This is the
// production signal for the IoT supervisor flow — supervisor row written
// first, work_session a few ms later.
func TestWorkSessionsSourceUp_HeuristicLabelsNFC(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9201
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "NFC", "Stamper")

	// Production timing: supervisorRepo.Create runs at T, then CheckIn writes
	// work_sessions.check_in_time at T + ~few ms. We seed +500ms to be safely
	// inside the 2-second window.
	gsTime := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)
	wsTime := gsTime.Add(500 * time.Millisecond)
	insertGroupSupervisor(t, db, tenantID, staff.ID, gsTime)
	wsID := insertWorkSession(t, db, tenantID, staff.ID, wsTime)

	dropWorkSessionsSourceColumn(t, db)
	require.NoError(t, workSessionsSourceUp(context.Background(), db))

	assert.Equal(t, "nfc", workSessionSource(t, db, wsID),
		"work_session created milliseconds after a group_supervisor row must be labelled 'nfc'")
}

// TestWorkSessionsSourceUp_HeuristicLabelsApp verifies that work_sessions
// without a nearby supervisor row are labelled 'app' — the App / Web check-in
// flow does not write group_supervisors, so this is the dominant historical
// case.
func TestWorkSessionsSourceUp_HeuristicLabelsApp(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9202
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "App", "Stamper")
	wsID := insertWorkSession(t, db, tenantID, staff.ID,
		time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC))

	dropWorkSessionsSourceColumn(t, db)
	require.NoError(t, workSessionsSourceUp(context.Background(), db))

	assert.Equal(t, "app", workSessionSource(t, db, wsID),
		"work_session without any group_supervisor row must be labelled 'app'")
}

// TestWorkSessionsSourceUp_AppCheckinBeforeLaterSupervisorStaysApp protects
// against the false-positive scenario where a staff member checks in via the
// App and then a few seconds later starts an NFC activity. Because the
// window is asymmetric (gs.created_at <= ws.check_in_time only), the App row
// must stay 'app'.
func TestWorkSessionsSourceUp_AppCheckinBeforeLaterSupervisorStaysApp(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9203
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "App", "First")

	// App check-in at T; NFC activity start 3 seconds later. EnsureCheckedIn
	// in prod would no-op (already checked in), but the supervisor row still
	// gets written. The asymmetric window must not match.
	wsTime := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)
	wsID := insertWorkSession(t, db, tenantID, staff.ID, wsTime)
	insertGroupSupervisor(t, db, tenantID, staff.ID, wsTime.Add(3*time.Second))

	dropWorkSessionsSourceColumn(t, db)
	require.NoError(t, workSessionsSourceUp(context.Background(), db))

	assert.Equal(t, "app", workSessionSource(t, db, wsID),
		"supervisor row written AFTER the work_session must not cause an 'nfc' relabel")
}

// TestWorkSessionsSourceUp_TenantIsolation guards against cross-tenant
// matches. Two tenants happen to share the same staff_id (impossible today
// but defended in the heuristic via gs.tenant_id = ws.tenant_id) — the
// supervisor row from tenant A must not relabel the work_session in tenant B.
func TestWorkSessionsSourceUp_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantA int64 = 9204
	const tenantB int64 = 9205
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	defer testpkg.CleanupTenantTestData(t, db, tenantA, tenantB)
	defer cleanupWorkSessionsSourceTest(t, db, tenantA)
	defer cleanupWorkSessionsSourceTest(t, db, tenantB)

	// Different staff in each tenant, but choose timestamps that would match
	// the heuristic if tenant_id were not part of the join.
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantA, "Cross", "TenantA")
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantB, "Cross", "TenantB")

	moment := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)
	// Supervisor row in tenant A, work_session in tenant B at the same moment.
	insertGroupSupervisor(t, db, tenantA, staffA.ID, moment)
	wsB := insertWorkSession(t, db, tenantB, staffB.ID, moment.Add(500*time.Millisecond))

	dropWorkSessionsSourceColumn(t, db)
	require.NoError(t, workSessionsSourceUp(context.Background(), db))

	assert.Equal(t, "app", workSessionSource(t, db, wsB),
		"supervisor row in another tenant must not leak across the heuristic join")
}

// TestWorkSessionsSourceUp_ConstraintRejectsBadValue verifies the CHECK
// constraint is in place after up() runs, so any future code path that
// bypasses WorkSessionService.CheckIn cannot smuggle a bogus channel into
// the table.
func TestWorkSessionsSourceUp_ConstraintRejectsBadValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9206
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Constraint", "Test")
	wsTime := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)

	// Re-running up() on the existing schema is idempotent (column already
	// exists with IF NOT EXISTS), so we don't need to drop/re-add to assert
	// the constraint is live.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO active.work_sessions
		    (tenant_id, staff_id, date, status, source, check_in_time, created_by, updated_by)
		VALUES (%d, %d, '2026-03-04', 'present', 'bogus', '2026-03-04 08:00:00+00', %d, %d);
	`, tenantID, staff.ID, staff.ID, staff.ID))
	require.Error(t, err, "CHECK constraint must reject unknown source values")
	assert.Contains(t, err.Error(), "chk_work_sessions_source",
		"error must name the constraint so the cause is obvious in logs")

	_ = wsTime
}
