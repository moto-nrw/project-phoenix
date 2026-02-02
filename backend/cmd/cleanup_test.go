package cmd

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTestCleanupContext creates a cleanupContext with test database
func setupTestCleanupContext(t *testing.T) *cleanupContext {
	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db)
	cleanupSvc := active.NewCleanupService(
		repoFactory.ActiveVisit,
		repoFactory.PrivacyConsent,
		repoFactory.DataDeletion,
		db,
	)
	return &cleanupContext{
		DB:             db,
		RepoFactory:    repoFactory,
		CleanupService: cleanupSvc,
	}
}

// setupTestCleanupContextWithServices creates a cleanupContext with ServiceFactory
func setupTestCleanupContextWithServices(t *testing.T) *cleanupContext {
	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	cleanupSvc := active.NewCleanupService(
		repoFactory.ActiveVisit,
		repoFactory.PrivacyConsent,
		repoFactory.DataDeletion,
		db,
	)
	return &cleanupContext{
		DB:             db,
		RepoFactory:    repoFactory,
		ServiceFactory: serviceFactory,
		CleanupService: cleanupSvc,
	}
}

// =============================================================================
// Category A: Pure Print/Format Functions
// =============================================================================

func TestGetStatusString(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		expected string
	}{
		{
			name:     "success true returns SUCCESS",
			success:  true,
			expected: "SUCCESS",
		},
		{
			name:     "success false returns COMPLETED WITH ERRORS",
			success:  false,
			expected: "COMPLETED WITH ERRORS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStatusString(tt.success)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogVisitCleanupResult_NoErrors(t *testing.T) {
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	result := &active.CleanupResult{
		StartedAt:         time.Now(),
		CompletedAt:       time.Now().Add(time.Second),
		StudentsProcessed: 10,
		RecordsDeleted:    50,
		Success:           true,
		Errors:            []active.CleanupError{},
	}

	logVisitCleanupResult(logger, result)

	output := logBuf.String()
	assert.Contains(t, output, "Cleanup completed in")
	assert.Contains(t, output, "Students processed: 10")
	assert.Contains(t, output, "Records deleted: 50")
	assert.NotContains(t, output, "Errors encountered")
}

func TestLogVisitCleanupResult_WithErrors_NotVerbose(t *testing.T) {
	// Ensure verbose is false
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	result := &active.CleanupResult{
		StartedAt:         time.Now(),
		CompletedAt:       time.Now().Add(time.Second),
		StudentsProcessed: 5,
		RecordsDeleted:    10,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 1, Error: "error 1", Timestamp: time.Now()},
			{StudentID: 2, Error: "error 2", Timestamp: time.Now()},
		},
	}

	logVisitCleanupResult(logger, result)

	output := logBuf.String()
	assert.Contains(t, output, "Errors encountered: 2")
	assert.NotContains(t, output, "Student 1")
	assert.NotContains(t, output, "Student 2")
}

func TestLogVisitCleanupResult_WithErrors_Verbose(t *testing.T) {
	// Set verbose to true
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	result := &active.CleanupResult{
		StartedAt:         time.Now(),
		CompletedAt:       time.Now().Add(time.Second),
		StudentsProcessed: 5,
		RecordsDeleted:    10,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 1, Error: "error 1", Timestamp: time.Now()},
			{StudentID: 2, Error: "error 2", Timestamp: time.Now()},
		},
	}

	logVisitCleanupResult(logger, result)

	output := logBuf.String()
	assert.Contains(t, output, "Errors encountered: 2")
	assert.Contains(t, output, "Student 1: error 1")
	assert.Contains(t, output, "Student 2: error 2")
}

func TestPrintVisitCleanupSummary_Success(t *testing.T) {
	result := &active.CleanupResult{
		StartedAt:         time.Now(),
		CompletedAt:       time.Now().Add(2 * time.Second),
		StudentsProcessed: 15,
		RecordsDeleted:    75,
		Success:           true,
		Errors:            []active.CleanupError{},
	}

	output := captureStdout(t, func() {
		printVisitCleanupSummary(result)
	})

	assert.Contains(t, output, "Cleanup Summary:")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Students processed: 15")
	assert.Contains(t, output, "Records deleted: 75")
	assert.Contains(t, output, "Status: SUCCESS")
	assert.NotContains(t, output, "Errors:")
}

