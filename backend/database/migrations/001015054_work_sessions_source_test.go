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
// 1.15.54 so a test can re-run the up against a controlled pre-migration
// state. SetupTestDB has already run every migration including 1.15.54, so
// the column exists at the start of every test in this file.
//
// The schema is shared across every test in the package (Postgres database
// scoped per process), so a panic or a require failure between the drop and
// the subsequent workSessionsSourceUp would leave sibling tests running
// against a column-less schema. To handle that we return a restore closure
// that down()s any leftover constraint/column and then up()s the migration
// — this works regardless of how the test exits (success, fail after drop,
// fail before drop) without relying on workSessionsSourceUp being
// self-idempotent (its ADD CONSTRAINT step is not).
//
// IMPORTANT: callers MUST register the returned closure with `defer` BEFORE
// any `defer db.Close()`, so LIFO order runs the restore against an open
// pool. Registering via t.Cleanup is wrong here because t.Cleanup fires
// AFTER the test function's deferred db.Close(), which would route the
// restore through a closed pool ("sql: database is closed") and leave the
// shared schema column-less for every subsequent test. See
// TestWorkSessionsSourceDown_DropsColumnAndConstraint for the same pattern.
func dropWorkSessionsSourceColumn(t *testing.T, db *bun.DB) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.work_sessions
		DROP CONSTRAINT IF EXISTS chk_work_sessions_source;
		ALTER TABLE active.work_sessions DROP COLUMN IF EXISTS source;
	`)
	require.NoError(t, err, "drop work_sessions.source column")

	return func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := workSessionsSourceDown(cleanupCtx, db); err != nil {
			t.Logf("rollback before schema restore failed: %v", err)
			return
		}
		if err := workSessionsSourceUp(cleanupCtx, db); err != nil {
			t.Logf("restoring active.work_sessions.source after test: %v", err)
		}
	}
}

// insertWorkSession stages a work_sessions row to verify that the migration's
// backfill labels pre-existing rows as 'unknown'. created_by/updated_by point
// at the staff member themselves, matching what WorkSessionService.CheckIn
// writes in production.
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
	if _, err := db.ExecContext(ctx,
		`DELETE FROM active.work_sessions WHERE tenant_id = ?`, tenantID); err != nil {
		t.Logf("work_sessions source migration cleanup: %v", err)
	}
}

// TestWorkSessionsSourceUp_LabelsLegacyRowsUnknown verifies the backfill:
// a row that exists before the migration runs (no source value) must end up
// labelled 'unknown'. This is the entire backfill contract — no heuristic,
// no guessing, just an honest sentinel for pre-existing data.
func TestWorkSessionsSourceUp_LabelsLegacyRowsUnknown(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9201
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Legacy", "Stamper")

	// Register restore via defer so LIFO runs it before the test function's
	// db.Close (line 104). t.Cleanup is wrong here — it fires AFTER deferred
	// calls return, by which point the pool is closed.
	restoreSchema := dropWorkSessionsSourceColumn(t, db)
	defer restoreSchema()

	wsID := insertWorkSession(t, db, tenantID, staff.ID,
		time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC))
	require.NoError(t, workSessionsSourceUp(context.Background(), db))

	assert.Equal(t, "unknown", workSessionSource(t, db, wsID),
		"row that pre-dates the migration must be labelled 'unknown'")
}

// TestWorkSessionsSourceUp_ConstraintAcceptsAllowedValues verifies that the
// CHECK constraint admits exactly the three documented values. New writes
// must still be able to land 'app' and 'nfc' from WorkSessionService.CheckIn,
// and the 'unknown' sentinel must remain valid because legacy rows carry it.
func TestWorkSessionsSourceUp_ConstraintAcceptsAllowedValues(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9202
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Constraint", "Test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i, src := range []string{"app", "nfc", "unknown"} {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO active.work_sessions
			    (tenant_id, staff_id, date, status, source, check_in_time, check_out_time, created_by, updated_by)
			VALUES (%d, %d, '2026-03-%02d', 'present', '%s', '2026-03-%02d 08:00:00+00', '2026-03-%02d 16:00:00+00', %d, %d);
		`, tenantID, staff.ID, i+10, src, i+10, i+10, staff.ID, staff.ID))
		assert.NoError(t, err, "CHECK constraint must accept source=%q", src)
	}
}

