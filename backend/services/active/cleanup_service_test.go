// Package active_test tests the cleanup service using the hermetic testing pattern.
//
// # HERMETIC TEST PATTERN
//
// Tests create their own fixtures, execute operations, and clean up afterward.
// This approach eliminates dependencies on seed data and prevents test pollution.
//
// STRUCTURE: ARRANGE-ACT-ASSERT
//
//	ARRANGE: Create test fixtures (real database records)
//	  student := testpkg.CreateTestStudent(t, db, "First", "Last", "1a")
//	  defer testpkg.CleanupActivityFixtures(t, db, student.ID)
//
//	ACT: Perform the operation under test
//	  result, err := cleanupService.CleanupStaleAttendance(ctx)
//
//	ASSERT: Verify the results
//	  require.NoError(t, err)
//	  assert.Equal(t, 1, result.RecordsClosed)
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupCleanupService creates a cleanup service with real database connection
func setupCleanupService(t *testing.T, db *bun.DB) active.CleanupService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.ActiveCleanup
}

// =============================================================================
// CleanupStaleAttendance Tests
// =============================================================================

// TestCleanupStaleAttendance_NoStaleRecords tests that cleanup works correctly
// when there are no stale attendance records to clean up.
func TestCleanupStaleAttendance_NoStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Run cleanup when there are no stale records
	result, err := cleanupService.CleanupStaleAttendance(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 0) // May have cleaned other test data
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
}

// TestCleanupStaleAttendance_ClosesStaleRecords tests that stale attendance
// records from previous days without checkout times are properly closed.
func TestCleanupStaleAttendance_ClosesStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for stale attendance
	student := testpkg.CreateTestStudent(t, db, "Stale", "Attendance", "5a")
	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Staff")
	device := testpkg.CreateTestDevice(t, db, "cleanup-device-001")

	// Create a stale attendance record (yesterday, no checkout)
	yesterday := timezone.Today().AddDate(0, 0, -1)
	checkInTime := yesterday.Add(8 * time.Hour) // 8:00 AM yesterday

	var attendanceID int64
	err := db.NewRaw(`
		INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, yesterday, checkInTime, staff.ID, device.ID).Scan(ctx, &attendanceID)
	require.NoError(t, err, "Failed to create stale attendance record")

	// IMPORTANT: Clean up attendance FIRST (before student/staff/device due to FK constraints)
	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.attendance", attendanceID)
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)
	}()

	// ACT: Run cleanup
	result, err := cleanupService.CleanupStaleAttendance(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 1, "Should have closed at least 1 stale record")
	assert.GreaterOrEqual(t, result.StudentsAffected, 1, "Should have affected at least 1 student")
}

// =============================================================================
// PreviewAttendanceCleanup Tests
// =============================================================================

// TestPreviewAttendanceCleanup_NoStaleRecords tests preview when there are no
// stale attendance records.
func TestPreviewAttendanceCleanup_NoStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Preview cleanup when there are no stale records
	preview, err := cleanupService.PreviewAttendanceCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	// May have other test data, so just check structure
	assert.NotNil(t, preview.StudentRecords)
	assert.NotNil(t, preview.RecordsByDate)
}

// TestPreviewAttendanceCleanup_ShowsStaleRecords tests that preview correctly
// identifies stale attendance records that would be cleaned.
func TestPreviewAttendanceCleanup_ShowsStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for stale attendance
	student := testpkg.CreateTestStudent(t, db, "Preview", "Stale", "5b")
	staff := testpkg.CreateTestStaff(t, db, "Preview", "Staff")
	device := testpkg.CreateTestDevice(t, db, "preview-device-001")

	// Create a stale attendance record (2 days ago, no checkout)
	twoDaysAgo := timezone.Today().AddDate(0, 0, -2)
	checkInTime := twoDaysAgo.Add(9 * time.Hour) // 9:00 AM

	var attendanceID int64
	err := db.NewRaw(`
		INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, twoDaysAgo, checkInTime, staff.ID, device.ID).Scan(ctx, &attendanceID)
	require.NoError(t, err, "Failed to create stale attendance record")

	// IMPORTANT: Clean up attendance FIRST (before student/staff/device due to FK constraints)
	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.attendance", attendanceID)
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)
	}()

	// ACT: Preview cleanup
	preview, err := cleanupService.PreviewAttendanceCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	assert.GreaterOrEqual(t, preview.TotalRecords, 1, "Should show at least 1 stale record")
	assert.NotNil(t, preview.OldestRecord)
}

