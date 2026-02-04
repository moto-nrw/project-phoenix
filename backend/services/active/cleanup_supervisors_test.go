// Package active_test tests the supervisor cleanup using the hermetic testing pattern.
//
// Tests create their own fixtures, execute operations, and clean up afterward.
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// utcToday returns today's date (Berlin calendar day) at midnight UTC,
// matching the cleanup service's timezone.TodayUTC() approach.
func utcToday() time.Time {
	return timezone.TodayUTC()
}

// =============================================================================
// CleanupStaleSupervisors Tests
// =============================================================================

// TestCleanupStaleSupervisors_NoStaleRecords tests that cleanup works correctly
// when there are no stale supervisor records to clean up.
func TestCleanupStaleSupervisors_NoStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Run cleanup when there are no stale records
	result, err := cleanupService.CleanupStaleSupervisors(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 0)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
}

// TestCleanupStaleSupervisors_ClosesYesterdayRecords tests that stale supervisor
// records from previous days without end_date are properly closed.
func TestCleanupStaleSupervisors_ClosesYesterdayRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Stale", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "Stale Supervisor Activity")
	room := testpkg.CreateTestRoom(t, db, "Stale Supervisor Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	// Create a stale supervisor record (yesterday, no end_date)
	yesterday := utcToday().AddDate(0, 0, -1)
	var supervisorID int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, staff.ID, activeGroup.ID, "supervisor", yesterday).Scan(ctx, &supervisorID)
	require.NoError(t, err, "Failed to create stale supervisor record")

	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisorID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, activity.ID, room.ID)
	}()

	// ACT: Run cleanup
	result, err := cleanupService.CleanupStaleSupervisors(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 1, "Should have closed at least 1 stale record")
	assert.GreaterOrEqual(t, result.StaffAffected, 1, "Should have affected at least 1 staff member")

	// Verify the record now has end_date set to the same date as start_date
	// (end_date is a DATE column, so only the date portion matters)
	var endDate *time.Time
	err = db.NewSelect().
		Table("active.group_supervisors").
		Column("end_date").
		Where("id = ?", supervisorID).
		Scan(ctx, &endDate)
	require.NoError(t, err)
	require.NotNil(t, endDate, "end_date should be set after cleanup")
	assert.Equal(t, yesterday.Year(), endDate.Year())
	assert.Equal(t, yesterday.Month(), endDate.Month())
	assert.Equal(t, yesterday.Day(), endDate.Day())
}

// TestCleanupStaleSupervisors_IgnoresTodayRecords tests that supervisor records
// from today (with no end_date) are NOT closed — they are still active.
func TestCleanupStaleSupervisors_IgnoresTodayRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Today", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "Today Supervisor Activity")
	room := testpkg.CreateTestRoom(t, db, "Today Supervisor Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	// Create a supervisor record from today (should NOT be cleaned)
	today := utcToday()
	var supervisorID int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, staff.ID, activeGroup.ID, "supervisor", today).Scan(ctx, &supervisorID)
	require.NoError(t, err, "Failed to create today's supervisor record")

	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisorID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, activity.ID, room.ID)
	}()

	// ACT: Run cleanup
	_, err = cleanupService.CleanupStaleSupervisors(ctx)
	require.NoError(t, err)

	// ASSERT: Today's record should still have end_date = NULL
	var endDate *time.Time
	err = db.NewSelect().
		Table("active.group_supervisors").
		Column("end_date").
		Where("id = ?", supervisorID).
		Scan(ctx, &endDate)
	require.NoError(t, err)
	assert.Nil(t, endDate, "Today's supervisor record should NOT have end_date set")
}

// TestCleanupStaleSupervisors_SucceedsEvenWithAuditError tests that cleanup
// succeeds even if the audit record creation fails (non-blocking error).
// Note: The audit.data_deletions.Validate() only accepts known deletion types
// (visit_retention, manual, gdpr_request). The "supervisor_cleanup" type is logged
// as an error in result.Errors but does not prevent the cleanup from completing.
func TestCleanupStaleSupervisors_SucceedsEvenWithAuditError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures with a stale record
	staff := testpkg.CreateTestStaff(t, db, "Audit", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "Audit Supervisor Activity")
	room := testpkg.CreateTestRoom(t, db, "Audit Supervisor Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	yesterday := utcToday().AddDate(0, 0, -1)
	var supervisorID int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, staff.ID, activeGroup.ID, "supervisor", yesterday).Scan(ctx, &supervisorID)
	require.NoError(t, err)

	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisorID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, activity.ID, room.ID)
	}()

	// ACT: Run cleanup
	result, err := cleanupService.CleanupStaleSupervisors(ctx)

	// ASSERT: Cleanup succeeds (records are closed)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.RecordsClosed, 1, "Should have closed at least 1 stale record")
	assert.GreaterOrEqual(t, result.StaffAffected, 1)

	// The audit log creation will fail because "supervisor_cleanup" is not a recognized
	// deletion type in DataDeletion.Validate(). This is the same behavior as
	// "attendance_cleanup". The error is captured in result.Errors but doesn't block cleanup.
	// We verify the record was actually closed despite the audit failure.
	var endDate *time.Time
	err = db.NewSelect().
		Table("active.group_supervisors").
		Column("end_date").
		Where("id = ?", supervisorID).
		Scan(ctx, &endDate)
	require.NoError(t, err)
	assert.NotNil(t, endDate, "Stale record should have end_date set despite audit error")
}

// =============================================================================
// PreviewSupervisorCleanup Tests
// =============================================================================

// TestPreviewSupervisorCleanup tests that preview correctly identifies stale
// supervisor records without modifying data.
func TestPreviewSupervisorCleanup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures with a stale record
	staff := testpkg.CreateTestStaff(t, db, "Preview", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "Preview Supervisor Activity")
	room := testpkg.CreateTestRoom(t, db, "Preview Supervisor Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	twoDaysAgo := utcToday().AddDate(0, 0, -2)
	var supervisorID int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, staff.ID, activeGroup.ID, "supervisor", twoDaysAgo).Scan(ctx, &supervisorID)
	require.NoError(t, err)

	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisorID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, activity.ID, room.ID)
	}()

	// ACT: Preview cleanup
	preview, err := cleanupService.PreviewSupervisorCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	assert.GreaterOrEqual(t, preview.TotalRecords, 1, "Should show at least 1 stale record")
	assert.NotNil(t, preview.OldestRecord)

	// Verify the record was NOT modified (still has end_date = NULL)
	var endDate *time.Time
	err = db.NewSelect().
		Table("active.group_supervisors").
		Column("end_date").
		Where("id = ?", supervisorID).
		Scan(ctx, &endDate)
	require.NoError(t, err)
	assert.Nil(t, endDate, "Preview should NOT modify records")
}