func TestPrintVisitCleanupSummary_WithErrors(t *testing.T) {
	result := &active.CleanupResult{
		StartedAt:         time.Now(),
		CompletedAt:       time.Now().Add(time.Second),
		StudentsProcessed: 5,
		RecordsDeleted:    10,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 1, Error: "error 1", Timestamp: time.Now()},
		},
	}

	output := captureStdout(t, func() {
		printVisitCleanupSummary(result)
	})

	assert.Contains(t, output, "Status: COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 1")
}

func TestPrintPreviewHeader_WithOldestVisit(t *testing.T) {
	oldestVisit := time.Now().Add(-48 * time.Hour)
	preview := &active.CleanupPreview{
		StudentVisitCounts: map[int64]int{1: 5, 2: 10},
		TotalVisits:        15,
		OldestVisit:        &oldestVisit,
	}

	output := captureStdout(t, func() {
		printPreviewHeader(preview)
	})

	assert.Contains(t, output, "Data Retention Cleanup Preview")
	assert.Contains(t, output, "Total visits to delete: 15")
	assert.Contains(t, output, "Oldest visit:")
	assert.Contains(t, output, "days ago")
	assert.Contains(t, output, "Students affected: 2")
}

func TestPrintPreviewHeader_NoOldestVisit(t *testing.T) {
	preview := &active.CleanupPreview{
		StudentVisitCounts: map[int64]int{1: 5},
		TotalVisits:        5,
		OldestVisit:        nil,
	}

	output := captureStdout(t, func() {
		printPreviewHeader(preview)
	})

	assert.Contains(t, output, "Data Retention Cleanup Preview")
	assert.Contains(t, output, "Total visits to delete: 5")
	assert.NotContains(t, output, "Oldest visit:")
	assert.Contains(t, output, "Students affected: 1")
}

func TestPrintRetentionStats_WithOldestExpiredVisit(t *testing.T) {
	oldestExpired := time.Now().Add(-30 * 24 * time.Hour)
	stats := &active.RetentionStats{
		TotalExpiredVisits: 100,
		StudentsAffected:   10,
		OldestExpiredVisit: &oldestExpired,
	}

	output := captureStdout(t, func() {
		printRetentionStats(stats)
	})

	assert.Contains(t, output, "Data Retention Statistics")
	assert.Contains(t, output, "Total expired visits: 100")
	assert.Contains(t, output, "Students affected: 10")
	assert.Contains(t, output, "Oldest expired visit:")
	assert.Contains(t, output, "days ago")
}

func TestPrintRetentionStats_NoOldestExpiredVisit(t *testing.T) {
	stats := &active.RetentionStats{
		TotalExpiredVisits: 50,
		StudentsAffected:   5,
		OldestExpiredVisit: nil,
	}

	output := captureStdout(t, func() {
		printRetentionStats(stats)
	})

	assert.Contains(t, output, "Data Retention Statistics")
	assert.Contains(t, output, "Total expired visits: 50")
	assert.Contains(t, output, "Students affected: 5")
	assert.NotContains(t, output, "Oldest expired visit:")
}

func TestPrintAttendancePreviewHeader_WithOldestRecord(t *testing.T) {
	oldestRecord := time.Now().Add(-24 * time.Hour)
	preview := &active.AttendanceCleanupPreview{
		TotalRecords:   20,
		StudentRecords: map[int64]int{1: 10, 2: 10},
		OldestRecord:   &oldestRecord,
	}

	output := captureStdout(t, func() {
		printAttendancePreviewHeader(preview)
	})

	assert.Contains(t, output, "Attendance Cleanup Preview:")
	assert.Contains(t, output, "Total stale records: 20")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "days ago")
	assert.Contains(t, output, "Students affected: 2")
}

