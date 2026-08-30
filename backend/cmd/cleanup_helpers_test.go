package cmd

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupRootsExposeOnlyCommandSpecificBuilders(t *testing.T) {
	t.Parallel()

	contextType := reflect.TypeFor[cleanupContext]()
	_, hasRepositoriesFactory := contextType.FieldByName("RepoFactory")
	_, hasServicesFactory := contextType.FieldByName("ServiceFactory")
	assert.False(t, hasRepositoriesFactory)
	assert.False(t, hasServicesFactory)
	assert.NotNil(t, defaultCleanupRoot.authCleanup)
	assert.NotNil(t, defaultCleanupRoot.invitationCleanup)
	assert.NotNil(t, defaultCleanupRoot.sessionCleanup)
	assert.NotNil(t, defaultCleanupRoot.retentionCleanup)
	assert.NotNil(t, defaultCleanupRoot.timetableCleanup)
	assert.NotNil(t, defaultCleanupRoot.timeTrackingCleanup)
}

func TestCleanupRootFailsFastForEveryMissingCapability(t *testing.T) {
	t.Parallel()

	for _, capability := range []string{"auth", "invitation", "session", "retention", "timetable", "time-tracking"} {
		root := defaultCleanupRoot
		switch capability {
		case "auth":
			root.authCleanup = nil
		case "invitation":
			root.invitationCleanup = nil
		case "session":
			root.sessionCleanup = nil
		case "retention":
			root.retentionCleanup = nil
		case "timetable":
			root.timetableCleanup = nil
		case "time-tracking":
			root.timeTrackingCleanup = nil
		}
		require.EqualError(t, root.validateCapability(capability), capability+" cleanup service builder is required")
	}

	root := defaultCleanupRoot
	root.openDatabase = nil
	require.EqualError(t, root.validateCapability("auth"), "cleanup database opener is required")
}

func TestCleanupRootFailsFastForEveryNilBuiltCapability(t *testing.T) {
	t.Parallel()

	ctx := &cleanupContext{}
	tests := []struct {
		capability string
		build      func() error
	}{
		{"auth", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) authCleanupService { return nil }, "auth")
			return err
		}},
		{"invitation", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) invitationCleanupService { return nil }, "invitation")
			return err
		}},
		{"session", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) sessionCleanupService { return nil }, "session")
			return err
		}},
		{"retention", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) retentionCleanupService { return nil }, "retention")
			return err
		}},
		{"timetable", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) timetableCleanupService { return nil }, "timetable")
			return err
		}},
		{"time-tracking", func() error {
			_, err := buildCleanupDependency(ctx, func(*cleanupContext) timeTrackingCleanupService { return nil }, "time-tracking")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.capability, func(t *testing.T) {
			require.EqualError(t, test.build(), test.capability+" cleanup service builder returned nil")
		})
	}
}

type nilAuthCleanup struct{}

func (*nilAuthCleanup) CleanupExpiredTokens(context.Context) (int, error)     { return 0, nil }
func (*nilAuthCleanup) CleanupExpiredRateLimits(context.Context) (int, error) { return 0, nil }

func TestCleanupRootRejectsTypedNilBuiltCapability(t *testing.T) {
	t.Parallel()

	_, err := buildCleanupDependency(&cleanupContext{}, func(*cleanupContext) authCleanupService {
		var service *nilAuthCleanup
		return service
	}, "auth")
	require.EqualError(t, err, "auth cleanup service builder returned nil")
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2006-01-02", dateFormat)
	assert.Equal(t, "2006-01-02 15:04:05", dateTimeFormat)
	assert.Equal(t, "failed to initialize database: %w", errInitDB)
	assert.Equal(t, "failed to close database: %v", errCloseDB)
	assert.Equal(t, "failed to flush writer: %v", errFlushWriter)
}

// =============================================================================
// setupLogger Tests
// =============================================================================

func TestSetupLogger_Stdout(t *testing.T) {
	logger, cleanup, err := setupLogger("")

	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, cleanup)

	// Cleanup should be callable without panic
	cleanup()
}

func TestSetupLogger_File(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, cleanup, err := setupLogger(logPath)

	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, cleanup)
	defer cleanup()

	// Write to the logger
	logger.Println("test message")

	// Verify file was created
	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestSetupLogger_InvalidPath(t *testing.T) {
	// Try to create log file in non-existent directory
	logger, cleanup, err := setupLogger("/nonexistent/path/test.log")

	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.Nil(t, cleanup)
}

