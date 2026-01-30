package cmd

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper: capture stdout output
// =============================================================================

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestCleanupConstants(t *testing.T) {
	assert.Equal(t, "dry-run", flagDryRun)
	assert.Equal(t, "Show detailed information", flagDescShowDetails)
	assert.Equal(t, "Students affected: %d\n", fmtStudentsAffected)
	assert.Equal(t, "Status: %s\n", fmtStatus)
}

// =============================================================================
// getStatusString Tests
// =============================================================================

func TestGetStatusString_Success(t *testing.T) {
	result := getStatusString(true)
	assert.Equal(t, "SUCCESS", result)
}

func TestGetStatusString_Failure(t *testing.T) {
	result := getStatusString(false)
	assert.Equal(t, "COMPLETED WITH ERRORS", result)
}

// =============================================================================
// Command Registration Tests
// =============================================================================

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
}

func TestCleanupCmd_SubcommandCount(t *testing.T) {
	assert.Len(t, cleanupCmd.Commands(), 8, "cleanupCmd should have exactly 8 subcommands")
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

// =============================================================================
// Flag Registration Tests
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

// =============================================================================
// Parent-Child Relationship Tests
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
}

// =============================================================================
// Print Function Tests (output verification)
// =============================================================================

func TestPrintVisitCleanupSummary(t *testing.T) {
	now := time.Now()
	result := &active.CleanupResult{
		StartedAt:         now.Add(-5 * time.Second),
		CompletedAt:       now,
		StudentsProcessed: 10,
		RecordsDeleted:    50,
		Success:           true,
		Errors:            nil,
	}

	output := captureOutput(t, func() {
		printVisitCleanupSummary(result)
	})

	assert.Contains(t, output, "Cleanup Summary")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Students processed: 10")
	assert.Contains(t, output, "Records deleted: 50")
	assert.Contains(t, output, "SUCCESS")
}

func TestPrintVisitCleanupSummary_WithErrors(t *testing.T) {
	now := time.Now()
	result := &active.CleanupResult{
		StartedAt:         now.Add(-5 * time.Second),
		CompletedAt:       now,
		StudentsProcessed: 10,
		RecordsDeleted:    30,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 1, Error: "some error"},
			{StudentID: 2, Error: "another error"},
		},
	}

	output := captureOutput(t, func() {
		printVisitCleanupSummary(result)
	})

	assert.Contains(t, output, "COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 2")
}

func TestPrintPreviewHeader(t *testing.T) {
	oldestVisit := time.Now().Add(-48 * time.Hour)
	preview := &active.CleanupPreview{
		TotalVisits: 100,
		OldestVisit: &oldestVisit,
		StudentVisitCounts: map[int64]int{
			1: 50,
			2: 50,
		},
	}

	output := captureOutput(t, func() {
		printPreviewHeader(preview)
	})

	assert.Contains(t, output, "Data Retention Cleanup Preview")
	assert.Contains(t, output, "Total visits to delete: 100")
	assert.Contains(t, output, "Oldest visit:")
	assert.Contains(t, output, "Students affected: 2")
}

func TestPrintPreviewHeader_NilOldestVisit(t *testing.T) {
	preview := &active.CleanupPreview{
		TotalVisits:        0,
		OldestVisit:        nil,
		StudentVisitCounts: map[int64]int{},
	}

	output := captureOutput(t, func() {
		printPreviewHeader(preview)
	})

	assert.Contains(t, output, "Total visits to delete: 0")
	assert.NotContains(t, output, "Oldest visit:")
}

func TestPrintRetentionStats(t *testing.T) {
	oldestVisit := time.Now().Add(-72 * time.Hour)
	stats := &active.RetentionStats{
		TotalExpiredVisits: 200,
		StudentsAffected:   15,
		OldestExpiredVisit: &oldestVisit,
		ExpiredVisitsByMonth: map[string]int64{
			"2024-01": 100,
			"2024-02": 100,
		},
	}

	output := captureOutput(t, func() {
		printRetentionStats(stats)
	})

	assert.Contains(t, output, "Data Retention Statistics")
	assert.Contains(t, output, "Total expired visits: 200")
	assert.Contains(t, output, "Students affected: 15")
	assert.Contains(t, output, "Oldest expired visit:")
}

