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

func TestRepairPickupScheduleTenantIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	const tenantID int64 = 2

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	staffID := createTenantStaff(t, db, tenantID)
	studentID := createTenantStudent(t, db, tenantID)
	defer cleanupPickupTenantRepairRows(t, db, staffID, studentID)

	var scheduleID, exceptionID, noteID int64
	withPickupTenantRepairConstraintsDisabled(t, db, func(ctx context.Context, tx bun.Tx) {
		scheduleID = insertPickupScheduleWithTenant(t, ctx, tx, 1, studentID, staffID)
		exceptionID = insertPickupExceptionWithTenant(t, ctx, tx, 1, studentID, staffID)
		noteID = insertPickupNoteWithTenant(t, ctx, tx, 1, studentID, staffID)
	})

	require.NoError(t, repairPickupScheduleTenantIDs(ctx, db))
	require.NoError(t, repairPickupScheduleTenantIDs(ctx, db), "repair migration must be idempotent")

	assertTenantID(t, db, "schedule.student_pickup_schedules", scheduleID, tenantID)
	assertTenantID(t, db, "schedule.student_pickup_exceptions", exceptionID, tenantID)
	assertTenantID(t, db, "schedule.student_pickup_notes", noteID, tenantID)
}

func TestRepairPickupScheduleTenantIDsRejectsCrossTenantCreator(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	const tenantID int64 = 2

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	studentID := createTenantStudent(t, db, tenantID)
	staff := testpkg.CreateTestStaff(t, db, "PickupRepair", "WrongTenant")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer cleanupPickupTenantRepairRows(t, db, 0, studentID)

	withPickupTenantRepairConstraintsDisabled(t, db, func(ctx context.Context, tx bun.Tx) {
		insertPickupScheduleWithTenant(t, ctx, tx, 1, studentID, staff.ID)
	})

	err := repairPickupScheduleTenantIDs(ctx, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by staff from a different tenant")
}

func TestRepairPickupScheduleTenantIDsRejectsUniqueConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	const tenantID int64 = 2

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	staffID := createTenantStaff(t, db, tenantID)
	studentID := createTenantStudent(t, db, tenantID)
	defer cleanupPickupTenantRepairRows(t, db, staffID, studentID)

	withPickupTenantRepairConstraintsDisabled(t, db, func(ctx context.Context, tx bun.Tx) {
		insertPickupScheduleWithTenant(t, ctx, tx, tenantID, studentID, staffID)
		insertPickupScheduleWithTenant(t, ctx, tx, 1, studentID, staffID)
	})

	err := repairPickupScheduleTenantIDs(ctx, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would conflict")
}

func withPickupTenantRepairConstraintsDisabled(t *testing.T, db *bun.DB, fn func(context.Context, bun.Tx)) {
	t.Helper()

	ctx := context.Background()
	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL session_replication_role = 'replica'`); err != nil {
			return err
		}
		fn(ctx, tx)
		return nil
	})
	require.NoError(t, err)
}

func createTenantStaff(t *testing.T, db *bun.DB, tenantID int64) int64 {
	t.Helper()

	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "PickupRepair", "Staff")
	var staffID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users.staff (tenant_id, person_id, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, person.ID).Scan(&staffID)
	require.NoError(t, err)

	return staffID
}

func createTenantStudent(t *testing.T, db *bun.DB, tenantID int64) int64 {
	t.Helper()

	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "PickupRepair", "Student")
	var studentID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users.students (tenant_id, person_id, school_class, created_at, updated_at)
		VALUES (?, ?, '1a', NOW(), NOW())
		RETURNING id
	`, tenantID, person.ID).Scan(&studentID)
	require.NoError(t, err)

	return studentID
}

func insertPickupScheduleWithTenant(t *testing.T, ctx context.Context, db bun.IDB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	err := db.NewRaw(`
		INSERT INTO schedule.student_pickup_schedules
			(tenant_id, student_id, weekday, pickup_time, created_by, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, studentID, time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC), staffID).Scan(ctx, &id)
	require.NoError(t, err)

	return id
}

func insertPickupExceptionWithTenant(t *testing.T, ctx context.Context, db bun.IDB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	err := db.NewRaw(`
		INSERT INTO schedule.student_pickup_exceptions
			(tenant_id, student_id, exception_date, pickup_time, reason, created_by, created_at, updated_at)
		VALUES (?, ?, CURRENT_DATE, ?, 'Repair test', ?, NOW(), NOW())
		RETURNING id
	`, tenantID, studentID, time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC), staffID).Scan(ctx, &id)
	require.NoError(t, err)

	return id
}

func insertPickupNoteWithTenant(t *testing.T, ctx context.Context, db bun.IDB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	err := db.NewRaw(`
		INSERT INTO schedule.student_pickup_notes
			(tenant_id, student_id, note_date, content, created_by, created_at, updated_at)
		VALUES (?, ?, CURRENT_DATE, 'Repair note', ?, NOW(), NOW())
		RETURNING id
	`, tenantID, studentID, staffID).Scan(ctx, &id)
	require.NoError(t, err)

	return id
}

func assertTenantID(t *testing.T, db *bun.DB, table string, id, expectedTenantID int64) {
	t.Helper()

	var tenantID int64
	err := db.QueryRowContext(context.Background(), "SELECT tenant_id FROM "+table+" WHERE id = ?", id).Scan(&tenantID)
	require.NoError(t, err)
	assert.Equal(t, expectedTenantID, tenantID)
}

func cleanupPickupTenantRepairRows(t *testing.T, db *bun.DB, staffID, studentID int64) {
	t.Helper()

	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_notes WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_schedules WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, studentID)
	if staffID > 0 {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staffID)
	}
}
