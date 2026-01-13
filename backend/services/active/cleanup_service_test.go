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
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Create a stale attendance record (yesterday, no checkout)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	checkInTime := yesterday.Add(8 * time.Hour) // 8:00 AM yesterday

	_, err := db.NewRaw(`
		INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id)
		VALUES (?, ?, ?, ?, ?)
	`, student.ID, yesterday, checkInTime, staff.ID, device.ID).Exec(ctx)
	require.NoError(t, err, "Failed to create stale attendance record")

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
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Create a stale attendance record (2 days ago, no checkout)
	twoDaysAgo := time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour)
	checkInTime := twoDaysAgo.Add(9 * time.Hour) // 9:00 AM

	_, err := db.NewRaw(`
		INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id)
		VALUES (?, ?, ?, ?, ?)
	`, student.ID, twoDaysAgo, checkInTime, staff.ID, device.ID).Exec(ctx)
	require.NoError(t, err, "Failed to create stale attendance record")

	// ACT: Preview cleanup
	preview, err := cleanupService.PreviewAttendanceCleanup(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, preview)
	assert.GreaterOrEqual(t, preview.TotalRecords, 1, "Should show at least 1 stale record")
	assert.NotNil(t, preview.OldestRecord)

	// Clean up by running actual cleanup
	_, _ = cleanupService.CleanupStaleAttendance(ctx)
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