func TestPrintRetentionStats_NilOldestVisit(t *testing.T) {
	stats := &active.RetentionStats{
		TotalExpiredVisits: 0,
		StudentsAffected:   0,
		OldestExpiredVisit: nil,
	}

	output := captureOutput(t, func() {
		printRetentionStats(stats)
	})

	assert.Contains(t, output, "Total expired visits: 0")
	assert.NotContains(t, output, "Oldest expired visit:")
}

func TestPrintAttendancePreviewHeader(t *testing.T) {
	oldest := time.Now().Add(-24 * time.Hour)
	preview := &active.AttendanceCleanupPreview{
		TotalRecords: 30,
		OldestRecord: &oldest,
		StudentRecords: map[int64]int{
			10: 15,
			20: 15,
		},
	}

	output := captureOutput(t, func() {
		printAttendancePreviewHeader(preview)
	})

	assert.Contains(t, output, "Attendance Cleanup Preview")
	assert.Contains(t, output, "Total stale records: 30")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "Students affected: 2")
}

func TestPrintAttendancePreviewHeader_NilOldestRecord(t *testing.T) {
	preview := &active.AttendanceCleanupPreview{
		TotalRecords:   0,
		OldestRecord:   nil,
		StudentRecords: map[int64]int{},
	}

	output := captureOutput(t, func() {
		printAttendancePreviewHeader(preview)
	})

	assert.Contains(t, output, "Total stale records: 0")
	assert.NotContains(t, output, "Oldest record:")
}

func TestPrintAttendanceCleanupSummary_Success(t *testing.T) {
	now := time.Now()
	oldest := now.Add(-48 * time.Hour)
	result := &active.AttendanceCleanupResult{
		StartedAt:        now.Add(-2 * time.Second),
		CompletedAt:      now,
		RecordsClosed:    25,
		StudentsAffected: 10,
		OldestRecordDate: &oldest,
		Success:          true,
		Errors:           nil,
	}

	output := captureOutput(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "Attendance Cleanup Summary")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Records closed: 25")
	assert.Contains(t, output, "Students affected: 10")
	assert.Contains(t, output, "Oldest record:")
	assert.Contains(t, output, "SUCCESS")
}

func TestPrintAttendanceCleanupSummary_NilOldestDate(t *testing.T) {
	now := time.Now()
	result := &active.AttendanceCleanupResult{
		StartedAt:        now.Add(-1 * time.Second),
		CompletedAt:      now,
		RecordsClosed:    0,
		StudentsAffected: 0,
		OldestRecordDate: nil,
		Success:          true,
	}

	output := captureOutput(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "Records closed: 0")
	assert.NotContains(t, output, "Oldest record:")
}

func TestPrintAttendanceCleanupSummary_WithErrors(t *testing.T) {
	now := time.Now()
	result := &active.AttendanceCleanupResult{
		StartedAt:        now.Add(-3 * time.Second),
		CompletedAt:      now,
		RecordsClosed:    5,
		StudentsAffected: 3,
		Success:          false,
		Errors:           []string{"failed for student 1", "failed for student 2"},
	}

	output := captureOutput(t, func() {
		printAttendanceCleanupSummary(result)
	})

	assert.Contains(t, output, "COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 2")
}

func TestPrintErrorList_Empty(t *testing.T) {
	output := captureOutput(t, func() {
		printErrorList(nil)
	})
	assert.Empty(t, output)
}

func TestPrintErrorList_EmptySlice(t *testing.T) {
	output := captureOutput(t, func() {
		printErrorList([]string{})
	})
	assert.Empty(t, output)
}

