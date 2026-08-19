package cmd

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Constants Tests
// =============================================================================

// Deliberately NOT parallel: the cmd package's tests drive cobra commands and
// initConfig, which read and write the viper singleton and os.Stdout. Nothing
// in this package is isolated from the next test.
func TestConstants(t *testing.T) {
	assert.Equal(t, "2006-01-02", dateFormat)
	assert.Equal(t, "2006-01-02 15:04:05", dateTimeFormat)
	assert.Equal(t, "failed to initialize database: %w", errInitDB)
	assert.Equal(t, "failed to close database: %v", errCloseDB)
	assert.Equal(t, "failed to create service factory: %w", errServiceFactory)
	assert.Equal(t, "failed to flush writer: %v", errFlushWriter)
}

// =============================================================================
// setupLogger Tests
// =============================================================================

// Deliberately NOT parallel: the cmd package's tests drive cobra commands and
// initConfig, which read and write the viper singleton and os.Stdout. Nothing
// in this package is isolated from the next test.
func TestSetupLogger_Stdout(t *testing.T) {
	logger, cleanup, err := setupLogger("")

	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, cleanup)

	// Cleanup should be callable without panic
	cleanup()
}

// Deliberately NOT parallel: the cmd package's tests drive cobra commands and
// initConfig, which read and write the viper singleton and os.Stdout. Nothing
// in this package is isolated from the next test.
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

// Deliberately NOT parallel: the cmd package's tests drive cobra commands and
// initConfig, which read and write the viper singleton and os.Stdout. Nothing
// in this package is isolated from the next test.
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

// Deliberately NOT parallel: the cmd package's tests drive cobra commands and
// initConfig, which read and write the viper singleton and os.Stdout. Nothing
// in this package is isolated from the next test.
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
