package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairPickupScheduleTenantIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	staffID := createTenantStaff(t, db, tenantID)
	studentID := createTenantStudent(t, db, tenantID)
	defer cleanupPickupTenantRepairRows(t, db, staffID, studentID)

	scheduleID := insertHistoricalPickupScheduleWithTenant(t, db, 1, studentID, staffID)
	exceptionID := insertHistoricalPickupExceptionWithTenant(t, db, 1, studentID, staffID)
	noteID := insertHistoricalPickupNoteWithTenant(t, db, 1, studentID, staffID)

	require.NoError(t, repairPickupScheduleTenantIDs(ctx, db))
	require.NoError(t, repairPickupScheduleTenantIDs(ctx, db), "repair migration must be idempotent")

	assertTenantID(t, db, "schedule.student_pickup_schedules", scheduleID, tenantID)
	assertTenantID(t, db, "schedule.student_pickup_exceptions", exceptionID, tenantID)
	assertTenantID(t, db, "schedule.student_pickup_notes", noteID, tenantID)
}

func TestRepairPickupScheduleTenantIDsRejectsCrossTenantCreator(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	studentID := createTenantStudent(t, db, tenantID)
	staff := testpkg.CreateTestStaff(t, db, "PickupRepair", "WrongTenant")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer cleanupPickupTenantRepairRows(t, db, 0, studentID)

	insertHistoricalPickupScheduleWithTenant(t, db, 1, studentID, staff.ID)

	err := repairPickupScheduleTenantIDs(ctx, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by staff from a different tenant")
}

func TestRepairPickupScheduleTenantIDsRejectsUniqueConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	staffID := createTenantStaff(t, db, tenantID)
	studentID := createTenantStudent(t, db, tenantID)
	defer cleanupPickupTenantRepairRows(t, db, staffID, studentID)

	insertPickupScheduleWithTenant(t, db, tenantID, studentID, staffID)
	insertHistoricalPickupScheduleWithTenant(t, db, 1, studentID, staffID)

	err := repairPickupScheduleTenantIDs(ctx, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would conflict")
}

func withReplicaTriggers(t *testing.T, db *testpkg.DB, fn func(tx testpkg.Tx)) {
	t.Helper()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)

	fn(tx)
	require.NoError(t, tx.Commit())
}

func createTenantStaff(t *testing.T, db *testpkg.DB, tenantID int64) int64 {
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

func createTenantStudent(t *testing.T, db *testpkg.DB, tenantID int64) int64 {
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

func insertPickupScheduleWithTenant(t *testing.T, db *testpkg.DB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO schedule.student_pickup_schedules
			(tenant_id, student_id, weekday, pickup_time, created_by, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, studentID, time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC), staffID).Scan(&id)
	require.NoError(t, err)

	return id
}

func insertHistoricalPickupScheduleWithTenant(t *testing.T, db *testpkg.DB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	withReplicaTriggers(t, db, func(tx testpkg.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			INSERT INTO schedule.student_pickup_schedules
				(tenant_id, student_id, weekday, pickup_time, created_by, created_at, updated_at)
			VALUES (?, ?, 1, ?, ?, NOW(), NOW())
			RETURNING id
		`, tenantID, studentID, time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC), staffID).Scan(&id)
		require.NoError(t, err)
	})

	return id
}

func insertHistoricalPickupExceptionWithTenant(t *testing.T, db *testpkg.DB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	withReplicaTriggers(t, db, func(tx testpkg.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			INSERT INTO schedule.student_pickup_exceptions
				(tenant_id, student_id, exception_date, pickup_time, reason, created_by, created_at, updated_at)
			VALUES (?, ?, CURRENT_DATE, ?, 'Repair test', ?, NOW(), NOW())
			RETURNING id
		`, tenantID, studentID, time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC), staffID).Scan(&id)
		require.NoError(t, err)
	})

	return id
}

func insertHistoricalPickupNoteWithTenant(t *testing.T, db *testpkg.DB, tenantID, studentID, staffID int64) int64 {
	t.Helper()

	var id int64
	withReplicaTriggers(t, db, func(tx testpkg.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			INSERT INTO schedule.student_pickup_notes
				(tenant_id, student_id, note_date, content, created_by, created_at, updated_at)
			VALUES (?, ?, CURRENT_DATE, 'Repair note', ?, NOW(), NOW())
			RETURNING id
		`, tenantID, studentID, staffID).Scan(&id)
		require.NoError(t, err)
	})

	return id
}

func assertTenantID(t *testing.T, db *testpkg.DB, table string, id, expectedTenantID int64) {
	t.Helper()

	var tenantID int64
	err := db.QueryRowContext(context.Background(), "SELECT tenant_id FROM "+table+" WHERE id = ?", id).Scan(&tenantID)
	require.NoError(t, err)
	assert.Equal(t, expectedTenantID, tenantID)
}

func cleanupPickupTenantRepairRows(t *testing.T, db *testpkg.DB, staffID, studentID int64) {
	t.Helper()

	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_notes WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_schedules WHERE student_id = ?`, studentID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, studentID)
	if staffID > 0 {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staffID)
	}
}