func TestPrintAttendancePreviewHeader_NoOldestRecord(t *testing.T) {
	preview := &active.AttendanceCleanupPreview{
		TotalRecords:   10,
		StudentRecords: map[int64]int{1: 10},
		OldestRecord:   nil,
	}

	output := captureStdout(t, func() {
		printAttendancePreviewHeader(preview)
	})

	assert.Contains(t, output, "Attendance Cleanup Preview:")
	assert.Contains(t, output, "Total stale records: 10")
	assert.NotContains(t, output, "Oldest record:")
	assert.Contains(t, output, "Students affected: 1")
}

func TestPrintAttendanceCleanupSummary_Success_WithOldestRecordDate(t *testing.T) {
	oldestDate := time.Now().Add(-48 * time.Hour)
	result := &active.AttendanceCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    15,
		StudentsAffected: 5,
		OldestRecordDate: &oldestDate,
		Success:          true,
		Errors:           []string{},
	}

	output := captureStdout(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "Attendance Cleanup Summary:")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Records closed: 15")
	assert.Contains(t, output, "Students affected: 5")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "Status: SUCCESS")
	assert.NotContains(t, output, "Errors:")
}

func TestPrintAttendanceCleanupSummary_Success_NoOldestRecordDate(t *testing.T) {
	result := &active.AttendanceCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    10,
		StudentsAffected: 3,
		OldestRecordDate: nil,
		Success:          true,
		Errors:           []string{},
	}

	output := captureStdout(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "Attendance Cleanup Summary:")
	assert.Contains(t, output, "Records closed: 10")
	assert.NotContains(t, output, "Oldest record:")
	assert.Contains(t, output, "Status: SUCCESS")
}

func TestPrintAttendanceCleanupSummary_WithErrors(t *testing.T) {
	result := &active.AttendanceCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    5,
		StudentsAffected: 2,
		OldestRecordDate: nil,
		Success:          false,
		Errors:           []string{"error 1", "error 2"},
	}

	output := captureStdout(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "Status: COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 2")
}

func TestPrintErrorList_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		printErrorList([]string{})
	})

	assert.Empty(t, output)
}

func TestPrintErrorList_EmptySlice_Nil(t *testing.T) {
	output := captureStdout(t, func() {
		printErrorList(nil)
	})

	assert.Empty(t, output)
}

func TestPrintErrorList_NotVerbose(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	errors := []string{"error 1", "error 2", "error 3"}

	output := captureStdout(t, func() {
		printErrorList(errors)
	})

	assert.Contains(t, output, "Errors: 3")
	assert.NotContains(t, output, "error 1")
	assert.NotContains(t, output, "error 2")
}

func TestPrintErrorList_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	errors := []string{"error 1", "error 2"}

	output := captureStdout(t, func() {
		printErrorList(errors)
	})

	assert.Contains(t, output, "Errors: 2")
	assert.Contains(t, output, "error 1")
	assert.Contains(t, output, "error 2")
}

func TestPrintAbandonedSessionSummary(t *testing.T) {
	threshold := 2 * time.Hour
	count := 5

	output := captureStdout(t, func() {
		printAbandonedSessionSummary(threshold, count)
	})

	assert.Contains(t, output, "Abandoned Session Cleanup Summary:")
	assert.Contains(t, output, "Threshold: 2h")
	assert.Contains(t, output, "Sessions cleaned: 5")
	assert.Contains(t, output, "Status: SUCCESS")
}

func TestPrintDailySessionSummary_Success(t *testing.T) {
	result := &active.DailySessionCleanupResult{
		SessionsEnded:    10,
		VisitsEnded:      50,
		SupervisorsEnded: 5,
		ExecutedAt:       time.Now(),
		Success:          true,
		Errors:           []string{},
	}

	output := captureStdout(t, func() {
		printDailySessionSummary(result)
	})

	assert.Contains(t, output, "Daily Session Cleanup Summary:")
	assert.Contains(t, output, "Sessions ended: 10")
	assert.Contains(t, output, "Visits ended: 50")
	assert.Contains(t, output, "Supervisors ended: 5")
	assert.Contains(t, output, "Status: SUCCESS")
	assert.NotContains(t, output, "Errors:")
}