// =============================================================================
// GetRetentionStatistics Tests
// =============================================================================

// TestGetRetentionStatistics_EmptyDatabase tests statistics when there are
// no expired visits to report.
func TestGetRetentionStatistics_EmptyDatabase(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Get statistics when there may be no expired visits
	stats, err := cleanupService.GetRetentionStatistics(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.NotNil(t, stats.ExpiredVisitsByMonth)
}

// TestGetRetentionStatistics_WithData tests statistics when there are
// expired visits in the database.
func TestGetRetentionStatistics_WithData(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for expired visits
	student := testpkg.CreateTestStudent(t, db, "Stats", "Test", "6d")
	staff := testpkg.CreateTestStaff(t, db, "Stats", "Staff")
	device := testpkg.CreateTestDevice(t, db, "stats-device-001")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Stats Activity")
	room := testpkg.CreateTestRoom(t, db, "Stats Room")

	// Create privacy consent with 1-day retention
	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0)
	durationDays := 365
	retentionDays := 1

	_, err := db.NewInsert().
		TableExpr("users.privacy_consents").
		Model(&map[string]interface{}{
			"student_id":          student.ID,
			"policy_version":      "v1.0",
			"accepted":            true,
			"accepted_at":         now,
			"expires_at":          expiresAt,
			"duration_days":       durationDays,
			"renewal_required":    false,
			"data_retention_days": retentionDays,
			"created_at":          now,
		}).
		Exec(ctx)
	require.NoError(t, err, "Failed to create privacy consent")

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	// Create old completed visit (5 days ago, beyond 1-day retention)
	// IMPORTANT: Must set created_at to old time for cleanup queries to work
	oldTime := time.Now().AddDate(0, 0, -5)
	exitTime := oldTime.Add(1 * time.Hour)

	var visitID int64
	err = db.NewRaw(`
		INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, activeGroup.ID, oldTime, exitTime, oldTime, oldTime).Scan(ctx, &visitID)
	require.NoError(t, err, "Failed to create old visit")

	// IMPORTANT: Clean up in correct order (FK constraints)
	defer testpkg.CleanupActivityFixtures(t, db, visitID, activeGroup.ID, student.ID, staff.ID, device.ID, activityGroup.ID, room.ID)

	// ACT: Get statistics
	stats, err := cleanupService.GetRetentionStatistics(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Greater(t, stats.TotalExpiredVisits, int64(0), "Should have at least 1 expired visit")
	assert.Greater(t, stats.StudentsAffected, 0, "Should have at least 1 student affected")
	assert.NotNil(t, stats.OldestExpiredVisit, "Should have oldest expired visit timestamp")
	assert.NotNil(t, stats.ExpiredVisitsByMonth)
}

// =============================================================================
// PreviewCleanup Tests
// =============================================================================

// TestPreviewCleanup_EmptyDatabase tests preview when there are no expired visits.
func TestPreviewCleanup_EmptyDatabase(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Preview cleanup
	preview, err := cleanupService.PreviewCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	assert.NotNil(t, preview.StudentVisitCounts)
}

// =============================================================================
// CleanupExpiredVisits Tests
// =============================================================================

// TestCleanupExpiredVisits_NoExpiredVisits tests cleanup when there are no
// expired visits to delete.
func TestCleanupExpiredVisits_NoExpiredVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Run cleanup when there are no expired visits
	result, err := cleanupService.CleanupExpiredVisits(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.Empty(t, result.Errors)
}

// TestCleanupExpiredVisits_WithExpiredData tests cleanup when there are
// expired visits that should be deleted.
func TestCleanupExpiredVisits_WithExpiredData(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for expired visits
	student := testpkg.CreateTestStudent(t, db, "Batch", "Cleanup", "6c")
	staff := testpkg.CreateTestStaff(t, db, "Batch", "Staff")
	device := testpkg.CreateTestDevice(t, db, "batch-device-001")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Batch Activity")
	room := testpkg.CreateTestRoom(t, db, "Batch Room")

	// Create privacy consent with 1-day retention
	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0)
	durationDays := 365
	retentionDays := 1

	_, err := db.NewInsert().
		TableExpr("users.privacy_consents").
		Model(&map[string]interface{}{
			"student_id":          student.ID,
			"policy_version":      "v1.0",
			"accepted":            true,
			"accepted_at":         now,
			"expires_at":          expiresAt,
			"duration_days":       durationDays,
			"renewal_required":    false,
			"data_retention_days": retentionDays,
			"created_at":          now,
		}).
		Exec(ctx)
	require.NoError(t, err, "Failed to create privacy consent")

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	// Create old completed visit (5 days ago, beyond 1-day retention)
	// IMPORTANT: Must set created_at to old time for cleanup queries to work
	oldTime := time.Now().AddDate(0, 0, -5)
	exitTime := oldTime.Add(1 * time.Hour)

	var visitID int64
	err = db.NewRaw(`
		INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, activeGroup.ID, oldTime, exitTime, oldTime, oldTime).Scan(ctx, &visitID)
	require.NoError(t, err, "Failed to create old visit")

	// IMPORTANT: Clean up in correct order (FK constraints)
	defer testpkg.CleanupActivityFixtures(t, db, visitID, activeGroup.ID, student.ID, staff.ID, device.ID, activityGroup.ID, room.ID)

	// ACT: Run batch cleanup
	result, err := cleanupService.CleanupExpiredVisits(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Greater(t, result.RecordsDeleted, int64(0), "Should have deleted at least 1 expired visit")
	assert.Greater(t, result.StudentsProcessed, 0, "Should have processed at least 1 student")
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
}

// =============================================================================
// CleanupVisitsForStudent Tests
// =============================================================================

// TestCleanupVisitsForStudent_NoConsent tests cleanup for student without
// privacy consent (should use default 30 days).
func TestCleanupVisitsForStudent_NoConsent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student without privacy consent
	student := testpkg.CreateTestStudent(t, db, "NoConsent", "Student", "6a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// ACT: Run cleanup for student
	deleted, err := cleanupService.CleanupVisitsForStudent(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, int64(0), "Should not fail for student without consent")
}

// TestCleanupVisitsForStudent_WithConsent tests cleanup for student with
// explicit privacy consent.
func TestCleanupVisitsForStudent_WithConsent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student and add privacy consent
	consent := testpkg.CreateTestPrivacyConsent(t, db, "WithConsent")
	defer testpkg.CleanupActivityFixtures(t, db, consent.Student.ID)

	// ACT: Run cleanup for student with consent
	deleted, err := cleanupService.CleanupVisitsForStudent(ctx, consent.Student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, int64(0))
}