// TestWorkSessionsSourceUp_ConstraintRejectsBadValue verifies the CHECK
// constraint is in place after up() runs, so any future code path that
// bypasses WorkSessionService.CheckIn cannot smuggle a bogus channel into
// the table.
func TestWorkSessionsSourceUp_ConstraintRejectsBadValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 9203
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer cleanupWorkSessionsSourceTest(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Constraint", "Reject")

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
}

// TestWorkSessionsSourceDown_DropsColumnAndConstraint exercises the rollback
// path directly. Other tests in this file only invoke workSessionsSourceDown
// indirectly via the dropWorkSessionsSourceColumn cleanup, so a regression
// in Down (e.g. typo in the DROP COLUMN statement, missing IF EXISTS, wrong
// constraint name) would otherwise go undetected until a real rollback was
// attempted in production.
//
// We deliberately do not call dropWorkSessionsSourceColumn here — that helper
// drops the column via raw SQL bypassing Down, which is exactly the code we
// want to test. Instead we run Down on the schema as it stands after
// SetupTestDB (column + constraint present), assert both are gone, then
// re-run Up so sibling tests see the expected schema. A t.Cleanup also
// re-runs Up as a belt-and-braces net in case the test fails mid-flight.
func TestWorkSessionsSourceDown_DropsColumnAndConstraint(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	// Defers run LIFO: restore the column FIRST, then close the connection.
	// Using t.Cleanup here is wrong because Cleanup runs AFTER the test
	// function's deferred db.Close, which would leave the restore call
	// hitting a dead pool and the shared Postgres schema in a column-less
	// state for every sibling test that follows.
	defer func() { _ = db.Close() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := workSessionsSourceUp(ctx, db); err != nil {
			t.Logf("restoring active.work_sessions.source after TestDown: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Self-heal the schema before testing. A previous test crashing between
	// Down and Up — or a previous run of this very test failing to restore —
	// would leave the column missing, and workSessionsSourceUp's ADD
	// CONSTRAINT step is not idempotent (no IF NOT EXISTS). Run Down first
	// (best-effort, uses IF EXISTS) then Up so we always start the assertion
	// chain from a known fully-migrated state.
	if err := workSessionsSourceDown(ctx, db); err != nil {
		t.Logf("pre-test Down (best-effort): %v", err)
	}
	require.NoError(t, workSessionsSourceUp(ctx, db),
		"pre-test Up must succeed to establish the known starting state")

	var preColCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'active' AND table_name = 'work_sessions' AND column_name = 'source';
	`).Scan(ctx, &preColCount))
	require.Equal(t, 1, preColCount, "pre-condition: source column must exist before Down")

	require.NoError(t, workSessionsSourceDown(ctx, db),
		"Down on a fully-migrated schema must succeed")

	var postColCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'active' AND table_name = 'work_sessions' AND column_name = 'source';
	`).Scan(ctx, &postColCount))
	assert.Equal(t, 0, postColCount, "Down must drop the source column")

	var postConstraintCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE table_schema = 'active' AND table_name = 'work_sessions'
		  AND constraint_name = 'chk_work_sessions_source';
	`).Scan(ctx, &postConstraintCount))
	assert.Equal(t, 0, postConstraintCount, "Down must drop the CHECK constraint")

	// Idempotency: Down on a schema that already lacks the column must not
	// error — the DROP statements use IF EXISTS specifically to support
	// re-running the rollback after a partial up.
	require.NoError(t, workSessionsSourceDown(ctx, db),
		"Down must be idempotent (second call on missing column must succeed)")
}