func TestPrintDailySessionSummary_WithErrors(t *testing.T) {
	result := &active.DailySessionCleanupResult{
		SessionsEnded:    5,
		VisitsEnded:      20,
		SupervisorsEnded: 2,
		ExecutedAt:       time.Now(),
		Success:          false,
		Errors:           []string{"error 1", "error 2"},
	}

	output := captureStdout(t, func() {
		printDailySessionSummary(result)
	})

	assert.Contains(t, output, "Status: COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 2")
}

func TestPrintSupervisorPreviewHeader_WithOldestRecord(t *testing.T) {
	oldestRecord := time.Now().Add(-24 * time.Hour)
	preview := &active.SupervisorCleanupPreview{
		TotalRecords: 15,
		StaffRecords: map[int64]int{1: 10, 2: 5},
		OldestRecord: &oldestRecord,
	}

	output := captureStdout(t, func() {
		printSupervisorPreviewHeader(preview)
	})

	assert.Contains(t, output, "Supervisor Cleanup Preview:")
	assert.Contains(t, output, "Total stale records: 15")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "days ago")
	assert.Contains(t, output, "Staff affected: 2")
}

func TestPrintSupervisorPreviewHeader_NoOldestRecord(t *testing.T) {
	preview := &active.SupervisorCleanupPreview{
		TotalRecords: 10,
		StaffRecords: map[int64]int{1: 10},
		OldestRecord: nil,
	}

	output := captureStdout(t, func() {
		printSupervisorPreviewHeader(preview)
	})

	assert.Contains(t, output, "Supervisor Cleanup Preview:")
	assert.Contains(t, output, "Total stale records: 10")
	assert.NotContains(t, output, "Oldest record:")
	assert.Contains(t, output, "Staff affected: 1")
}

func TestPrintSupervisorCleanupSummary_Success_WithOldestRecordDate(t *testing.T) {
	oldestDate := time.Now().Add(-48 * time.Hour)
	result := &active.SupervisorCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    20,
		StaffAffected:    4,
		OldestRecordDate: &oldestDate,
		Success:          true,
		Errors:           []string{},
	}

	output := captureStdout(t, func() {
		printSupervisorCleanupSummary(result)
	})

	assert.Contains(t, output, "Supervisor Cleanup Summary:")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Records closed: 20")
	assert.Contains(t, output, "Staff affected: 4")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "Status: SUCCESS")
	assert.NotContains(t, output, "Errors:")
}

func TestPrintSupervisorCleanupSummary_Success_NoOldestRecordDate(t *testing.T) {
	result := &active.SupervisorCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    10,
		StaffAffected:    2,
		OldestRecordDate: nil,
		Success:          true,
		Errors:           []string{},
	}

	output := captureStdout(t, func() {
		printSupervisorCleanupSummary(result)
	})

	assert.Contains(t, output, "Supervisor Cleanup Summary:")
	assert.Contains(t, output, "Records closed: 10")
	assert.NotContains(t, output, "Oldest record:")
	assert.Contains(t, output, "Status: SUCCESS")
}

func TestPrintSupervisorCleanupSummary_WithErrors(t *testing.T) {
	result := &active.SupervisorCleanupResult{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(time.Second),
		RecordsClosed:    5,
		StaffAffected:    1,
		OldestRecordDate: nil,
		Success:          false,
		Errors:           []string{"error 1"},
	}

	output := captureStdout(t, func() {
		printSupervisorCleanupSummary(result)
	})

	assert.Contains(t, output, "Status: COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 1")
}

func TestPrintStaffBreakdown_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		printStaffBreakdown("Test Header", "Count", map[int64]int{})
	})

	assert.Empty(t, output)
}

func TestPrintStaffBreakdown_WithData(t *testing.T) {
	data := map[int64]int{
		1: 10,
		2: 20,
		3: 5,
	}

	output := captureStdout(t, func() {
		printStaffBreakdown("Per-staff breakdown", "Stale Records", data)
	})

	assert.Contains(t, output, "Per-staff breakdown:")
	assert.Contains(t, output, "Staff ID")
	assert.Contains(t, output, "Stale Records")
	// Data is in a map, so we can't guarantee order, but we can check values are present
	assert.True(t, strings.Contains(output, "1") && strings.Contains(output, "10"))
	assert.True(t, strings.Contains(output, "2") && strings.Contains(output, "20"))
	assert.True(t, strings.Contains(output, "3") && strings.Contains(output, "5"))
}