// TestCleanupVisitsForStudent_WithExpiredVisits tests cleanup when student has
// expired visits that should be deleted based on retention policy.
func TestCleanupVisitsForStudent_WithExpiredVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for expired visits
	student := testpkg.CreateTestStudent(t, db, "Expired", "Visits", "6b")
	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Staff")
	device := testpkg.CreateTestDevice(t, db, "cleanup-device-002")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Cleanup Activity")
	room := testpkg.CreateTestRoom(t, db, "Cleanup Room")

	// Create privacy consent with 1-day retention
	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0)
	durationDays := 365
	retentionDays := 1

	_, err := db.NewInsert().
		TableExpr("users.privacy_consents").
		Model(&map[string]interface{}{
			"student_id":          student.ID,
			"policy_version":      "v1.0",
			"accepted":            true,
			"accepted_at":         now,
			"expires_at":          expiresAt,
			"duration_days":       durationDays,
			"renewal_required":    false,
			"data_retention_days": retentionDays,
			"created_at":          now,
		}).
		Exec(ctx)
	require.NoError(t, err, "Failed to create privacy consent")

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	// Create old completed visit (5 days ago, beyond 1-day retention)
	// IMPORTANT: Must set created_at to old time for cleanup queries to work
	oldTime := time.Now().AddDate(0, 0, -5)
	exitTime := oldTime.Add(1 * time.Hour)

	var visitID int64
	err = db.NewRaw(`
		INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, activeGroup.ID, oldTime, exitTime, oldTime, oldTime).Scan(ctx, &visitID)
	require.NoError(t, err, "Failed to create old visit")

	// IMPORTANT: Clean up in correct order (FK constraints)
	defer testpkg.CleanupActivityFixtures(t, db, visitID, activeGroup.ID, student.ID, staff.ID, device.ID, activityGroup.ID, room.ID)

	// ACT: Run cleanup for student
	deleted, err := cleanupService.CleanupVisitsForStudent(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Greater(t, deleted, int64(0), "Should have deleted at least 1 expired visit")
}

// =============================================================================
// Database Error Handling Tests
// =============================================================================

// TestCleanupStaleAttendance_DatabaseError tests error handling when database
// operations fail.
func TestCleanupStaleAttendance_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Run cleanup with canceled context
	result, err := cleanupService.CleanupStaleAttendance(canceledCtx)

	// ASSERT: Should return error (result may or may not be nil depending on where error occurs)
	require.Error(t, err)
	// If result is returned, it should indicate failure
	if result != nil {
		assert.False(t, result.Success)
	}
}

// TestPreviewAttendanceCleanup_DatabaseError tests error handling when database
// operations fail.
func TestPreviewAttendanceCleanup_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Preview cleanup with canceled context
	preview, err := cleanupService.PreviewAttendanceCleanup(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.Nil(t, preview)
}

// TestGetRetentionStatistics_DatabaseError tests error handling when database
// operations fail.
func TestGetRetentionStatistics_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Get statistics with canceled context
	stats, err := cleanupService.GetRetentionStatistics(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.Nil(t, stats)
}

// TestPreviewCleanup_DatabaseError tests error handling when database
// operations fail.
func TestPreviewCleanup_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Preview cleanup with canceled context
	preview, err := cleanupService.PreviewCleanup(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.Nil(t, preview)
}

// TestCleanupExpiredVisits_DatabaseError tests error handling when getting
// students fails.
func TestCleanupExpiredVisits_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Run cleanup with canceled context
	result, err := cleanupService.CleanupExpiredVisits(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.NotNil(t, result) // Result is always returned with error details
	assert.False(t, result.Success)
}

// TestCleanupVisitsForStudent_DatabaseError tests error handling when getting
// privacy consent fails.
func TestCleanupVisitsForStudent_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Run cleanup with canceled context
	deleted, err := cleanupService.CleanupVisitsForStudent(canceledCtx, 1)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.Equal(t, int64(0), deleted)
}

// =============================================================================
// CleanupStaleAttendance Edge Cases
// =============================================================================

// TestCleanupStaleAttendance_CheckInAfterEndOfDay tests the edge case where
// check_in_time is after the end of the day (23:59:59), which indicates a
// data integrity issue. The service should use check_in_time + 1 second as
// the checkout time instead of 23:59:59.
func TestCleanupStaleAttendance_CheckInAfterEndOfDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures
	student := testpkg.CreateTestStudent(t, db, "Late", "CheckIn", "5c")
	staff := testpkg.CreateTestStaff(t, db, "Late", "Staff")
	device := testpkg.CreateTestDevice(t, db, "late-device-001")

	// Create a stale attendance record with check_in_time AFTER end-of-day
	// Set date to 2 days ago, but check_in_time to 2 days ago at 23:59:59 + 10 seconds
	// This simulates a data integrity issue where check-in happened after midnight
	twoDaysAgo := timezone.Today().AddDate(0, 0, -2)
	checkInTime := twoDaysAgo.Add(23*time.Hour + 59*time.Minute + 59*time.Second + 10*time.Second)

	var attendanceID int64
	err := db.NewRaw(`
		INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, student.ID, twoDaysAgo, checkInTime, staff.ID, device.ID).Scan(ctx, &attendanceID)
	require.NoError(t, err, "Failed to create attendance record with late check-in")

	// IMPORTANT: Clean up attendance FIRST (before student/staff/device due to FK constraints)
	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.attendance", attendanceID)
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)
	}()

	// ACT: Run cleanup
	result, err := cleanupService.CleanupStaleAttendance(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 1, "Should have closed at least 1 stale record")

	// Verify that check_out_time was set to check_in_time + 1 second (not 23:59:59)
	var checkOutTime time.Time
	err = db.NewRaw(`
		SELECT check_out_time
		FROM active.attendance
		WHERE id = ?
	`, attendanceID).Scan(ctx, &checkOutTime)
	require.NoError(t, err)
	assert.False(t, checkOutTime.IsZero(), "check_out_time should be set")

	// Should be exactly check_in_time + 1 second
	expectedCheckOut := checkInTime.Add(time.Second)
	assert.True(t, checkOutTime.Equal(expectedCheckOut),
		"check_out_time should be check_in_time + 1 second, got %v, expected %v",
		checkOutTime, expectedCheckOut)
}

