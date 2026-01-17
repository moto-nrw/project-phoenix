package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Constants Tests
// =============================================================================

func TestConstants(t *testing.T) {
	assert.Equal(t, "2006-01-02", dateFormat)
	assert.Equal(t, "2006-01-02 15:04:05", dateTimeFormat)
	assert.Equal(t, "failed to initialize database: %w", errInitDB)
	assert.Equal(t, "failed to create service factory: %w", errServiceFactory)
}

// =============================================================================
// printStudentBreakdown Tests
// =============================================================================

func TestPrintStudentBreakdown_Empty(_ *testing.T) {
	// Should not panic with empty data
	printStudentBreakdown("Test Header", "Count", map[int64]int{})
}

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
	ctx := &cleanupContext{
		DB: nil,
	}

	// Should not panic
	ctx.Close()
}