// =============================================================================
// Category B: Command Handler Functions (Require Test DB)
// =============================================================================

func TestRunVisitsDryRun_NotVerbose(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	err := runVisitsDryRun(logger, ctx)
	require.NoError(t, err)

	// Check logger output
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "DRY RUN MODE")

	// Note: stdout output is captured during actual execution
	// Since we can't easily capture it here without more complex test setup,
	// we just verify no error occurred
}

func TestRunVisitsDryRun_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	err := runVisitsDryRun(logger, ctx)
	require.NoError(t, err)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "DRY RUN MODE")
}

func TestRunVisitsCleanup(t *testing.T) {
	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	// Should run without error even if no data to clean
	err := runVisitsCleanup(logger, ctx)
	require.NoError(t, err)
}

func TestRunAttendanceDryRun_NotVerbose(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runAttendanceDryRun(ctx)
	require.NoError(t, err)
}

func TestRunAttendanceDryRun_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runAttendanceDryRun(ctx)
	require.NoError(t, err)
}

func TestRunAttendanceCleanup(t *testing.T) {
	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runAttendanceCleanup(ctx)
	require.NoError(t, err)
}

func TestRunSupervisorsDryRun_NotVerbose(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runSupervisorsDryRun(ctx)
	require.NoError(t, err)
}

func TestRunSupervisorsDryRun_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runSupervisorsDryRun(ctx)
	require.NoError(t, err)
}

func TestRunSupervisorsCleanup(t *testing.T) {
	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	err := runSupervisorsCleanup(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Category C: Functions Needing ServiceFactory (New Tests)
// =============================================================================

func TestPrintVerboseRecentDeletions(t *testing.T) {
	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	// Should run without error even if no recent deletions exist
	output := captureStdout(t, func() {
		printVerboseRecentDeletions(ctx)
	})

	// Should at least print the header
	assert.Contains(t, output, "Recent deletion activity:")
}

func TestCountExpiredTokens(t *testing.T) {
	ctx := setupTestCleanupContext(t)
	defer ctx.Close()

	// Should return count (0 or more) without error
	count, err := countExpiredTokens(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
}

func TestRunAbandonedSessionCleanup_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	ctx := setupTestCleanupContextWithServices(t)
	defer ctx.Close()

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	threshold := 5 * time.Minute
	err := runAbandonedSessionCleanup(ctx, threshold)
	require.NoError(t, err)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "DRY RUN MODE")
	assert.Contains(t, logOutput, "5m")
}

func TestRunAbandonedSessionCleanup_Execute(t *testing.T) {
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	ctx := setupTestCleanupContextWithServices(t)
	defer ctx.Close()

	threshold := 5 * time.Minute

	output := captureStdout(t, func() {
		err := runAbandonedSessionCleanup(ctx, threshold)
		require.NoError(t, err)
	})

	// Should print summary
	assert.Contains(t, output, "Abandoned Session Cleanup Summary")
	assert.Contains(t, output, "Threshold:")
	assert.Contains(t, output, "Sessions cleaned:")
	assert.Contains(t, output, "Status: SUCCESS")
}

func TestRunDailySessionCleanup_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	ctx := setupTestCleanupContextWithServices(t)
	defer ctx.Close()

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	err := runDailySessionCleanup(ctx)
	require.NoError(t, err)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "DRY RUN MODE")
	assert.Contains(t, logOutput, "Would end all active sessions")
}

func TestRunDailySessionCleanup_Execute(t *testing.T) {
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	ctx := setupTestCleanupContextWithServices(t)
	defer ctx.Close()

	output := captureStdout(t, func() {
		err := runDailySessionCleanup(ctx)
		require.NoError(t, err)
	})

	// Should print summary
	assert.Contains(t, output, "Daily Session Cleanup Summary")
	assert.Contains(t, output, "Sessions ended:")
	assert.Contains(t, output, "Visits ended:")
	assert.Contains(t, output, "Supervisors ended:")
	assert.Contains(t, output, "Status:")
}