func TestPrintErrorList_WithErrors_NotVerbose(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	errors := []string{"error one", "error two"}
	output := captureOutput(t, func() {
		printErrorList(errors)
	})

	assert.Contains(t, output, "Errors: 2")
	assert.NotContains(t, output, "error one")
	assert.NotContains(t, output, "error two")
}

func TestPrintErrorList_WithErrors_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	errors := []string{"error one", "error two"}
	output := captureOutput(t, func() {
		printErrorList(errors)
	})

	assert.Contains(t, output, "Errors: 2")
	assert.Contains(t, output, "error one")
	assert.Contains(t, output, "error two")
}

func TestPrintAbandonedSessionSummary(t *testing.T) {
	output := captureOutput(t, func() {
		printAbandonedSessionSummary(2*time.Hour, 5)
	})

	assert.Contains(t, output, "Abandoned Session Cleanup Summary")
	assert.Contains(t, output, "Threshold: 2h0m0s")
	assert.Contains(t, output, "Sessions cleaned: 5")
	assert.Contains(t, output, "Status: SUCCESS")
}

func TestPrintDailySessionSummary(t *testing.T) {
	result := &active.DailySessionCleanupResult{
		SessionsEnded: 3,
		VisitsEnded:   15,
		Success:       true,
		Errors:        nil,
	}

	output := captureOutput(t, func() {
		printDailySessionSummary(result)
	})

	assert.Contains(t, output, "Daily Session Cleanup Summary")
	assert.Contains(t, output, "Sessions ended: 3")
	assert.Contains(t, output, "Visits ended: 15")
	assert.Contains(t, output, "SUCCESS")
}

func TestPrintDailySessionSummary_WithErrors(t *testing.T) {
	result := &active.DailySessionCleanupResult{
		SessionsEnded: 1,
		VisitsEnded:   5,
		Success:       false,
		Errors:        []string{"session 1 failed"},
	}

	output := captureOutput(t, func() {
		printDailySessionSummary(result)
	})

	assert.Contains(t, output, "COMPLETED WITH ERRORS")
	assert.Contains(t, output, "Errors: 1")
}

// =============================================================================
// logVisitCleanupResult Tests
// =============================================================================

func TestLogVisitCleanupResult_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	now := time.Now()
	result := &active.CleanupResult{
		StartedAt:         now.Add(-3 * time.Second),
		CompletedAt:       now,
		StudentsProcessed: 5,
		RecordsDeleted:    25,
		Success:           true,
		Errors:            nil,
	}

	logVisitCleanupResult(logger, result)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "Cleanup completed in")
	assert.Contains(t, logOutput, "Students processed: 5")
	assert.Contains(t, logOutput, "Records deleted: 25")
	assert.NotContains(t, logOutput, "Errors encountered")
}

func TestLogVisitCleanupResult_WithErrors_NotVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	now := time.Now()
	result := &active.CleanupResult{
		StartedAt:         now.Add(-3 * time.Second),
		CompletedAt:       now,
		StudentsProcessed: 5,
		RecordsDeleted:    10,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 100, Error: "delete failed"},
		},
	}

	logVisitCleanupResult(logger, result)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "Errors encountered: 1")
	assert.NotContains(t, logOutput, "Student 100")
}

func TestLogVisitCleanupResult_WithErrors_Verbose(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	now := time.Now()
	result := &active.CleanupResult{
		StartedAt:         now.Add(-1 * time.Second),
		CompletedAt:       now,
		StudentsProcessed: 3,
		RecordsDeleted:    5,
		Success:           false,
		Errors: []active.CleanupError{
			{StudentID: 100, Error: "delete failed"},
			{StudentID: 200, Error: "timeout"},
		},
	}

	logVisitCleanupResult(logger, result)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "Errors encountered: 2")
	assert.Contains(t, logOutput, "Student 100: delete failed")
	assert.Contains(t, logOutput, "Student 200: timeout")
}

// =============================================================================
// Usage Output Tests
// =============================================================================

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
