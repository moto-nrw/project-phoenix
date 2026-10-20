package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropWorkSessionsSourceColumn rolls back the schema portion of migration
// 1.15.54 so a test can re-run the up against a controlled pre-migration
// state. The package runs every test on its own database clone, so no other
// test sees the dropped column.
func dropWorkSessionsSourceColumn(t *testing.T, db *testpkg.DB) {
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

// insertWorkSession stages a work_sessions row to verify that the migration's
// backfill labels pre-existing rows as 'unknown'. created_by/updated_by point
// at the staff member themselves, matching what WorkSessionService.CheckIn
// writes in production.
func insertWorkSession(t *testing.T, db *testpkg.DB, tenantID, staffID int64, checkInTime time.Time) int64 {
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

func workSessionSource(t *testing.T, db *testpkg.DB, id int64) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var source string
	err := db.NewRaw(`SELECT source FROM active.work_sessions WHERE id = ?;`, id).Scan(ctx, &source)
	require.NoError(t, err, "read work_session.source for id=%d", id)
	return source
}

// TestWorkSessionsSourceUp_LabelsLegacyRowsUnknown verifies the backfill:
// a row that exists before the migration runs (no source value) must end up
// labelled 'unknown'. This is the entire backfill contract — no heuristic,
// no guessing, just an honest sentinel for pre-existing data.
func TestWorkSessionsSourceUp_LabelsLegacyRowsUnknown(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Legacy", "Stamper")

	dropWorkSessionsSourceColumn(t, db)

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
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

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
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

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