// =============================================================================
// Category D: Branch Coverage Tests for printStaffBreakdown
// =============================================================================

func TestPrintStaffBreakdown_Empty_AlreadyTested(t *testing.T) {
	// This test already exists as TestPrintStaffBreakdown_Empty in cleanup_test.go
	// It properly tests the empty data case (line 639 early return)
	output := captureStdout(t, func() {
		printStaffBreakdown("Test Header", "Count", map[int64]int{})
	})

	assert.Empty(t, output)
}

func TestPrintStaffBreakdown_WithData_AlreadyTested(t *testing.T) {
	// This test already exists as TestPrintStaffBreakdown_WithData in cleanup_test.go
	// It properly tests the non-empty data case with table output
	data := map[int64]int{
		1: 10,
		2: 20,
		3: 5,
	}

	output := captureStdout(t, func() {
		printStaffBreakdown("Per-staff breakdown", "Stale Records", data)
	})

	assert.Contains(t, output, "Per-staff breakdown:")
	assert.Contains(t, output, "Staff ID")
	assert.Contains(t, output, "Stale Records")
	assert.True(t, strings.Contains(output, "1") && strings.Contains(output, "10"))
	assert.True(t, strings.Contains(output, "2") && strings.Contains(output, "20"))
	assert.True(t, strings.Contains(output, "3") && strings.Contains(output, "5"))
}

// =============================================================================
// Category E: Cobra Command Registration Tests (from test/improve-coverage)
// =============================================================================

func TestCleanupConstants(t *testing.T) {
	assert.Equal(t, "dry-run", flagDryRun)
	assert.Equal(t, "Show detailed information", flagDescShowDetails)
	assert.Equal(t, "Students affected: %d\n", fmtStudentsAffected)
	assert.Equal(t, "Status: %s\n", fmtStatus)
}

func TestCleanupCmd_Metadata(t *testing.T) {
	assert.Equal(t, "cleanup", cleanupCmd.Use)
	assert.Contains(t, cleanupCmd.Short, "Clean up expired data")
	assert.Contains(t, cleanupCmd.Long, "retention policies")
}

func TestCleanupCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "cleanup" {
			found = true
			break
		}
	}
	assert.True(t, found, "cleanupCmd should be registered on RootCmd")
}

func TestCleanupCmd_HasSubcommands(t *testing.T) {
	subcommands := cleanupCmd.Commands()
	names := make([]string, 0, len(subcommands))
	for _, cmd := range subcommands {
		names = append(names, cmd.Use)
	}

	assert.Contains(t, names, "visits")
	assert.Contains(t, names, "preview")
	assert.Contains(t, names, "stats")
	assert.Contains(t, names, "tokens")
	assert.Contains(t, names, "invitations")
	assert.Contains(t, names, "rate-limits")
	assert.Contains(t, names, "attendance")
	assert.Contains(t, names, "sessions")
	assert.Contains(t, names, "supervisors")
}

func TestCleanupCmd_SubcommandCount(t *testing.T) {
	assert.Len(t, cleanupCmd.Commands(), 9, "cleanupCmd should have exactly 9 subcommands")
}

func TestCleanupVisitsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "visits", cleanupVisitsCmd.Use)
	assert.Contains(t, cleanupVisitsCmd.Short, "expired visit records")
	assert.Contains(t, cleanupVisitsCmd.Long, "GDPR compliance")
	assert.NotNil(t, cleanupVisitsCmd.RunE)
}

func TestCleanupPreviewCmd_Metadata(t *testing.T) {
	assert.Equal(t, "preview", cleanupPreviewCmd.Use)
	assert.Contains(t, cleanupPreviewCmd.Short, "Preview")
	assert.NotNil(t, cleanupPreviewCmd.RunE)
}

func TestCleanupStatsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "stats", cleanupStatsCmd.Use)
	assert.Contains(t, cleanupStatsCmd.Short, "retention statistics")
	assert.NotNil(t, cleanupStatsCmd.RunE)
}