// =============================================================================
// PreviewSupervisorCleanup Tests
// =============================================================================

// TestPreviewSupervisorCleanup_NoStaleRecords tests preview when there are no
// stale supervisor records.
func TestPreviewSupervisorCleanup_NoStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ACT: Preview cleanup when there are no stale supervisor records
	preview, err := cleanupService.PreviewSupervisorCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	// May have other test data, so just check structure
	assert.NotNil(t, preview.StaffRecords)
	assert.NotNil(t, preview.RecordsByDate)
	assert.GreaterOrEqual(t, preview.TotalRecords, 0)
}

// =============================================================================
// CleanupStaleSupervisors Tests
// =============================================================================

// TestCleanupStaleSupervisors_ClosesStaleRecords tests that stale supervisor
// records from previous days without end_date are properly closed.
func TestCleanupStaleSupervisors_ClosesStaleRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures for stale supervisor
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Supervisor Activity")
	room := testpkg.CreateTestRoom(t, db, "Supervisor Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	// Create a stale supervisor record (yesterday, no end_date)
	yesterday := time.Now().AddDate(0, 0, -1)

	var supervisorID int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, staff.ID, activeGroup.ID, "supervisor", yesterday).Scan(ctx, &supervisorID)
	require.NoError(t, err, "Failed to create stale supervisor record")

	// IMPORTANT: Clean up in correct order (FK constraints)
	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisorID)
		testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, staff.ID, activityGroup.ID, room.ID)
	}()

	// ACT: Run cleanup
	result, err := cleanupService.CleanupStaleSupervisors(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.RecordsClosed, 1, "Should have closed at least 1 stale supervisor record")
	assert.GreaterOrEqual(t, result.StaffAffected, 1, "Should have affected at least 1 staff member")
}

// =============================================================================
// Supervisor Cleanup Database Error Handling Tests
// =============================================================================

// TestCleanupStaleSupervisors_DatabaseError tests error handling when database
// operations fail.
func TestCleanupStaleSupervisors_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Run cleanup with canceled context
	result, err := cleanupService.CleanupStaleSupervisors(canceledCtx)

	// ASSERT: Should return error (result may or may not be nil depending on where error occurs)
	require.Error(t, err)
	// If result is returned, it should indicate failure
	if result != nil {
		assert.False(t, result.Success)
	}
}

// TestPreviewSupervisorCleanup_DatabaseError tests error handling when database
// operations fail.
func TestPreviewSupervisorCleanup_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupService := setupCleanupService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT: Preview cleanup with canceled context
	preview, err := cleanupService.PreviewSupervisorCleanup(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
	assert.Nil(t, preview)
}