// =============================================================================
// printStudentBreakdown Tests
// =============================================================================

func TestPrintStudentBreakdown_Empty(_ *testing.T) {
	// Should not panic with empty data
	printStudentBreakdown("Test Header", "Count", map[int64]int{})
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPrintStudentBreakdown_WithData(t *testing.T) {
	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[int64]int{
		1: 10,
		2: 20,
	}
	printStudentBreakdown("Test Header", "Visit Count", data)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Test Header")
	assert.Contains(t, output, "Visit Count")
}

// =============================================================================
// printDateBreakdown Tests
// =============================================================================

func TestPrintDateBreakdown_Empty(_ *testing.T) {
	// Should not panic with empty data
	printDateBreakdown(map[string]int{})
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPrintDateBreakdown_WithData(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]int{
		"2024-01-15": 5,
		"2024-01-16": 10,
	}
	printDateBreakdown(data)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Per-date breakdown")
	assert.Contains(t, output, "Date")
	assert.Contains(t, output, "Records")
}

// =============================================================================
// printStudentBreakdownWithTotal Tests
// =============================================================================

func TestPrintStudentBreakdownWithTotal_Empty(_ *testing.T) {
	// Should not panic with empty data
	printStudentBreakdownWithTotal("Count", map[int64]int{})
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPrintStudentBreakdownWithTotal_WithData(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[int64]int{
		1: 10,
		2: 20,
		3: 30,
	}
	printStudentBreakdownWithTotal("Visits", data)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Per-student breakdown")
	assert.Contains(t, output, "Student ID")
	assert.Contains(t, output, "TOTAL")
}

// =============================================================================
// printMonthlyBreakdownWithTotal Tests
// =============================================================================

func TestPrintMonthlyBreakdownWithTotal_Empty(_ *testing.T) {
	// Should not panic with empty data
	printMonthlyBreakdownWithTotal("Test Header", map[string]int64{})
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPrintMonthlyBreakdownWithTotal_WithData(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]int64{
		"2024-01": 100,
		"2024-02": 200,
	}
	printMonthlyBreakdownWithTotal("Monthly Stats", data)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Monthly Stats")
	assert.Contains(t, output, "Month")
	assert.Contains(t, output, "TOTAL")
}

// =============================================================================
// printRecentDeletions Tests
// =============================================================================

func TestPrintRecentDeletions_Empty(_ *testing.T) {
	// Should not panic with empty slice
	printRecentDeletions([]recentDeletionRow{})
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPrintRecentDeletions_WithData(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := []recentDeletionRow{
		{Date: "2024-01-15", RecordsDeleted: 50, StudentCount: 5},
		{Date: "2024-01-14", RecordsDeleted: 30, StudentCount: 3},
	}
	printRecentDeletions(data)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Date")
	assert.Contains(t, output, "Records Deleted")
	assert.Contains(t, output, "Students")
}

// =============================================================================
// recentDeletionRow Tests
// =============================================================================

func TestRecentDeletionRow_Struct(t *testing.T) {
	row := recentDeletionRow{
		Date:           "2024-01-15",
		RecordsDeleted: 100,
		StudentCount:   10,
	}

	assert.Equal(t, "2024-01-15", row.Date)
	assert.Equal(t, int64(100), row.RecordsDeleted)
	assert.Equal(t, int64(10), row.StudentCount)
}

// =============================================================================
// cleanupContext Tests
// =============================================================================

func TestCleanupContext_Close_NilDB(_ *testing.T) {
	ctx := &cleanupContext{
		DB: nil,
	}

	// Should not panic with nil DB
	ctx.Close()
}

func TestCleanupContext_Close_WithDB(_ *testing.T) {
	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	// Create a context with nil DB (simulating closed DB)
	ctx := &cleanupContext{
		DB: nil,
	}

	// Should not panic
	ctx.Close()
}

func TestCleanupRootFailsFastWithoutDatabaseDependency(t *testing.T) {
	_, err := (cleanupRoot{}).newContext()
	require.ErrorContains(t, err, "cleanup database opener is required")
}