func TestCleanupTokensCmd_Metadata(t *testing.T) {
	assert.Equal(t, "tokens", cleanupTokensCmd.Use)
	assert.Contains(t, cleanupTokensCmd.Short, "expired authentication tokens")
	assert.NotNil(t, cleanupTokensCmd.RunE)
}

func TestCleanupInvitationsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "invitations", cleanupInvitationsCmd.Use)
	assert.Contains(t, cleanupInvitationsCmd.Short, "invitation tokens")
	assert.NotNil(t, cleanupInvitationsCmd.RunE)
}

func TestCleanupRateLimitsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "rate-limits", cleanupRateLimitsCmd.Use)
	assert.Contains(t, cleanupRateLimitsCmd.Short, "rate limit")
	assert.NotNil(t, cleanupRateLimitsCmd.RunE)
}

func TestCleanupAttendanceCmd_Metadata(t *testing.T) {
	assert.Equal(t, "attendance", cleanupAttendanceCmd.Use)
	assert.Contains(t, cleanupAttendanceCmd.Short, "stale attendance")
	assert.NotNil(t, cleanupAttendanceCmd.RunE)
}

func TestCleanupSessionsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "sessions", cleanupSessionsCmd.Use)
	assert.Contains(t, cleanupSessionsCmd.Short, "abandoned active sessions")
	assert.NotNil(t, cleanupSessionsCmd.RunE)
}

func TestCleanupSupervisorsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "supervisors", cleanupSupervisorsCmd.Use)
	assert.Contains(t, cleanupSupervisorsCmd.Short, "stale supervisor records")
	assert.NotNil(t, cleanupSupervisorsCmd.RunE)
}

// =============================================================================
// Category F: Flag Registration Tests (from test/improve-coverage)
// =============================================================================

func TestCleanupVisitsCmd_Flags(t *testing.T) {
	f := cleanupVisitsCmd.Flags()
	assert.NotNil(t, f.Lookup("dry-run"))
	assert.NotNil(t, f.Lookup("verbose"))
	assert.NotNil(t, f.Lookup("log-file"))
	assert.NotNil(t, f.Lookup("batch-size"))
}

func TestCleanupPreviewCmd_Flags(t *testing.T) {
	f := cleanupPreviewCmd.Flags()
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestCleanupStatsCmd_Flags(t *testing.T) {
	f := cleanupStatsCmd.Flags()
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestCleanupAttendanceCmd_Flags(t *testing.T) {
	f := cleanupAttendanceCmd.Flags()
	assert.NotNil(t, f.Lookup("dry-run"))
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestCleanupSessionsCmd_Flags(t *testing.T) {
	f := cleanupSessionsCmd.Flags()
	assert.NotNil(t, f.Lookup("dry-run"))
	assert.NotNil(t, f.Lookup("verbose"))
	assert.NotNil(t, f.Lookup("mode"))
	assert.NotNil(t, f.Lookup("threshold"))
}

func TestCleanupSupervisorsCmd_Flags(t *testing.T) {
	f := cleanupSupervisorsCmd.Flags()
	assert.NotNil(t, f.Lookup("dry-run"))
	assert.NotNil(t, f.Lookup("verbose"))
}

// =============================================================================
// Category G: Parent-Child and Usage Tests (from test/improve-coverage)
// =============================================================================

func TestCleanupSubcommands_ParentRelationship(t *testing.T) {
	assert.Equal(t, cleanupCmd, cleanupVisitsCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupPreviewCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupStatsCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupTokensCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupInvitationsCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupRateLimitsCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupAttendanceCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupSessionsCmd.Parent())
	assert.Equal(t, cleanupCmd, cleanupSupervisorsCmd.Parent())
}

func TestCleanupCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cleanupCmd.SetOut(buf)
	cleanupCmd.SetErr(buf)

	err := cleanupCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "cleanup")
	assert.Contains(t, output, "Available Commands")
}

func TestCleanupVisitsCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cleanupVisitsCmd.SetOut(buf)
	cleanupVisitsCmd.SetErr(buf)

	err := cleanupVisitsCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "visits")
}

func TestCleanupSessionsCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cleanupSessionsCmd.SetOut(buf)
	cleanupSessionsCmd.SetErr(buf)

	err := cleanupSessionsCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "sessions")
}
